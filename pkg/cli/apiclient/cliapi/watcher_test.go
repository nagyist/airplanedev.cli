package api

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/airplanedev/ojson"
	"github.com/stretchr/testify/require"
)

func init() {
	fetchInterval = time.Nanosecond
}

func TestWatcher(t *testing.T) {
	var ctx = context.Background()
	var assert = require.New(t)
	var lcm = logsClientMock{}

	var a = LogItem{
		Timestamp: time.Now().Add(time.Second),
		InsertID:  "001",
		Text:      "A",
	}
	var b = LogItem{
		Timestamp: time.Now().Add(2 * time.Second),
		InsertID:  "002",
		Text:      "B",
	}

	var responses = []GetLogsResponse{
		{Logs: []LogItem{a}, PrevPageToken: "001"},
		{Logs: []LogItem{b}, PrevPageToken: "002"},
	}

	outputs := GetOutputsResponse{
		Outputs: Outputs{
			V: ojson.NewObject().SetAndReturn("output", []interface{}{
				ojson.NewObject().SetAndReturn("test key", "test value"),
			}),
		},
	}

	var reqs int64
	lcm.getLogs = func(runID, prevToken string) (GetLogsResponse, error) {
		atomic.AddInt64(&reqs, 1)
		if prevToken == "" {
			return responses[0], nil
		}
		foundResponseIndex := -1
		for i, r := range responses {
			if prevToken == r.Logs[0].InsertID {
				foundResponseIndex = i
			}
		}
		if foundResponseIndex == -1 {
			return GetLogsResponse{}, nil
		}
		if foundResponseIndex+1 == len(responses) {
			return GetLogsResponse{}, nil
		}
		return responses[foundResponseIndex+1], nil
	}

	lcm.getRun = func(string) (resp GetRunResponse, err error) {
		var run = Run{Status: RunActive}

		if atomic.LoadInt64(&reqs) > 1 {
			run.Status = RunSucceeded
		}

		return GetRunResponse{run}, nil
	}

	lcm.getOutputs = func(string) (GetOutputsResponse, error) {
		fmt.Println("get outputs", outputs)
		return outputs, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var w = newWatcher(ctx, lcm, "run_id")
	var state RunState
	var printed []string

	for {
		if state = w.Next(); state.Err() != nil {
			break
		}

		for _, l := range state.Logs {
			printed = append(printed, l.Text)
		}

		if state.Stopped() {
			break
		}
	}

	assert.NoError(state.Err())
	assert.Equal([]string{"A", "B"}, printed)
}

type logsClientMock struct {
	getLogs    func(runID string, s string) (GetLogsResponse, error)
	getRun     func(runID string) (GetRunResponse, error)
	getOutputs func(runID string) (GetOutputsResponse, error)
}

func (lcm logsClientMock) GetLogs(ctx context.Context, runID, s string) (GetLogsResponse, error) {
	return lcm.getLogs(runID, s)
}

func (lcm logsClientMock) GetRun(ctx context.Context, runID string) (GetRunResponse, error) {
	return lcm.getRun(runID)
}

func (lcm logsClientMock) GetOutputs(ctx context.Context, runID string) (GetOutputsResponse, error) {
	return lcm.getOutputs(runID)
}
