package httpd

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/pkg/errors"
)

// CmdExecutor handles the lifetime of an executed command.
// The current active command, if there is one, is stored in `ActiveCmd`.
type CmdExecutor struct {
	ActiveCmd *exec.Cmd
	SignalC   chan os.Signal
}

func GetCmdExecutor() *CmdExecutor {
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	return &CmdExecutor{
		SignalC:   signalC,
		ActiveCmd: nil,
	}
}

// Run manages the lifetime of the subprocess. It consumes outputs from the process and returns when the process has completed.
// The process is determined as completed when all outputs are finished writing or if there is an error in this function itself.
func (c *CmdExecutor) Run(outputC <-chan interface{}, outputDoneC <-chan interface{}, outputErrC <-chan error, encoder *ChunkedEncoder) error {
	if c.ActiveCmd == nil || c.ActiveCmd.Process == nil {
		return errors.New("running inactive command")
	}

	outputDoneCount := 0
	for {
		select {
		case signal := <-c.SignalC:
			logger.Log("recieved signal: %v", signal)
			if c.ActiveCmd == nil || c.ActiveCmd.Process == nil {
				return errors.New("unable to signal, command already exited")
			}
			if err := c.ActiveCmd.Process.Signal(signal); err != nil {
				return errors.Wrap(err, "unable to signal, command already exited")
			}
		case output := <-outputC:
			err := encoder.Encode(output)
			if err != nil {
				return err
			}
		case <-outputDoneC:
			outputDoneCount++
			// Process is completed.
			if outputDoneCount == cap(outputDoneC) {
				return nil
			}
		case err := <-outputErrC:
			return err
		}
	}
}

func (c *CmdExecutor) Wait() error {
	if c.ActiveCmd == nil || c.ActiveCmd.Process == nil {
		return errors.New("waiting on inactive command")
	}
	err := c.ActiveCmd.Wait()
	c.ActiveCmd = nil
	return err
}

func (c *CmdExecutor) Execute(cmd *exec.Cmd) error {
	if c.ActiveCmd != nil {
		return &AlreadyRunningCmdError{
			ExistingCmd: c.ActiveCmd,
			NewCmd:      cmd,
		}
	}
	logger.Log("starting up: %s", cmd)
	if err := cmd.Start(); err != nil {
		return &StartCmdError{
			Cmd: cmd,
			Err: err,
		}
	}
	c.ActiveCmd = cmd
	return nil
}

func (c *CmdExecutor) Cancel() error {
	select {
	case c.SignalC <- os.Kill:
	default:
		logger.Warning("already processing signal, discarding kill")
	}
	// TODO: check if cmd is actually killed
	// TODO: first try SIGINT and then SIGKILL if it doesn't work
	return nil
}
