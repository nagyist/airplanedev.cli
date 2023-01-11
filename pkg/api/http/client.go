package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/pkg/errors"
)

// RequiredHeaders are HTTP headers that must be set on every HTTP request.
var RequiredHeaders = []string{"X-Airplane-Client-Kind", "X-Airplane-Client-Version", "User-Agent"}

// Client is an HTTP client that can be used to issue HTTP requests to the Airplane API.
type Client struct {
	opts ClientOpts
	http *retryablehttp.Client
}

type ClientOpts struct {
	// Headers are HTTP headers to attach to each request. Certain HTTP headers are required
	// and will return an error if not set (see RequiredHeaders).
	Headers map[string]string
	// RequestLogHook is an optional hook that is called before each HTTP request. This hook
	// is called once per retry. Attempt starts at one.
	RequestLogHook func(req *http.Request, attempt int)
	// ResponseLogHook is an optional hook that is called after each HTTP response. This hook
	// is called once per retry, but will not be called if an error is returned. If you read or
	// close resp.Body, ensure you reset resp.Body.
	ResponseLogHook func(resp *http.Response)
	// Timeout is the maximum amount of time spent performing a single request. The total
	// cumulative time may be much more due to retries.
	//
	// A negative value will disable the timeout entirely.
	//
	// Defaults to 10s.
	Timeout time.Duration
	// UserAgent is required and determines the value of the `User-Agent` header sent in
	// HTTP requests. If set, this takes precedence over ClientOpts.UserAgent or a
	// `User-Agent` header directly set on Headers.
	//
	// In general, this should be in the form of:
	//   airplane/[SERVICE]/[SERVICE_VERSION] (FIELD/[FIELD_VALUE])*
	// Fields are optional and are used to identify where the HTTP request is originating
	// from. Some common fields are `team/TEAM_ID`, `run/RUN_ID`, and `build/BUILD_ID`, e.g.
	// "airplane/cli/v0.1.4 team/tea123".
	UserAgent string

	// retryWaitMin overrides the minimum wait time between retries. Used for testing purposes only.
	retryWaitMin time.Duration
	// retryWaitMax overrides the maximum wait time between retries. Used for testing purposes only.
	retryWaitMax time.Duration
}

// NewClient creates a new Airplane HTTP client that can be used to issue HTTP requests.
func NewClient(opts ClientOpts) Client {
	if opts.retryWaitMin == 0 {
		opts.retryWaitMin = 100 * time.Millisecond
	}
	if opts.retryWaitMax == 0 {
		opts.retryWaitMax = 30 * time.Second
	}

	// Only disable the timeout if a negative value is explicitly passed. Otherwise,
	// default to a reasonable amount.
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.Timeout < 0 {
		opts.Timeout = 0
	}

	rhc := retryablehttp.NewClient()

	rhc.Backoff = backoffExponential
	rhc.RetryWaitMin = opts.retryWaitMin
	rhc.RetryWaitMax = opts.retryWaitMax
	// Perform up to 9 retries (10 total attempts).
	rhc.RetryMax = 9

	// Disable retryablehttp's automatic error wrapping, otherwise we don't get an *http.Response
	// when all retries fail which prevents us from initializing an ErrStatusCode.
	rhc.ErrorHandler = retryablehttp.PassthroughErrorHandler

	// Surface the underlying error message on failure.
	rhc.CheckRetry = errorPropagatedRetryPolicy

	// Disable logging.
	rhc.Logger = nil

	// Configure a per-request timeout.
	//
	// Currently, retryablehttp does not support a per-request timeout. If you need to support multiple timeouts,
	// create multiple Clients.
	rhc.HTTPClient.Timeout = opts.Timeout

	// Attach optional logging hooks.
	if opts.RequestLogHook != nil {
		rhc.RequestLogHook = func(l retryablehttp.Logger, r *http.Request, i int) {
			// Increment `i` so it starts at 1.
			opts.RequestLogHook(r, i+1)
		}
	}
	if opts.ResponseLogHook != nil {
		rhc.ResponseLogHook = func(l retryablehttp.Logger, r *http.Response) {
			opts.ResponseLogHook(r)
		}
	}

	return Client{
		opts: opts,
		http: rhc,
	}
}

type ReqOpts struct {
	// Headers are HTTP headers to attach to this request. Certain HTTP headers are required
	// and will return an error if not set (see RequiredHeaders).
	//
	// These headers take precedence over conflicting headers set in ClientOpts.
	Headers map[string]string
	// UserAgent is required and determines the value of the `User-Agent` header sent in
	// HTTP requests. If set, this takes precedence over ClientOpts.UserAgent or a
	// `User-Agent` header directly set on Headers.
	//
	// In general, this should be in the form of:
	//   airplane/[SERVICE]/[SERVICE_VERSION] (FIELD/[FIELD_VALUE])*
	// Fields are optional and are used to identify where the HTTP request is originating
	// from. Some common fields are `team/TEAM_ID`, `run/RUN_ID`, and `build/BUILD_ID`, e.g.
	// "airplane/cli/v0.1.4 team/tea123".
	UserAgent string
}

