package httpd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

var (
	checkCancelledProcessDuration = 250 * time.Millisecond
	sigtermCancelWaitDuration     = 10 * time.Second
	sigkillCancelWaitDuration     = 10 * time.Second
)

func GetCmd(cmd string, args []string, env map[string]interface{}) *exec.Cmd {
	// Set up command to run and arguments.
	execCmd := exec.Command(cmd, args...)
	execCmd.Env = os.Environ()
	execCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	for k, v := range env {
		execCmd.Env = append(execCmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return execCmd
}

// CmdExecutors handles the lifetime of all executing commands.
type CmdExecutorManager struct {
	Executors map[string]*CmdExecutor
	Mutex     *sync.Mutex
}

// CreateExecutor creates a new CmdExecutor, inserts the CmdExecutor into a global map of
// running executions, and returns the CmdExecutor along with its unique identifier.
func (c *CmdExecutorManager) CreateExecutor() *CmdExecutor {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	executionID := uuid.NewString()
	executor := newCmdExecutor(executionID)
	c.Executors[executionID] = executor
	return executor
}

// DeleteExecutor deletes a executor from a global map of executions. If the executor is currently
// running, we cancel it. If a nil executionID is passed, we default to deleting the only
// executor in the global map if there is exactly one, otherwise we error.
func (c *CmdExecutorManager) DeleteExecutor(executionID *string) error {
	cmdExecutor, err := func() (*CmdExecutor, error) {
		c.Mutex.Lock()
		defer c.Mutex.Unlock()
		var idToDelete string
		if executionID != nil {
			if _, ok := c.Executors[*executionID]; ok {
				idToDelete = *executionID
			} else {
				return nil, &InvalidExecIDError{*executionID}
			}
		} else {
			if len(c.Executors) == 1 {
				for key := range c.Executors {
					idToDelete = key
					break
				}
				logger.Log("no execution id provided, cancelling only existing execution: %s", idToDelete)
			} else if len(c.Executors) == 0 {
				return nil, &NoExecutionToCancelError{}
			} else {
				return nil, &AmbiguousCancelError{}
			}
		}
		cmdExecutor, ok := c.Executors[idToDelete]
		if !ok {
			return nil, errors.Errorf("trying to delete missing executionID: %s", idToDelete)
		}
		// Delete even if the process doesn't exist or can't be cancelled.
		delete(c.Executors, idToDelete)
		return cmdExecutor, nil
	}()
	if err != nil {
		return err
	}
	return cmdExecutor.Cancel()
}

// CmdExecutor handles the lifetime of an executing command.
// The current active command, if there is one, is stored in `ActiveCmd`.
type CmdExecutor struct {
	ActiveCmd      *exec.Cmd
	ActiveCmdMutex *sync.Mutex
	SignalC        chan syscall.Signal
	ExecutionID    string
}

func newCmdExecutor(executionID string) *CmdExecutor {
	signalC := make(chan syscall.Signal, 1)
	mutex := sync.Mutex{}
	return &CmdExecutor{
		ActiveCmd:      nil,
		ActiveCmdMutex: &mutex,
		SignalC:        signalC,
		ExecutionID:    executionID,
	}
}

type Encoder interface {
	Encode(v interface{}) error
}

type ChunkedEncoder struct {
	Encoder Encoder
	Flusher http.Flusher
}

func (c *ChunkedEncoder) Encode(v interface{}) error {
	err := c.Encoder.Encode(v)
	c.Flusher.Flush()
	return err
}

// Run manages the lifetime of the subprocess. It consumes outputs from the process and returns when the process has completed.
// The process is determined as completed when all outputs are finished writing or if there is an error in this function itself.
func (c *CmdExecutor) Run(outputC <-chan Output, outputDoneC <-chan interface{}, outputErrC <-chan error, encoder *ChunkedEncoder) error {
	if !c.hasActiveProcess() {
		return errors.New("running inactive command")
	}

	outputDoneCount := 0
	for {
		select {
		case signal := <-c.SignalC:
			logger.Log("recieved signal: %v", signal)
			if !c.hasActiveProcess() {
				return errors.New("unable to signal, command already exited")
			}
			// Kill all processes in the process group by sending signal to -pid.
			err := syscall.Kill(-c.ActiveCmd.Process.Pid, signal)
			// If for some reason we aren't able to kill the process group, try to at least kill the
			// parent process.
			if err != nil && err.Error() == "no such process" {
				err = syscall.Kill(c.ActiveCmd.Process.Pid, signal)
			}
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("unable to signal: %s", c.ExecutionID))
			}
		case output := <-outputC:
			if err := encoder.Encode(output); err != nil {
				return err
			}
		case <-outputDoneC:
			outputDoneCount++
			// Process is completed.
			if outputDoneCount == cap(outputDoneC) {
				// Finish writing any outputs that are left.
				for {
					select {
					case output := <-outputC:
						if err := encoder.Encode(output); err != nil {
							return err
						}
					default:
						return nil
					}
				}
			}
		case err := <-outputErrC:
			logger.Log("recieved err: %v", err)
			return err
		}
	}
}

func (c *CmdExecutor) hasActiveProcess() bool {
	c.ActiveCmdMutex.Lock()
	defer c.ActiveCmdMutex.Unlock()
	return c.ActiveCmd != nil && c.ActiveCmd.Process != nil
}

// Wait waits on the underlying subprocess. This should only be called when all outputs from
// the subprocess are finished being consumed.
func (c *CmdExecutor) Wait() error {
	if !c.hasActiveProcess() {
		return errors.New("waiting on inactive command")
	}
	err := c.ActiveCmd.Wait()
	c.ActiveCmdMutex.Lock()
	defer c.ActiveCmdMutex.Unlock()
	c.ActiveCmd = nil
	return err
}

// Execute begins executing the command subprocess and sets up stdout/stderr pipes for
// the process.
func (c *CmdExecutor) Execute(cmd *exec.Cmd) (io.ReadCloser, io.ReadCloser, error) {
	c.ActiveCmdMutex.Lock()
	defer c.ActiveCmdMutex.Unlock()
	if c.ActiveCmd != nil {
		return nil, nil, &AlreadyRunningCmdError{
			ExistingCmd: c.ActiveCmd,
			NewCmd:      cmd,
		}
	}
	logger.Log("starting up: %s", cmd)

	// Configure stdout/stderr pipes for streaming outputs.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, &StartCmdError{
			Cmd: cmd,
			Err: err,
		}
	}
	c.ActiveCmd = cmd
	return stdoutPipe, stderrPipe, nil
}

// Cancel attempts to terminate the running process. It does so first by sending a SIGTERM,
// waiting to see if the running process has stopped, and if the process hasn't, sends a SIGKILL.
func (c *CmdExecutor) Cancel() error {
	if !c.hasActiveProcess() {
		return nil
	}
	select {
	case c.SignalC <- syscall.SIGTERM:
	default:
		logger.Warning("already processing signal, discarding sigterm")
	}
	sigintTimer := time.NewTimer(sigtermCancelWaitDuration)
	sigkillTimer := time.NewTimer(sigtermCancelWaitDuration + sigkillCancelWaitDuration)
	checkCancelledTicker := time.NewTicker(checkCancelledProcessDuration)
	for {
		select {
		case <-sigintTimer.C:
			select {
			case c.SignalC <- syscall.SIGKILL:
			default:
				logger.Warning("already processing signal, discarding sigkill")
			}
		case <-sigkillTimer.C:
			return errors.New("unable to determine if process has been cancelled")
		case <-checkCancelledTicker.C:
			if !c.hasActiveProcess() {
				return nil
			}
		}
	}
}
