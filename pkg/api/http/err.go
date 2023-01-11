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
		var e struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
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
