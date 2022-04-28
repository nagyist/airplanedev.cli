package httpd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gavv/httpexpect/v2"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func getHttpExpect(ctx context.Context, t *testing.T, router *mux.Router) *httpexpect.Expect {
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

func parseOutputs(s string, require *require.Assertions) []Output {
	var outputs []Output
	for _, s := range strings.Split(s, "\n") {
		if len(s) == 0 {
			continue
		}
		var output Output
		require.NoError(json.Unmarshal([]byte(s), &output))
		outputs = append(outputs, output)
	}
	return outputs
}

func TestExecuteCmdSimpleEcho(t *testing.T) {
	require := require.New(t)
	slots := make(chan interface{}, 1)
	slots <- true
	serverDoneC := make(chan interface{})
	h := getHttpExpect(
		context.Background(),
		t,
		Route("echo", []string{"hello"}, serverDoneC, slots),
	)
	body := h.POST("/execute").
		WithJSON(map[string]interface{}{}).
		Expect().
		Status(http.StatusOK).Body()

	outputs := parseOutputs(body.Raw(), require)
	execID := outputs[0].ExecutionID
	require.Equal(
		[]Output{
			{Msg: "", Type: OutputTypeSystem, ExecutionID: execID},
			{Msg: "hello", Type: OutputTypeStdout, ExecutionID: execID},
			{Msg: "", Type: OutputTypeExit, Status: OutputStatusSuccess, ExecutionID: execID},
		},
		outputs,
	)
}

func TestExecuteCmdLongPrint(t *testing.T) {
	require := require.New(t)
	slots := make(chan interface{}, 1)
	slots <- true
	serverDoneC := make(chan interface{})

	// Check that long output finishes correctly.
	const numOutputs = 1000
	var s string
	for i := 1; i < numOutputs; i++ {
		s = fmt.Sprintf("%s\n%d", s, i)
	}

	h := getHttpExpect(
		context.Background(),
		t,
		Route("printf", []string{}, slots, serverDoneC),
	)
	body := h.POST("/execute").
		WithJSON(ExecuteCmdRequest{
			Args: []string{s},
		}).
		Expect().
		Status(http.StatusOK).Body()

	outputs := parseOutputs(body.Raw(), require)
	// +1 for execID system message
	// +1 for exit message
	require.Len(outputs, numOutputs+2)
}

func TestExecuteCmdError(t *testing.T) {
	require := require.New(t)
	slots := make(chan interface{}, 1)
	slots <- true
	serverDoneC := make(chan interface{})

	h := getHttpExpect(
		context.Background(),
		t,
		Route("nonexistingbinary", []string{""}, slots, serverDoneC),
	)
	body := h.POST("/execute").
		WithJSON(map[string]interface{}{}).
		Expect().
		Status(http.StatusInternalServerError).Body()

	var responseErr errorResponse
	require.NoError(json.Unmarshal([]byte(body.Raw()), &responseErr))
	require.Equal(
		500,
		responseErr.Code,
	)
}

func TestExecuteCmdExitError(t *testing.T) {
	require := require.New(t)
	slots := make(chan interface{}, 1)
	slots <- true
	serverDoneC := make(chan interface{})

	h := getHttpExpect(
		context.Background(),
		t,
		Route("grep", []string{""}, slots, serverDoneC),
	)
	body := h.POST("/execute").
		WithJSON(ExecuteCmdRequest{
			Args: []string{"--non-existing-flag"},
		}).
		Expect().
		Status(http.StatusOK).Body()

	outputs := parseOutputs(body.Raw(), require)
	require.Equal(
		Output{
			Msg:         "exit status 2",
			Type:        OutputTypeExit,
			Status:      OutputStatusError,
			ExecutionID: outputs[0].ExecutionID,
		},
		outputs[len(outputs)-1],
	)
}
func TestExecuteCmdEnv(t *testing.T) {
	require := require.New(t)
	slots := make(chan interface{}, 1)
	slots <- true
	serverDoneC := make(chan interface{})

	h := getHttpExpect(
		context.Background(),
		t,
		Route("env", []string{}, slots, serverDoneC),
	)
	body := h.POST("/execute").
		WithJSON(ExecuteCmdRequest{
			Args: []string{},
			Env:  map[string]interface{}{"MY_FOO_ENVIRONMENT": "bar"},
		}).
		Expect().
		Status(http.StatusOK).Body()

	outputs := parseOutputs(body.Raw(), require)
	require.Contains(
		outputs,
		Output{Msg: "MY_FOO_ENVIRONMENT=bar", Type: OutputTypeStdout, ExecutionID: outputs[0].ExecutionID},
	)
}

