package server

import (
	"os"
	"sync"

	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/lib/pkg/deploy/discover"
)

type State struct {
	cliConfig *cli.Config
	envSlug   string
	executor  dev.Executor
	port      int
	runs      *runsStore
	// Mapping from task slug to task config
	taskConfigs map[string]discover.TaskConfig
	// Mapping from view slug to view config
	viewConfigs map[string]discover.ViewConfig
	devConfig   conf.DevConfig
	viteProcess *os.Process
	viteMutex   sync.Mutex
}

// TODO: add limit on max items
type runsStore struct {
	runs map[string]LocalRun
	// A tasks's previous runs
	runHistory map[string][]string
}

func NewRunStore() *runsStore {
	r := &runsStore{
		runs:       map[string]LocalRun{},
		runHistory: map[string][]string{},
	}
	return r
}

func (store *runsStore) add(taskID string, runID string, run LocalRun) {
	store.runs[runID] = run
	if _, ok := store.runHistory[taskID]; !ok {
		store.runHistory[taskID] = make([]string, 0)
	}
	store.runHistory[taskID] = append([]string{runID}, store.runHistory[taskID]...)
	return
}

func (store *runsStore) get(runID string) (LocalRun, bool) {
	res, ok := store.runs[runID]
	return res, ok
}

func (store *runsStore) getRunHistory(taskID string) []LocalRun {
	runIDs := store.runHistory[taskID]
	res := make([]LocalRun, len(runIDs))
	for i, runID := range runIDs {
		res[i] = store.runs[runID]
	}
	return res

}
