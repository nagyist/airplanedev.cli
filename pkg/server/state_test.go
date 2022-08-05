package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	s := NewRunStore()
	task1 := "task1"
	testRuns := []LocalRun{
		{Status: api.RunFailed},
		{Status: api.RunSucceeded},
		{Status: api.RunFailed, CreatedAt: time.Now()},
		{Status: api.RunNotStarted},
	}
	for i, run := range testRuns {
		s.add(task1, fmt.Sprintf("run_%v", i), run)
	}
	result, ok := s.get("run_0")
	require.Equal(t, testRuns[0], result)

	runHistory := s.getRunHistory(task1)
	require.True(t, ok)
	require.Equal(t, 4, len(runHistory))
	for i := range runHistory {
		// runHistory is ordered by most recent
		require.EqualValues(t, runHistory[i], testRuns[len(testRuns)-i-1])
	}

	task2 := "task2"
	run2 := LocalRun{Status: api.RunSucceeded}
	runID2 := "task2_run"
	s.add(task2, "task2_run", run2)
	result2, ok := s.get(runID2)
	require.Equal(t, run2, result2)

	runHistory = s.getRunHistory(task2)
	require.True(t, ok)
	require.Equal(t, 1, len(runHistory))
	require.EqualValues(t, runHistory[0], run2)

}

func TestStoreGet(t *testing.T) {
	emptyStore := NewRunStore()
	_, ok := emptyStore.get("runID1")
	require.False(t, ok)
	runHistory := emptyStore.getRunHistory("taskID")
	require.Empty(t, runHistory)
	emptyStore.add("task", "run", LocalRun{})

}
