package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var requiredHeaderValues = map[string]string{
	"X-Airplane-Client-Kind":    "test",
	"X-Airplane-Client-Version": "1",
}

func TestClientGet(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		require.Equal("GET", req.Method)
		require.Equal(req.Header.Get("X-Airplane-Client-Kind"), "test")
		require.Equal(req.Header.Get("X-Airplane-Client-Version"), "1")
		require.Equal(req.Header.Get("X-Req-Opts-Header"), "hello")
		require.Equal(req.Header.Get("User-Agent"), "airplane/test/1")
		require.Empty(req.Header.Get("Idempotency-Key"))
		require.Equal("/foobar", req.URL.String())
		_, _ = rw.Write([]byte(`OK`))
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:   requiredHeaderValues,
		UserAgent: "airplane/test/1",
	})
	body, err := client.Get(ctx, server.URL+"/foobar", ReqOpts{
		Headers: map[string]string{
			"X-Req-Opts-Header": "hello",
		},
		UserAgent: "airplane/test/1",
	})
	require.NoError(err)
	require.Equal("OK", string(body))
}

func TestClientGetJSON(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		require.Equal("GET", req.Method)
		require.Equal(req.Header.Get("X-Airplane-Client-Kind"), "test")
		require.Equal(req.Header.Get("X-Airplane-Client-Version"), "1")
		require.Equal(req.Header.Get("X-Req-Opts-Header"), "hello")
		require.Equal(req.Header.Get("User-Agent"), "airplane/test/1")
		require.Empty(req.Header.Get("Idempotency-Key"))
		require.Equal(req.Header.Get("Accept"), "application/json")
		require.Equal("/foobar", req.URL.String())
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write([]byte(`{"message": "hello world"}`))
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:   requiredHeaderValues,
		UserAgent: "airplane/test/1",
	})
	var resp struct {
		Message string `json:"message"`
	}
	err := client.GetJSON(ctx, server.URL+"/foobar", &resp, ReqOpts{
		Headers: map[string]string{
			"X-Req-Opts-Header": "hello",
		},
	})
	require.NoError(err)
	require.Equal("hello world", resp.Message)

	// GetJSON should accept `nil` as a resp value (in the case where `resp` is ignored)
	err = client.GetJSON(ctx, server.URL+"/foobar", nil, ReqOpts{
		Headers: map[string]string{
			"X-Req-Opts-Header": "hello",
		},
	})
	require.NoError(err)
}

func TestClientPost(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		require.Equal("POST", req.Method)
		body, err := io.ReadAll(req.Body)
		require.NoError(err)
		require.Equal("testing 123", string(body))
		require.Equal(req.Header.Get("X-Airplane-Client-Kind"), "test")
		require.Equal(req.Header.Get("X-Airplane-Client-Version"), "1")
		require.Equal(req.Header.Get("X-Req-Opts-Header"), "hello")
		require.Equal(req.Header.Get("User-Agent"), "airplane/test/1")
		require.NotEmpty(req.Header.Get("Idempotency-Key"))
		require.Equal("/foobar", req.URL.String())
		_, _ = rw.Write([]byte(`OK`))
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:   requiredHeaderValues,
		UserAgent: "airplane/test/1",
	})
	body, err := client.Post(ctx, server.URL+"/foobar", []byte("testing 123"), ReqOpts{
		Headers: map[string]string{
			"X-Req-Opts-Header": "hello",
		},
		UserAgent: "airplane/test/1",
	})
	require.NoError(err)
	require.Equal("OK", string(body))
}

func TestClientPostGZipLargeRequest(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	reqBody := []byte{}
	for i := 0; i < 2000; i++ {
		reqBody = append(reqBody, []byte(fmt.Sprintf("item %d", i))...)
	}

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		require.Equal("POST", req.Method)
		body, err := io.ReadAll(req.Body)
		require.NoError(err)

		gzippedBody, err := gzipBytes(reqBody)
		require.NoError(err)
		require.Equal(gzippedBody, body)

		require.Equal(req.Header.Get("Content-Encoding"), "gzip")
		require.Equal("/foobar", req.URL.String())
		_, _ = rw.Write([]byte(`OK`))
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:   requiredHeaderValues,
		UserAgent: "airplane/test/1",
	})
	body, err := client.Post(
		ctx,
		server.URL+"/foobar",
		reqBody,
		ReqOpts{},
	)
	require.NoError(err)
	require.Equal("OK", string(body))
}

