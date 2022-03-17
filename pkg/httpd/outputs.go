package httpd

import (
	"bufio"
	"io"
)

type OutputType string

const (
	// Stdout logs.
	OutputTypeStdout OutputType = "stdout"
	// Stderr logs.
	OutputTypeStderr OutputType = "stderr"
	// Exit status of the subprocess.
	OutputTypeExit OutputType = "exit"
	// System messages from the http server. Includes metadata and error messages.
	OutputTypeSystem OutputType = "system"
)

type OutputStatus string

const (
	OutputStatusNone      = ""
	OutputStatusError     = "error"
	OutputStatusSuccess   = "success"
	OutputStatusCancelled = "cancelled"
	OutputStatusKilled    = "killed"
)

type Output struct {
	Msg  string     `json:"msg"`
	Type OutputType `json:"type"`
	// Indicates the status of the Output if the Output is a status message.
	Status      OutputStatus `json:"status,omitempty"`
	ExecutionID string       `json:"executionID"`
}

// WriteOutput writes the contents of a pipe into three output channels:
// 1. doneC is written to when the pipe is closed
// 2. outputC is written to when the pipe contains data
// 3. errC is written to when there is an error processesing the pipe
func WriteOutput(pipe io.ReadCloser, outputType OutputType, doneC chan<- interface{}, outputC chan<- Output, errC chan<- error, executionID string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		outputC <- Output{
			Msg:         scanner.Text(),
			Type:        outputType,
			ExecutionID: executionID,
		}
	}
	if scanner.Err() != nil {
		errC <- scanner.Err()
		return
	}
	doneC <- struct{}{}
}
