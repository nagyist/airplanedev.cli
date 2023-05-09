package http

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrStatusCode(t *testing.T) {
	require := require.New(t)

	require.Equal("404: no task found", ErrStatusCode{
		StatusCode: http.StatusNotFound,
		Msg:        "no task found",
	}.Error())
}