func TestClientPostLargeRequestPreEncoded(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	reqBody := []byte{}
	for i := 0; i < 2000; i++ {
		reqBody = append(reqBody, []byte(fmt.Sprintf("item %d", i))...)
	}

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		require.Equal("POST", req.Method)
		body, err := io.ReadAll(req.Body)
		require.NoError(err)
		require.Equal(reqBody, body)
		require.Equal(req.Header.Get("Content-Encoding"), "unknown")
		require.Equal("/foobar", req.URL.String())
		_, _ = rw.Write([]byte(`OK`))
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers: map[string]string{
			"X-Airplane-Client-Kind":    "test",
			"X-Airplane-Client-Version": "1",
			"Content-Encoding":          "unknown",
		},
		UserAgent: "airplane/test/1",
	})
	body, err := client.Post(
		ctx,
		server.URL+"/foobar",
		reqBody,
		ReqOpts{},
	)
	require.NoError(err)
	require.Equal("OK", string(body))
}

func TestClientPostJSON(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	type content struct {
		Message string `json:"message"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		require.Equal("POST", req.Method)
		require.Equal(req.Header.Get("X-Airplane-Client-Kind"), "test")
		require.Equal(req.Header.Get("X-Airplane-Client-Version"), "1")
		require.Equal(req.Header.Get("X-Req-Opts-Header"), "hello")
		require.Equal(req.Header.Get("User-Agent"), "airplane/test/1")
		require.NotEmpty(req.Header.Get("Idempotency-Key"))
		require.Equal(req.Header.Get("Accept"), "application/json")
		body, err := io.ReadAll(req.Body)
		require.NoError(err)
		switch req.URL.Path {
		case "/foobar":
			var rb content
			require.NoError(json.Unmarshal(body, &rb))
			require.Equal("my name's world!", rb.Message)
		case "/empty":
			require.Empty(body)
		default:
			require.Fail("Unexpected pathname: %q", req.URL.Path)
		}
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write([]byte(`{"message": "hello world"}`))
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:   requiredHeaderValues,
		UserAgent: "airplane/test/1",
	})
	req := content{Message: "my name's world!"}
	var resp content
	err := client.PostJSON(ctx, server.URL+"/foobar", req, &resp, ReqOpts{
		Headers: map[string]string{
			"X-Req-Opts-Header": "hello",
		},
	})
	require.NoError(err)
	require.Equal("hello world", resp.Message)

	// PostJSON should accept `nil` as a req/resp value (in which case both should be ignored).
	err = client.PostJSON(ctx, server.URL+"/empty", nil, nil, ReqOpts{
		Headers: map[string]string{
			"X-Req-Opts-Header": "hello",
		},
	})
	require.NoError(err)
}

func TestClientPut(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		require.Equal("PUT", req.Method)
		body, err := io.ReadAll(req.Body)
		require.NoError(err)
		require.Equal("testing 123", string(body))
		require.Equal(req.Header.Get("X-Airplane-Client-Kind"), "test")
		require.Equal(req.Header.Get("X-Airplane-Client-Version"), "1")
		require.Equal(req.Header.Get("X-Req-Opts-Header"), "hello")
		require.Equal(req.Header.Get("User-Agent"), "airplane/test/1")
		require.Empty(req.Header.Get("Idempotency-Key"))
		require.Equal("/foobar", req.URL.String())
		_, _ = rw.Write([]byte(`OK`))
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:   requiredHeaderValues,
		UserAgent: "airplane/test/1",
	})
	body, err := client.Put(ctx, server.URL+"/foobar", []byte("testing 123"), ReqOpts{
		Headers: map[string]string{
			"X-Req-Opts-Header": "hello",
		},
		UserAgent: "airplane/test/1",
	})
	require.NoError(err)
	require.Equal("OK", string(body))
}

func TestClientPutJSON(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	type content struct {
		Message string `json:"message"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		require.Equal("PUT", req.Method)
		require.Equal(req.Header.Get("X-Airplane-Client-Kind"), "test")
		require.Equal(req.Header.Get("X-Airplane-Client-Version"), "1")
		require.Equal(req.Header.Get("X-Req-Opts-Header"), "hello")
		require.Equal(req.Header.Get("User-Agent"), "airplane/test/1")
		require.Empty(req.Header.Get("Idempotency-Key"))
		require.Equal(req.Header.Get("Accept"), "application/json")
		body, err := io.ReadAll(req.Body)
		require.NoError(err)
		switch req.URL.Path {
		case "/foobar":
			var rb content
			require.NoError(json.Unmarshal(body, &rb))
			require.Equal("my name's world!", rb.Message)
		case "/empty":
			require.Empty(body)
		default:
			require.Fail("Unexpected pathname: %q", req.URL.Path)
		}
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write([]byte(`{"message": "hello world"}`))
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:   requiredHeaderValues,
		UserAgent: "airplane/test/1",
	})
	req := content{Message: "my name's world!"}
	var resp content
	err := client.PutJSON(ctx, server.URL+"/foobar", req, &resp, ReqOpts{
		Headers: map[string]string{
			"X-Req-Opts-Header": "hello",
		},
	})
	require.NoError(err)
	require.Equal("hello world", resp.Message)

	// PutJSON should accept `nil` as a req/resp value (in which case both should be ignored).
	err = client.PutJSON(ctx, server.URL+"/empty", nil, nil, ReqOpts{
		Headers: map[string]string{
			"X-Req-Opts-Header": "hello",
		},
	})
	require.NoError(err)
}

