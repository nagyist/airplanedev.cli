package httpd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	shutdownTimeoutDuration = 10 * time.Second
)

type OutputType string

const (
	OutputTypeStdout     OutputType = "stdout"
	OutputTypeStderr     OutputType = "stderr"
	OutputTypeExitStatus OutputType = "status"
	OutputTypeSystem     OutputType = "system"
)

type Output struct {
	Msg         string     `json:"msg"`
	OutputType  OutputType `json:"outputType"`
	ExecutionID string     `json:"execID"`
}

type ExitStatusOutput struct {
	Output
	Code int `json:"code"`
}

type SystemOutputType string

const (
	SystemOutputTypeMetadata SystemOutputType = "metadata"
	SystemOutputTypeError    SystemOutputType = "error"
)

type SystemOutput struct {
	Output
	SystemOutputType SystemOutputType `json:"systemOutputType"`
}

type ChunkedEncoder struct {
	Encoder *json.Encoder
	Flusher http.Flusher
}

func (c *ChunkedEncoder) Encode(v interface{}) error {
	err := c.Encoder.Encode(v)
	c.Flusher.Flush()
	return err
}

// getExitStatus tries to extract the exit status from an error. The default
// error status is 1 if we are unable to determine the error.
func getExitStatus(err error) (int, string) {
	var code int
	var msg string
	if err == nil {
		code = 0
	} else {
		msg = err.Error()
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0

			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				code = status.ExitStatus()
			}
		} else {
			// If we're unable to determine the exit status, default the error code to 1.
			code = 1
		}
	}
	return code, msg
}

