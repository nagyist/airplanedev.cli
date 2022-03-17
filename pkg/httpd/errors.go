package httpd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
)

type StartCmdError struct {
	Cmd *exec.Cmd
	Err error
}

func (e *StartCmdError) Error() string {
	return fmt.Sprintf("cmd %s: err %v", e.Cmd, e.Err)
}

type AlreadyRunningCmdError struct {
	NewCmd      *exec.Cmd
	ExistingCmd *exec.Cmd
}

func (e *AlreadyRunningCmdError) Error() string {
	return fmt.Sprintf("unable to run: %s, already running: %s", e.NewCmd, e.ExistingCmd)
}

type NoExistingCmdError struct{}

func (e *NoExistingCmdError) Error() string {
	return "no existing command"
}

type AmbiguousCancelError struct{}

func (e *AmbiguousCancelError) Error() string {
	return "multiple executions found, must specify execution id to cancel"
}

type NoExecutionToCancelError struct{}

func (e *NoExecutionToCancelError) Error() string {
	return "no executions to cancel"
}

type InvalidExecIDError struct {
	ExecID string
}

func (e *InvalidExecIDError) Error() string {
	return fmt.Sprintf("invalid execID: %s", e.ExecID)
}

type errorResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func WriteHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	var status int
	switch err.(type) {
	case *AlreadyRunningCmdError:
		status = http.StatusServiceUnavailable
	case *NoExistingCmdError:
		status = http.StatusServiceUnavailable
	case *InvalidExecIDError:
		status = http.StatusBadRequest
	case *NoExecutionToCancelError:
		status = http.StatusBadRequest
	case *AmbiguousCancelError:
		status = http.StatusBadRequest
	case *StartCmdError:
		status = http.StatusInternalServerError
	default:
		status = http.StatusInternalServerError
	}

	w.WriteHeader(status)
	resp, err := json.Marshal(errorResponse{
		Code:  status,
		Error: err.Error(),
	})
	if err != nil {
		log.Fatalf("errored while writing HTTP error: %v", err)
		return
	}
	fmt.Fprint(w, string(resp))
}