func TestClientRetries(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	failuresRemaining := 15
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if failuresRemaining > 0 {
			failuresRemaining--
			rw.WriteHeader(500)
			_, _ = rw.Write([]byte("Internal server error"))
			return
		}
		_, _ = rw.Write([]byte(`OK`))
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:      requiredHeaderValues,
		UserAgent:    "airplane/test/1",
		retryWaitMin: time.Millisecond,
		retryWaitMax: time.Millisecond,
	})
	body, err := client.Get(ctx, server.URL+"/foobar", ReqOpts{})
	require.Nil(body)
	var errsc ErrStatusCode
	require.ErrorAs(err, &errsc)
	require.Equal(500, errsc.StatusCode)
	require.Equal("Internal server error", errsc.Msg)

	// Since the handler only fails 15 times, the next request will succeed halfway through retrying.
	body, err = client.Get(ctx, server.URL+"/foobar", ReqOpts{})
	require.NoError(err)
	require.Equal("OK", string(body))
}

func TestClientRetryHeader(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	failuresRemaining := 15
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if failuresRemaining > 0 {
			failuresRemaining--
			rw.Header().Set("X-Airplane-Retryable", "true")
			rw.WriteHeader(409)
			_, _ = rw.Write([]byte("Conflict"))
			return
		}
		rw.Header().Set("X-Airplane-Retryable", "false")
		_, _ = rw.Write([]byte(`OK`))
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:      requiredHeaderValues,
		UserAgent:    "airplane/test/1",
		retryWaitMin: time.Millisecond,
		retryWaitMax: time.Millisecond,
	})
	body, err := client.Get(ctx, server.URL+"/foobar", ReqOpts{})
	require.Nil(body)
	var errsc ErrStatusCode
	require.ErrorAs(err, &errsc)
	require.Equal(409, errsc.StatusCode)
	require.Equal("Conflict", errsc.Msg)

	// Since the handler only fails 15 times, the next request will succeed halfway through retrying.
	body, err = client.Get(ctx, server.URL+"/foobar", ReqOpts{})
	require.NoError(err)
	require.Equal("OK", string(body))
}

func TestClientRetryHeader500(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	retries := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		retries++
		rw.Header().Set("X-Airplane-Retryable", "false")
		rw.WriteHeader(500)
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:      requiredHeaderValues,
		UserAgent:    "airplane/test/1",
		retryWaitMin: time.Millisecond,
		retryWaitMax: time.Millisecond,
	})
	body, err := client.Get(ctx, server.URL+"/foobar", ReqOpts{})
	require.Nil(body)
	var errsc ErrStatusCode
	require.ErrorAs(err, &errsc)
	require.Equal(500, errsc.StatusCode)
	// This request should not have been retried because of the response header.
	require.Equal(1, retries)
}

func TestClientTimeout(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	var mu sync.Mutex
	failuresRemaining := 15
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		mu.Lock()
		if failuresRemaining > 0 {
			failuresRemaining--
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
		} else {
			mu.Unlock()
		}
		_, _ = rw.Write([]byte(`OK`))
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:      requiredHeaderValues,
		UserAgent:    "airplane/test/1",
		Timeout:      5 * time.Millisecond,
		retryWaitMin: time.Millisecond,
		retryWaitMax: time.Millisecond,
	})
	body, err := client.Get(ctx, server.URL+"/foobar", ReqOpts{})
	require.Error(err)
	require.Nil(body)
	// Assert that a timeout error was returned.
	var errnt net.Error
	require.ErrorAs(err, &errnt)
	require.True(errnt.Timeout())

	// Since the handler only fails 15 times, the next request will succeed halfway through retrying.
	body, err = client.Get(ctx, server.URL+"/foobar", ReqOpts{})
	require.NoError(err)
	require.Equal("OK", string(body))
}

