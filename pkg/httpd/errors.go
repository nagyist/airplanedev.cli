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

func (r *StartCmdError) Error() string {
	return fmt.Sprintf("cmd %s: err %v", r.Cmd, r.Err)
}

type AlreadyRunningCmdError struct {
	NewCmd      *exec.Cmd
	ExistingCmd *exec.Cmd
}

func (r *AlreadyRunningCmdError) Error() string {
	return fmt.Sprintf("unable to run: %s, already running: %s", r.NewCmd, r.ExistingCmd)
}

type NoExistingCmdError struct{}

func (r *NoExistingCmdError) Error() string {
	return "no existing command"
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