func TestExecuteCmdContextCancel(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	slots := make(chan interface{}, 1)
	slots <- true
	serverDoneC := make(chan interface{})

	go func() {
		time.Sleep(time.Second)
		cancel()
	}()

	h := getHttpExpect(
		ctx,
		t,
		Route("sleep", []string{}, slots, serverDoneC),
	)
	body := h.POST("/execute").
		WithJSON(ExecuteCmdRequest{
			Args: []string{"100"},
		}).
		Expect().
		Status(http.StatusOK).Body()

	outputs := parseOutputs(body.Raw(), require)
	require.Equal(
		[]Output{
			{Msg: "", Type: OutputTypeSystem, ExecutionID: outputs[0].ExecutionID},
		},
		outputs,
	)
}

func TestExecuteCmdCancel(t *testing.T) {
	require := require.New(t)
	slots := make(chan interface{}, 1)
	slots <- true
	serverDoneC := make(chan interface{})

	h := getHttpExpect(
		context.Background(),
		t,
		Route("sleep", []string{}, slots, serverDoneC),
	)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		body := h.POST("/execute").
			WithJSON(ExecuteCmdRequest{
				Args: []string{"100"},
				Env:  map[string]interface{}{},
			}).
			Expect().
			Status(http.StatusOK).Body()

		outputs := parseOutputs(body.Raw(), require)
		require.Equal(
			[]Output{
				{Msg: "", Type: OutputTypeSystem, ExecutionID: outputs[0].ExecutionID},
				{Msg: "", Type: OutputTypeSystem, Status: OutputStatusCancelled, ExecutionID: outputs[0].ExecutionID},
			},
			outputs,
		)
	}()
	time.Sleep(time.Second)
	body := h.POST("/cancel").
		WithJSON(map[string]interface{}{}).
		Expect().
		Status(http.StatusOK).Body()
	require.Equal(
		"{}",
		body.Raw(),
	)
	wg.Wait()

	// Do it again
	wg.Add(1)
	go func() {
		defer wg.Done()
		body := h.POST("/execute").
			WithJSON(ExecuteCmdRequest{
				Args: []string{"100"},
				Env:  map[string]interface{}{},
			}).
			Expect().
			Status(http.StatusOK).Body()

		outputs := parseOutputs(body.Raw(), require)
		require.Equal(
			[]Output{
				{Msg: "", Type: OutputTypeSystem, ExecutionID: outputs[0].ExecutionID},
				{Msg: "", Type: OutputTypeSystem, Status: OutputStatusCancelled, ExecutionID: outputs[0].ExecutionID},
			},
			outputs,
		)
	}()
	time.Sleep(time.Second)
	body = h.POST("/cancel").
		WithJSON(map[string]interface{}{}).
		Expect().
		Status(http.StatusOK).Body()
	require.Equal(
		"{}",
		body.Raw(),
	)
	wg.Wait()
}

func TestExecuteCmdContextCancelThenCmdCancel(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	slots := make(chan interface{}, 1)
	slots <- true
	serverDoneC := make(chan interface{})

	go func() {
		time.Sleep(time.Second)
		cancel()
	}()

	h := getHttpExpect(
		ctx,
		t,
		Route("sleep", []string{}, slots, serverDoneC),
	)
	body := h.POST("/execute").
		WithJSON(ExecuteCmdRequest{
			Args: []string{"100"},
		}).
		Expect().
		Status(http.StatusOK).Body()

	outputs := parseOutputs(body.Raw(), require)
	require.Equal(
		[]Output{
			{Msg: "", Type: OutputTypeSystem, ExecutionID: outputs[0].ExecutionID},
		},
		outputs,
	)

	h = getHttpExpect(
		context.Background(),
		t,
		Route("sleep", []string{}, slots, serverDoneC),
	)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		body := h.POST("/execute").
			WithJSON(ExecuteCmdRequest{
				Args: []string{"100"},
				Env:  map[string]interface{}{},
			}).
			Expect().
			Status(http.StatusOK).Body()

		outputs := parseOutputs(body.Raw(), require)
		require.Equal(
			[]Output{
				{Msg: "", Type: OutputTypeSystem, ExecutionID: outputs[0].ExecutionID},
				{Msg: "", Type: OutputTypeSystem, Status: OutputStatusCancelled, ExecutionID: outputs[0].ExecutionID},
			},
			outputs,
		)
	}()
	time.Sleep(time.Second)
	body = h.POST("/cancel").
		WithJSON(map[string]interface{}{}).
		Expect().
		Status(http.StatusOK).Body()
	require.Equal(
		"{}",
		body.Raw(),
	)
	wg.Wait()
}
