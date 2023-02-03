package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

type ErrStatusCode struct {
	// StatusCode is an HTTP status code.
	StatusCode int
	// Msg is the error message extracted, if set, from the API's response.
	Msg string
	// ErrorCode is a unique identifier of this error scenario extracted, if set, from the
	// API's response
	ErrorCode string
}

func (e ErrStatusCode) Error() string {
	return fmt.Sprintf("%d: %s", e.StatusCode, e.Msg)
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func NewErrStatusCodeFromResponse(resp *http.Response) error {
	errsc := ErrStatusCode{
		StatusCode: resp.StatusCode,
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "reading response body")
	}
	if err := resp.Body.Close(); err != nil {
		return errors.Wrap(err, "closing response body")
	}

	if isJSONContentType(resp.Header) {
		// Try to grab an error message.
		var e ErrorResponse
		if err := json.Unmarshal(body, &e); err != nil {
			return errors.Wrap(err, "unmarshalling error response body as JSON")
		}
		errsc.Msg = e.Error
		errsc.ErrorCode = e.Code
	} else {
		errsc.Msg = string(body)
	}

	return errsc
}

func newErrStatusCode(statusCode int, msg string, args ...any) ErrStatusCode {
	return ErrStatusCode{
		StatusCode: statusCode,
		Msg:        fmt.Sprintf(msg, args...),
	}
}

func NewErrBadRequest(msg string, args ...any) ErrStatusCode {
	return newErrStatusCode(http.StatusBadRequest, msg, args...)
}

func NewErrNotFound(msg string, args ...any) ErrStatusCode {
	return newErrStatusCode(http.StatusNotFound, msg, args...)
}

func NewErrUnauthorized(msg string, args ...any) ErrStatusCode {
	return newErrStatusCode(http.StatusUnauthorized, msg, args...)
}

func NewErrForbidden(msg string, args ...any) ErrStatusCode {
	return newErrStatusCode(http.StatusForbidden, msg, args...)
}

func NewErrNotImplemented(msg string, args ...any) ErrStatusCode {
	return newErrStatusCode(http.StatusNotImplemented, msg, args...)
}

func NewErrConflict(msg string, args ...any) ErrStatusCode {
	return newErrStatusCode(http.StatusConflict, msg, args...)
}

func NewErrTooManyRequests(msg string, args ...any) ErrStatusCode {
	return newErrStatusCode(http.StatusTooManyRequests, msg, args...)
}

func NewErrLocked(msg string, args ...any) ErrStatusCode {
	return newErrStatusCode(http.StatusLocked, msg, args...)
}

func NewErrInternalServerError(msg string, args ...any) ErrStatusCode {
	return newErrStatusCode(http.StatusInternalServerError, msg, args...)
}