func TestClientRequiredHeaders(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	{
		client := NewClient(ClientOpts{})
		body, err := client.Get(ctx, "http://localhost:50000/foobar", ReqOpts{})
		require.Nil(body)
		require.ErrorContains(err, `required header "X-Airplane-Client-Kind" not set`)
	}

	{
		client := NewClient(ClientOpts{
			Headers: map[string]string{
				"X-Airplane-Client-Kind": "test",
			},
		})
		body, err := client.Get(ctx, "http://localhost:50000/foobar", ReqOpts{})
		require.Nil(body)
		require.ErrorContains(err, `required header "X-Airplane-Client-Version" not set`)
	}

	{
		client := NewClient(ClientOpts{
			Headers: map[string]string{
				"X-Airplane-Client-Kind":    "test",
				"X-Airplane-Client-Version": "1",
			},
		})
		body, err := client.Get(ctx, "http://localhost:50000/foobar", ReqOpts{})
		require.Nil(body)
		require.ErrorContains(err, `required header "User-Agent" not set`)
	}

	{
		client := NewClient(ClientOpts{
			Headers: map[string]string{
				"X-Airplane-Client-Kind":    "test",
				"X-Airplane-Client-Version": "1",
				"User-Agent":                "invalid",
			},
		})
		body, err := client.Get(ctx, "http://localhost:50000/foobar", ReqOpts{})
		require.Nil(body)
		require.ErrorContains(err, `User-Agent must start with "airplane/"`)
	}
}

func TestClientLogHooks(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	failuresRemaining := 15
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if failuresRemaining > 0 {
			failuresRemaining--
			rw.WriteHeader(500)
			_, _ = rw.Write([]byte("Internal server error"))
			return
		}
		_, _ = rw.Write([]byte(`OK`))
	}))
	defer server.Close()

	// Ensure that both log hooks are called for each retry.
	requestLogHookCalled := 0
	responseLogHookCalled := 0
	client := NewClient(ClientOpts{
		Headers:      requiredHeaderValues,
		UserAgent:    "airplane/test/1",
		retryWaitMin: time.Millisecond,
		retryWaitMax: time.Millisecond,
		RequestLogHook: func(req *http.Request, attempt int) {
			requestLogHookCalled++
		},
		ResponseLogHook: func(resp *http.Response) {
			responseLogHookCalled++
		},
	})

	_, err := client.Get(ctx, server.URL+"/foobar", ReqOpts{})
	require.Error(err)

	_, err = client.Get(ctx, server.URL+"/foobar", ReqOpts{})
	require.NoError(err)

	// 15 failures + 1 success = 16 calls
	require.Equal(16, requestLogHookCalled)
	require.Equal(16, responseLogHookCalled)
}

func TestClientErrors(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/plain":
			rw.Header().Set("Content-Type", "text/plain")
			_, _ = rw.Write([]byte(`OK`))
		case "/none":
			rw.Header().Set("Content-Type", "")
			_, _ = rw.Write([]byte(`OK`))
		case "/error/json":
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(500)
			_, _ = rw.Write([]byte(`{"error": "hello world"}`))
		case "/error/plain":
			rw.Header().Set("Content-Type", "text/plain")
			rw.WriteHeader(500)
			_, _ = rw.Write([]byte(`Something went wrong...`))
		case "/error/none":
			rw.Header().Set("Content-Type", "")
			rw.WriteHeader(500)
			_, _ = rw.Write([]byte(`No clue what happened`))
		default:
			require.Fail("Unexpected pathname: %q", req.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(ClientOpts{
		Headers:      requiredHeaderValues,
		UserAgent:    "airplane/test/1",
		retryWaitMin: time.Millisecond,
		retryWaitMax: time.Millisecond,
	})

	{
		// If a JSON call gets an HTTP response with a non-JSON Content-Type, error.
		var resp struct{}
		err := client.GetJSON(ctx, server.URL+"/plain", &resp, ReqOpts{})
		require.ErrorContains(err, `expected "application/json" response: got "text/plain"`)
	}

	{
		// If a JSON call gets an HTTP response without a Content-Type, error.
		var resp struct{}
		err := client.GetJSON(ctx, server.URL+"/none", &resp, ReqOpts{})
		require.ErrorContains(err, `expected "application/json" response: got ""`)
	}

	{
		// If a client method gets an HTTP response with a JSON Content-Type, error.
		body, err := client.Get(ctx, server.URL+"/error/json", ReqOpts{})
		var errsc ErrStatusCode
		require.ErrorAs(err, &errsc)
		require.Equal(500, errsc.StatusCode)
		require.Equal("hello world", errsc.Msg)
		require.Nil(body)
	}

	{
		// If a client method gets an HTTP response with a non-JSON Content-Type, error.
		body, err := client.Get(ctx, server.URL+"/error/plain", ReqOpts{})
		var errsc ErrStatusCode
		require.ErrorAs(err, &errsc)
		require.Equal(500, errsc.StatusCode)
		require.Equal("Something went wrong...", errsc.Msg)
		require.Nil(body)
	}

	{
		// If a client method gets an HTTP response without a Content-Type, error.
		body, err := client.Get(ctx, server.URL+"/error/none", ReqOpts{})
		var errsc ErrStatusCode
		require.ErrorAs(err, &errsc)
		require.Equal(500, errsc.StatusCode)
		require.Equal("No clue what happened", errsc.Msg)
		require.Nil(body)
	}
}