// Get issues an HTTP GET request to the Airplane API.
//
// If the API returns a non-2xx status code, an ErrStatusCode error will be returned.
// For example, you can check if a response returned a 404 with the following:
//
//	_, err := client.Get(ctx, /*...*/)
//	var errsc libhttp.ErrStatusCode
//	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
//		// ...
//	} else if err != nil {
//		// ...
//	}
//
// If you expect a JSON response, use GetJSON instead.
func (c Client) Get(ctx context.Context, url string, opts ReqOpts) ([]byte, error) {
	body, _, err := c.do(ctx, http.MethodGet, url, nil, opts)
	return body, err
}

// GetJSON issues an HTTP GET request to the Airplane API and unmarshals a the response body
// as JSON into `resp`.
//
// If the response body is not necessary, `resp` can be set to `nil` to avoid unmarshalling.
//
// If the API returns a non-2xx status code, an ErrStatusCode error will be returned.
// For example, you can check if a response returned a 404 with the following:
//
//	err := client.GetJSON(ctx, /*...*/)
//	var errsc libhttp.ErrStatusCode
//	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
//		// ...
//	} else if err != nil {
//		// ...
//	}
func (c Client) GetJSON(ctx context.Context, url string, resp any, opts ReqOpts) error {
	return c.doJSON(ctx, http.MethodGet, url, nil, resp, opts)
}

// Post issues an HTTP POST request to the Airplane API with the provided request body.
//
// If the API returns a non-2xx status code, an ErrStatusCode error will be returned.
// For example, you can check if a response returned a 404 with the following:
//
//	_, err := client.Post(ctx, /*...*/)
//	var errsc libhttp.ErrStatusCode
//	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
//		// ...
//	} else if err != nil {
//		// ...
//	}
//
// If you expect a JSON response, use PostJSON instead.
func (c Client) Post(ctx context.Context, url string, req []byte, opts ReqOpts) ([]byte, error) {
	body, _, err := c.do(ctx, http.MethodPost, url, req, opts)
	return body, err
}

// PostJSON issues an HTTP POST request to the Airplane API by serializing `req` as JSON in the HTTP request
// body and unmarshalling the response body as JSON into `resp`.
//
// If `req` is `nil`, the request will not include a body.
//
// If the response body is not necessary, `resp` can be set to `nil` to avoid unmarshalling.
//
// If the API returns a non-2xx status code, an ErrStatusCode error will be returned.
// For example, you can check if a response returned a 404 with the following:
//
//	err := client.PostJSON(ctx, /*...*/)
//	var errsc libhttp.ErrStatusCode
//	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
//		// ...
//	} else if err != nil {
//		// ...
//	}
func (c Client) PostJSON(ctx context.Context, url string, req any, resp any, opts ReqOpts) error {
	var serializedReq []byte
	if req != nil {
		if opts.Headers == nil {
			opts.Headers = map[string]string{}
		}
		opts.Headers["Content-Type"] = "application/json"

		var err error
		serializedReq, err = json.Marshal(req)
		if err != nil {
			return errors.Wrap(err, "marshalling request body as JSON")
		}
	}

	return c.doJSON(ctx, http.MethodPost, url, serializedReq, resp, opts)
}

// Put issues an HTTP PUT request to the Airplane API with the provided request body.
//
// If the API returns a non-2xx status code, an ErrStatusCode error will be returned.
// For example, you can check if a response returned a 404 with the following:
//
//	_, err := client.Put(ctx, /*...*/)
//	var errsc libhttp.ErrStatusCode
//	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
//		// ...
//	} else if err != nil {
//		// ...
//	}
//
// If you expect a JSON response, use PutJSON instead.
func (c Client) Put(ctx context.Context, url string, req []byte, opts ReqOpts) ([]byte, error) {
	body, _, err := c.do(ctx, http.MethodPut, url, req, opts)
	return body, err
}

// PutJSON issues an HTTP PUT request to the Airplane API by serializing `req` as JSON in the HTTP request
// body and unmarshalling the response body as JSON into `resp`.
//
// If `req` is `nil`, the request will not include a body.
//
// If the response body is not necessary, `resp` can be set to `nil` to avoid unmarshalling.
//
// If the API returns a non-2xx status code, an ErrStatusCode error will be returned.
// For example, you can check if a response returned a 404 with the following:
//
//	err := client.PutJSON(ctx, /*...*/)
//	var errsc libhttp.ErrStatusCode
//	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
//		// ...
//	} else if err != nil {
//		// ...
//	}
func (c Client) PutJSON(ctx context.Context, url string, req any, resp any, opts ReqOpts) error {
	var serializedReq []byte
	if req != nil {
		if opts.Headers == nil {
			opts.Headers = map[string]string{}
		}
		opts.Headers["Content-Type"] = "application/json"

		var err error
		serializedReq, err = json.Marshal(req)
		if err != nil {
			return errors.Wrap(err, "marshalling request body as JSON")
		}
	}

	return c.doJSON(ctx, http.MethodPut, url, serializedReq, resp, opts)
}

