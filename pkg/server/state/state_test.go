package state

import (
	"fmt"
	"testing"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/stretchr/testify/require"
)

func TestRunStore(t *testing.T) {
	s := NewRunStore()
	task1 := "task1"
	testRuns := []dev.LocalRun{
		{RunID: "run_0", TaskID: task1, Status: api.RunFailed},
		{RunID: "run_1", TaskID: task1, Status: api.RunSucceeded},
		{RunID: "run_2", TaskID: task1, Status: api.RunFailed, CreatedAt: time.Now()},
		{RunID: "run_3", TaskID: task1, Status: api.RunNotStarted},
	}
	for i, run := range testRuns {
		s.Add(task1, fmt.Sprintf("run_%v", i), run)
	}
	result, ok := s.Get("run_0")
	require.Equal(t, testRuns[0], result)

	runHistory := s.GetRunHistory(task1)
	require.True(t, ok)
	require.Equal(t, 4, len(runHistory))
	for i := range runHistory {
		// runHistory is ordered by most recent
		require.EqualValues(t, runHistory[i], testRuns[len(testRuns)-i-1])
	}

	task2 := "task2"
	runID2 := "task2_run"
	run2 := dev.LocalRun{RunID: runID2, TaskID: task2, Status: api.RunSucceeded}
	s.Add(task2, "task2_run", run2)
	result2, ok := s.Get(runID2)
	require.Equal(t, run2, result2)

	runHistory = s.GetRunHistory(task2)
	require.True(t, ok)
	require.Equal(t, 1, len(runHistory))
	require.EqualValues(t, runHistory[0], run2)

}

func TestRunStoreGet(t *testing.T) {
	emptyStore := NewRunStore()
	_, ok := emptyStore.Get("runID1")
	require.False(t, ok)
	runHistory := emptyStore.GetRunHistory("taskID")
	require.Empty(t, runHistory)
	emptyStore.Add("task", "run", dev.LocalRun{})
}

func TestRunStoreDupes(t *testing.T) {
	store := NewRunStore()
	taskID := "task_1"
	runID := "run_1"
	store.Add(taskID, runID, dev.LocalRun{})
	store.Add(taskID, runID, dev.LocalRun{})
	runHistory := store.GetRunHistory(taskID)
	require.Len(t, runHistory, 1)
}

func TestRunStoreUpdate(t *testing.T) {
	store := NewRunStore()
	taskID := "task1"
	runID := "task1_run"
	run := dev.LocalRun{RunID: runID, TaskID: taskID, Status: api.RunQueued}

	store.Add(taskID, runID, run)
	res, ok := store.Get(runID)
	require.True(t, ok)
	require.Equal(t, run, res)

	now := time.Now()
	updatedRun := dev.LocalRun{RunID: runID, TaskID: taskID, Status: api.RunFailed, FailedAt: &now}
	_, err := store.Update(runID, func(run *dev.LocalRun) error {
		*run = updatedRun
		return nil
	})
	require.NoError(t, err)
	res, ok = store.Get(runID)
	require.True(t, ok)
	require.Equal(t, updatedRun, res)
}