// ExecuteCmdHandler executes a command as a subprocess and streams the process's outputs to the client.
func ExecuteCmdHandler(cmd string, args []string, cmdExecutors map[string]*CmdExecutor, cmdExecutorsMutex *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Args []string               `json:"args"`
			Env  map[string]interface{} `json:"env"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		// Set up command to run and escape arguments.
		cmd := exec.Command(cmd, append(args, req.Args...)...)
		cmd.Env = os.Environ()
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		// Configure stdout/stderr pipes for streaming outputs.
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			WriteHTTPError(w, r, err)
			return
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			WriteHTTPError(w, r, err)
			return
		}

		// Create a new CmdExecutor and insert it into a global map of running executions
		// using a unique id to keep track of each execution.
		cmdExecutorsMutex.Lock()
		executionID := uuid.NewString()
		cmdExecutor := GetCmdExecutor()
		cmdExecutors[executionID] = cmdExecutor
		cmdExecutorsMutex.Unlock()

		// Begin executing the command.
		if err := cmdExecutor.Execute(cmd); err != nil {
			WriteHTTPError(w, r, err)
			return
		}

		// Configure chunked encoding responses.
		flusher, ok := w.(http.Flusher)
		if !ok {
			WriteHTTPError(w, r, errors.New("expected http.ResponseWriter to be an http.Flusher"))
			return
		}
		// After setting these headers, we can't call `WriteHTTPError` anymore to write an error.
		// Instead, we should write the error as a ExistStatusOutput message.
		w.Header().Set("Connection", "Keep-Alive")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Transfer-Encoding", "chunked")

		// outputC contains a stream of stdout / stderr from the child process
		outputC := make(chan interface{}, 4096)
		// outputErrC contains errors from the stdout / stderr forwarding routines
		outputErrC := make(chan error)
		// outputDoneC recieves a message from the stdout / stderr routines when they are done.
		// Since there is one routine for capturing stdout and one for stderr, we set a channel
		// capacity of 2. The capacity is used by the downstream process to determine when
		// outputs have finished.
		outputDoneC := make(chan interface{}, 2)
		// Send a message telling the client what the execution id is.
		outputC <- SystemOutput{
			Output{
				Msg:         executionID,
				OutputType:  OutputTypeSystem,
				ExecutionID: executionID,
			},
			SystemOutputTypeMetadata,
		}

		// Begin streaming stdout / stderr
		go WriteOutput(stdoutPipe, OutputTypeStdout, outputDoneC, outputC, outputErrC, executionID)
		go WriteOutput(stderrPipe, OutputTypeStderr, outputDoneC, outputC, outputErrC, executionID)

		// Check for if the request context ends prematurely. This can happen if the client ends
		// the connection.
		go func() {
			<-r.Context().Done()
			if r.Context().Err() != nil {
				outputErrC <- r.Context().Err()
			}
		}()

		// Run the command until completion.
		if err := cmdExecutor.Run(outputC, outputDoneC, outputErrC, &ChunkedEncoder{
			Encoder: json.NewEncoder(w),
			Flusher: flusher,
		}); err != nil {
			if err := json.NewEncoder(w).Encode(SystemOutput{
				Output{
					Msg:         err.Error(),
					OutputType:  OutputTypeSystem,
					ExecutionID: executionID,
				},
				SystemOutputTypeError,
			}); err != nil {
				log.Fatalf("unable to write system error: %+v", err)
			}
			err = cmdExecutor.ActiveCmd.Process.Signal(os.Kill)
			if err != nil {
				logger.Error("unable to kill process: %+v", err)
			}
		}

		exitStatus, exitMsg := getExitStatus(cmdExecutor.Wait())
		if err := json.NewEncoder(w).Encode(ExitStatusOutput{
			Output{
				Msg:         exitMsg,
				OutputType:  OutputTypeExitStatus,
				ExecutionID: executionID,
			},
			exitStatus,
		}); err != nil {
			log.Fatalf("unable to write exit status message: %+v", err)
		}
		cmdExecutorsMutex.Lock()
		delete(cmdExecutors, executionID)
		cmdExecutorsMutex.Unlock()
	}
}

func WriteOutput(pipe io.ReadCloser, outputType OutputType, doneC chan<- interface{}, outputC chan<- interface{}, errC chan<- error, executionID string) {
	scanner := bufio.NewScanner(pipe)
	i := 1
	for scanner.Scan() {
		outputC <- Output{
			Msg:         scanner.Text(),
			OutputType:  outputType,
			ExecutionID: executionID,
		}
		i++
		if i == 3 {
			errC <- errors.New("blsh")
		}
	}
	if scanner.Err() != nil {
		errC <- scanner.Err()
		return
	}
	doneC <- struct{}{}
}

type CancelCmdRequest struct {
	ExecutionID string `json:"execID"`
}

type CancelCmdResponse struct {
	// Maps failed execution id to error message.
	FailedIDs    map[string]string `json:"failedIDs"`
	CancelledIDs []string          `json:"cancelledIDs"`
}

func CancelCmdHandler(cmdExecutors map[string]*CmdExecutor, cmdExecutorsMutex *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CancelCmdRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		cmdExecutorsMutex.Lock()
		defer cmdExecutorsMutex.Unlock()

		idsToCancel := []string{}
		if _, ok := cmdExecutors[req.ExecutionID]; ok {
			idsToCancel = append(idsToCancel, req.ExecutionID)
		} else if req.ExecutionID == "" {
			logger.Log("no execution id provided, cancelling every job")
			for key := range cmdExecutors {
				idsToCancel = append(idsToCancel, key)
			}
		} else {
			http.Error(w, fmt.Sprintf("invalid execID: %s", req.ExecutionID), http.StatusBadRequest)
			return
		}

		cancelledIDs := []string{}
		failedIDs := map[string]string{}
		for _, id := range idsToCancel {
			if err := cmdExecutors[id].Cancel(); err != nil {
				failedIDs[id] = err.Error()
			} else {
				cancelledIDs = append(cancelledIDs, id)
			}
			delete(cmdExecutors, id)
		}

		if err := json.NewEncoder(w).Encode(CancelCmdResponse{
			FailedIDs:    failedIDs,
			CancelledIDs: cancelledIDs,
		}); err != nil {
			WriteHTTPError(w, r, err)
		}
	}
}

func Route(cmd string, args []string, cmdExecutors map[string]*CmdExecutor) *mux.Router {
	router := mux.NewRouter()
	var cmdExecutorsMutex sync.Mutex
	router.HandleFunc("/", ExecuteCmdHandler(cmd, args, cmdExecutors, &cmdExecutorsMutex)).Methods("POST")
	router.HandleFunc("/cancel", CancelCmdHandler(cmdExecutors, &cmdExecutorsMutex)).Methods("POST")
	return router
}

// ServeWithGracefulShutdown starts the server and gracefully shuts down on SIGINT or SIGTERM.
// See: https://medium.com/honestbee-tw-engineer/gracefully-shutdown-in-go-http-server-5f5e6b83da5a
func ServeWithGracefulShutdown(
	ctx context.Context,
	server *http.Server,
) error {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Log("listening and serving on %s", server.Addr)
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalln(err)
		}
	}()

	<-signalChan
	logger.Warning("server shutting down: waiting up to %s", shutdownTimeoutDuration)

	ctx, cancel := context.WithTimeout(ctx, shutdownTimeoutDuration)
	defer cancel()

	if err := server.Shutdown(ctx); err != context.Canceled {
		log.Fatalf("server shutdown failed: %+v", err)
		return err
	}

	logger.Warning("server shutdown successfully")
	return nil
}