func (c Client) doJSON(ctx context.Context, method string, url string, req []byte, resp any, opts ReqOpts) error {
	if opts.Headers == nil {
		opts.Headers = map[string]string{}
	}
	opts.Headers["Accept"] = "application/json"

	body, headers, err := c.do(ctx, method, url, req, opts)
	if err != nil {
		return err
	}

	if resp != nil {
		if !isJSONContentType(headers) {
			// The API returned a 2xx, but the body is not JSON (or a Content-Type
			// header was not set). This is unexpected.
			return errors.Errorf(`expected "application/json" response: got %q`, headers.Get("Content-Type"))
		}

		if err := json.Unmarshal(body, &resp); err != nil {
			return errors.Wrap(err, "unmarshalling response body as JSON")
		}
	}

	return nil
}

func (c Client) do(ctx context.Context, method string, url string, req []byte, opts ReqOpts) ([]byte, http.Header, error) {
	httpreq, err := retryablehttp.NewRequestWithContext(ctx, method, url, req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "initializing HTTP request")
	}

	// Attach a GetBody method so that the net/http can automatically retry requests
	// that encounter certain connection errors:
	// https://github.com/golang/go/blob/f721fa3be9bb52524f97b409606f9423437535e8/src/net/http/request.go#L1441-L1455
	if req != nil {
		httpreq.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(req)), nil
		}
	}

	for k, v := range c.opts.Headers {
		httpreq.Header.Set(k, v)
	}
	if c.opts.UserAgent != "" {
		httpreq.Header.Set("User-Agent", c.opts.UserAgent)
	}
	for k, v := range opts.Headers {
		httpreq.Header.Set(k, v)
	}
	if opts.UserAgent != "" {
		httpreq.Header.Set("User-Agent", opts.UserAgent)
	}

	// Set an Idempotency-Key header. This will ensure we can safely retry all HTTP requests.
	switch method {
	// Idempotency keys are not necessary for idempotent-by-definition methods, such as GET/DELETE.
	case http.MethodPost, http.MethodPatch:
		idempotencyKey, err := uuid.NewRandom()
		if err != nil {
			return nil, nil, errors.Wrap(err, "generating idempotency key")
		}
		httpreq.Header.Set("Idempotency-Key", idempotencyKey.String())
	}

	// Validate headers
	for _, h := range RequiredHeaders {
		if v := httpreq.Header.Get(h); v == "" {
			return nil, nil, errors.Errorf("required header %q not set", h)
		}
	}
	if err := validateUserAgent(httpreq.Header.Get("User-Agent")); err != nil {
		return nil, nil, err
	}

	resp, err := c.http.Do(httpreq)
	if err != nil {
		return nil, nil, errors.Wrap(err, "performing HTTP request")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, errors.Wrap(err, "reading response body")
		}

		return body, resp.Header, nil
	}

	return nil, nil, NewErrStatusCodeFromResponse(resp)
}

// backoffExponential is a retryablehttp.Backoff function that produces an exponential
// backoff policy with jitter.
//
// Inspired by: https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/
func backoffExponential(min, max time.Duration, attempt int, resp *http.Response) time.Duration {
	if max <= min {
		return min
	}

	jitter := rand.Float64()
	// Note: attempt starts at zero.
	duration := min * time.Duration(math.Pow(2, float64(attempt)))
	if duration > max {
		duration = max
	}
	return time.Duration(jitter * float64(duration))
}

// errorPropagatedRetryPolicy is a modified version of retryablehttp.ErrorPropagatedRetryPolicy.
func errorPropagatedRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	retry, err := retryablehttp.ErrorPropagatedRetryPolicy(ctx, resp, err)

	// ErrorPropagatedRetryPolicy will return an "unexpected HTTP status" error when a 5xx error
	// is returned from the server. This makes converting 5xx errors into ErrStatusCode harder,
	// because that error will get returned rather than an `*http.Response` that we can inspect
	// like we do for 4xx errors. We overwrite this behavior here.
	if err != nil && strings.HasPrefix(err.Error(), "unexpected HTTP status") {
		return retry, nil
	}

	return retry, err
}

func isJSONContentType(header http.Header) bool {
	return strings.Contains(header.Get("Content-Type"), "application/json")
}

func validateUserAgent(header string) error {
	if !strings.HasPrefix(header, "airplane/") {
		return errors.New(`User-Agent must start with "airplane/"`)
	}

	return nil
}
