package httpd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

const (
	shutdownTimeoutDuration = 10 * time.Second
)

var (
	sigtermMsg = fmt.Sprintf("signal: %s", syscall.SIGTERM.String())
	sigkillMsg = fmt.Sprintf("signal: %s", syscall.SIGKILL.String())
)

type ExecuteCmdRequest struct {
	Args []string               `json:"args"`
	Env  map[string]interface{} `json:"env"`
}

// ExecuteCmdHandler executes a command as a subprocess and streams the process's outputs to the client
// via HTTP chunked encoding.
func ExecuteCmdHandler(cmd string, args []string, manager *CmdExecutorManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ExecuteCmdRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		// Configure chunked encoding responses.
		flusher, ok := w.(http.Flusher)
		if !ok {
			WriteHTTPError(w, r, errors.New("expected http.ResponseWriter to be an http.Flusher"))
			return
		}

		cmdExecutor := manager.CreateExecutor()
		defer func() {
			// It's possible that this executor has already been deleted if /cancel has been called on this executor, so we discard
			// InvalidExecIDError.
			if err := manager.DeleteExecutor(&cmdExecutor.ExecutionID); err != nil && errors.Is(err, &InvalidExecIDError{cmdExecutor.ExecutionID}) {
				logger.Error("unable to delete executor: %+v", err)
			}
		}()

		// Begin executing the command.
		stdoutPipe, stderrPipe, err := cmdExecutor.Execute(
			GetCmd(cmd, append(args, req.Args...), req.Env),
		)
		if err != nil {
			WriteHTTPError(w, r, err)
			return
		}

		// After setting these headers, we can't call `WriteHTTPError` anymore to write an error.
		// Instead, we should write the error as a ExitStatusOutput message.
		w.Header().Set("Connection", "Keep-Alive")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Transfer-Encoding", "chunked")

		// outputC contains a stream of stdout / stderr from the child process
		outputC := make(chan Output, 4096)
		// outputErrC contains errors from the stdout / stderr forwarding routines
		outputErrC := make(chan error)
		// outputDoneC recieves a message from the stdout / stderr routines when they are done.
		// Since there is one routine for capturing stdout and one for stderr, we set a channel
		// capacity of 2. The capacity is used by the downstream process to determine when
		// outputs have finished.
		outputDoneC := make(chan interface{}, 2)
		// Send an empty message so the client knows the execution id.
		outputC <- Output{
			Msg:         "",
			Type:        OutputTypeSystem,
			ExecutionID: cmdExecutor.ExecutionID,
		}
		// Begin streaming outputs
		go WriteOutput(stdoutPipe, OutputTypeStdout, outputDoneC, outputC, outputErrC, cmdExecutor.ExecutionID)
		go WriteOutput(stderrPipe, OutputTypeStderr, outputDoneC, outputC, outputErrC, cmdExecutor.ExecutionID)

		// Check for if the request context ends prematurely. This will happen if the client ends
		// the connection.
		go func() {
			<-r.Context().Done()
			if r.Context().Err() != nil {
				outputErrC <- r.Context().Err()
			}
		}()

		encoder := &ChunkedEncoder{
			Encoder: json.NewEncoder(w),
			Flusher: flusher,
		}
		// Run the command until completion.
		runErr := cmdExecutor.Run(outputC, outputDoneC, outputErrC, encoder)
		if runErr == context.Canceled {
			// Since the request context has been cancelled, there's no more content
			// we can write.
			// TODO(eric): cancel underlying process when request context ends
			return
		} else if runErr != nil {
			if err := json.NewEncoder(w).Encode(Output{
				Msg:         runErr.Error(),
				Type:        OutputTypeSystem,
				Status:      OutputStatusError,
				ExecutionID: cmdExecutor.ExecutionID,
			}); err != nil {
				log.Fatalf("unable to write system error: %+v", err)
			}
		}

		var exitMsg string
		var outputStatus OutputStatus = OutputStatusSuccess
		var outputType OutputType = OutputTypeExit
		if err := cmdExecutor.Wait(); err != nil {
			res, ok := err.(*exec.ExitError)
			if ok && res.ProcessState.String() == sigtermMsg {
				outputType = OutputTypeSystem
				outputStatus = OutputStatusCancelled
			} else if ok && res.ProcessState.String() == sigkillMsg {
				outputType = OutputTypeSystem
				outputStatus = OutputStatusKilled
			} else {
				exitMsg = err.Error()
				outputStatus = OutputStatusError
			}
		}

		if err := json.NewEncoder(w).Encode(Output{
			Msg:         exitMsg,
			Type:        outputType,
			Status:      outputStatus,
			ExecutionID: cmdExecutor.ExecutionID,
		}); err != nil {
			log.Fatalf("unable to write exit status message: %+v", err)
		}
	}
}

type CancelCmdRequest struct {
	ExecutionID *string `json:"execID"`
}

func CancelCmdHandler(manager *CmdExecutorManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CancelCmdRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if err := manager.DeleteExecutor(req.ExecutionID); err != nil {
			WriteHTTPError(w, r, err)
			return
		}
		fmt.Fprintf(w, "{}")
	}
}

func Route(cmd string, args []string) *mux.Router {
	router := mux.NewRouter()
	mutex := sync.Mutex{}
	manager := CmdExecutorManager{
		Executors: map[string]*CmdExecutor{},
		Mutex:     &mutex,
	}
	router.HandleFunc("/", ExecuteCmdHandler(cmd, args, &manager)).Methods("POST")
	router.HandleFunc("/cancel", CancelCmdHandler(&manager)).Methods("POST")
	return router
}

// ServeWithGracefulShutdown starts the server and gracefully shuts down on SIGINT or SIGTERM.
// See: https://medium.com/honestbee-tw-engineer/gracefully-shutdown-in-go-http-server-5f5e6b83da5a
func ServeWithGracefulShutdown(
	ctx context.Context,
	server *http.Server,
) error {
	signalChan := make(chan os.Signal, 1)
	// TODO(eric): gracefully shutdown running executions
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
