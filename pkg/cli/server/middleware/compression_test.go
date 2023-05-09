package middleware

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/airplanedev/cli/pkg/cli/analytics"
	"github.com/airplanedev/cli/pkg/cli/server/handlers"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestReqBodyDecompression(t *testing.T) {
	r := mux.NewRouter()
	r.Use(ReqBodyDecompression)
	r.HandleFunc(
		"/echo",
		func(rw http.ResponseWriter, req *http.Request) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				handlers.WriteHTTPError(rw, req, err, analytics.ReportError)
				return
			}

			_, _ = rw.Write([]byte(body))
		},
	)

	server := httptest.NewServer(r)
	defer server.Close()

	client := &http.Client{}
	ctx := context.Background()

	// Basic GET
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/echo", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "", string(respBody))
	require.NoError(t, resp.Body.Close())

	// POST without content-encoding
	req, err = http.NewRequestWithContext(
		ctx,
		"POST",
		server.URL+"/echo",
		bytes.NewBuffer([]byte("hello")),
	)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	respBody, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hello", string(respBody))
	require.NoError(t, resp.Body.Close())

	// POST with content-encoding gzip
	buf := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(buf)
	_, err = gzipWriter.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	req, err = http.NewRequestWithContext(
		ctx,
		"POST",
		server.URL+"/echo",
		buf,
	)
	require.NoError(t, err)

	req.Header.Set("Content-Encoding", "gzip")
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	respBody, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hello", string(respBody))
	require.NoError(t, resp.Body.Close())
}
