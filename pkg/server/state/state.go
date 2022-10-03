package state

import (
	"os"
	"sync"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/version"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	lrucache "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
)

type ViteContext struct {
	ServerURL string
	Process   *os.Process
}

type State struct {
	CliConfig   *cli.Config
	LocalClient *api.Client
	// Directory from which tasks and views were discovered
	Dir      string
	Executor dev.Executor
	Port     int
	Runs     *runsStore
	// Mapping from task slug to task config
	TaskConfigs map[string]discover.TaskConfig
	// Mapping from view slug to view config
	ViewConfigs  map[string]discover.ViewConfig
	DevConfig    *conf.DevConfig
	ViteContexts *lrucache.Cache
	ViteMutex    sync.Mutex
	Logger       logger.Logger

	EnvID   string
	EnvSlug string

	AuthInfo     api.AuthInfoResponse
	VersionCache version.Cache
}

// TODO: add limit on max items
type runsStore struct {
	// All runs
	runs map[string]dev.LocalRun
	// A task's previous runs
	runHistory map[string][]string
	// A run's descendants
	runDescendants map[string][]string

	mu sync.Mutex
}

func NewRunStore() *runsStore {
	r := &runsStore{
		runs:           map[string]dev.LocalRun{},
		runHistory:     map[string][]string{},
		runDescendants: map[string][]string{},
	}
	return r
}

func contains(runID string, history []string) bool {
	for _, id := range history {
		if id == runID {
			return true
		}
	}
	return false
}

func (store *runsStore) Add(taskSlug string, runID string, run dev.LocalRun) {
	store.mu.Lock()
	defer store.mu.Unlock()
	run.RunID = runID
	store.runs[runID] = run
	if _, ok := store.runHistory[taskSlug]; !ok {
		store.runHistory[taskSlug] = make([]string, 0)
	}
	if !contains(runID, store.runHistory[taskSlug]) {
		store.runHistory[taskSlug] = append([]string{runID}, store.runHistory[taskSlug]...)
	}

	if run.ParentID != "" {
		// attach run to parent
		store.runDescendants[run.ParentID] = append(store.runDescendants[run.ParentID], runID)
	}
}

func (store *runsStore) Get(runID string) (dev.LocalRun, bool) {
	res, ok := store.runs[runID]
	return res, ok
}

func (store *runsStore) GetDescendants(runID string) []dev.LocalRun {
	descendants := []dev.LocalRun{}
	descIDs, ok := store.runDescendants[runID]
	if !ok {
		return []dev.LocalRun{}
	}
	for _, descID := range descIDs {
		descendants = append(descendants, store.runs[descID])
	}
	return descendants
}

func (store *runsStore) Update(runID string, f func(run *dev.LocalRun) error) (dev.LocalRun, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	res, ok := store.runs[runID]
	if !ok {
		return dev.LocalRun{}, errors.Errorf("run with id %q not found", runID)
	}
	if err := f(&res); err != nil {
		return dev.LocalRun{}, err
	}
	store.runs[runID] = res

	return res, nil
}

func (store *runsStore) GetRunHistory(taskID string) []dev.LocalRun {
	runIDs := store.runHistory[taskID]
	res := make([]dev.LocalRun, len(runIDs))
	for i, runID := range runIDs {
		res[i] = store.runs[runID]
	}

	return res
}
