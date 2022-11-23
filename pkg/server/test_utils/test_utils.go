package test_utils

import (
	"context"
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/gorilla/mux"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func GetHttpExpect(ctx context.Context, t *testing.T, router *mux.Router) *httpexpect.Expect {
	return httpexpect.WithConfig(httpexpect.Config{
		Reporter: httpexpect.NewAssertReporter(t),
		Client: &http.Client{
			Transport: httpexpect.NewBinder(router),
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Jar: httpexpect.NewJar(),
		},
		Context: ctx,
	})
}
