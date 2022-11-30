package state

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/server/dev_errors"
	"github.com/airplanedev/cli/pkg/version"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/bep/debounce"
	lrucache "github.com/hashicorp/golang-lru/v2"
	"github.com/pkg/errors"
)

type ViteContext struct {
	Closer    io.Closer
	ServerURL string
	Process   *os.Process
}

type State struct {
	LocalClient  *api.Client
	RemoteClient api.APIClient
	RemoteEnv    libapi.Env
	// We need UseFallbackEnv because there will always be a RemoteEnv (to look for a user's team's default resources).
	// UseFallbackEnv determines whether we should use this remote env for tasks/resources/etc. outside of these
	// defaults.
	UseFallbackEnv bool

	// Directory from which tasks and views were discovered
	Dir      string
	Executor dev.Executor

	Runs *runsStore
	// Mapping from task slug to task config
	TaskConfigs Store[string, discover.TaskConfig]
	// Mapping from view slug to view config
	ViewConfigs Store[string, discover.ViewConfig]
	// AppCondition holds info about task such as errors to display and time registered
	AppCondition Store[string, AppCondition]

	Discoverer *discover.Discoverer
	//Debouncer returns the debouncing function for a given key
	Debouncer DebounceStore

	DevConfig *conf.DevConfig
	// ViteContexts is an in-memory cache that maps view slugs to vite contexts.
	ViteContexts *lrucache.Cache[string, ViteContext]
	Logger       logger.Logger

	AuthInfo     api.AuthInfoResponse
	VersionCache version.Cache
}

type AppCondition struct {
	RefreshedAt time.Time
	Errors      []dev_errors.AppError
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

// DebounceStore keeps a mapping of keys to debounce functions
// A debouncer takes in a function and executes it when the debouncer stops being called after X duration.
// The debounce function can be called with different functions, but the last one will win.
type DebounceStore struct {
	// Mapping of key to debounce function
	debouncers map[string]func(f func())
	mu         sync.Mutex
}

func NewDebouncer() DebounceStore {
	return DebounceStore{debouncers: map[string]func(f func()){}}
}

const DefaultDebounceDuration = time.Second * 1

// Get will return the debounce function for a given key
// If it does not exist, it will create one
func (r *DebounceStore) Get(key string) func(f func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn, exists := r.debouncers[key]
	if exists {
		return fn
	} else {
		debouncer := debounce.New(DefaultDebounceDuration)
		r.debouncers[key] = debouncer
		return debouncer
	}
}
