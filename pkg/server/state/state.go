package state

import (
	"context"
	"fmt"
	"io"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/resources"
	"github.com/airplanedev/cli/pkg/server/dev_errors"
	"github.com/airplanedev/cli/pkg/server/network"
	"github.com/airplanedev/cli/pkg/server/status"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/airplanedev/cli/pkg/version"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/bundlediscover"
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
	LocalClient          *api.Client
	RemoteClient         api.APIClient
	InitialRemoteEnvSlug *string
	StudioURL            url.URL

	EnvCache Store[string, libapi.Env]

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

	Discoverer       *discover.Discoverer
	BundleDiscoverer *bundlediscover.Discoverer
	//Debouncer returns the debouncing function for a given key
	Debouncer DebounceStore

	DevConfig *conf.DevConfig
	// ViteContexts is an in-memory cache that maps view slugs to vite contexts.
	ViteContexts *lrucache.Cache[string, ViteContext]
	Logger       logger.Logger

	AuthInfo     api.AuthInfoResponse
	VersionCache version.Cache

	PortProxy *httputil.ReverseProxy
	DevToken  *string
	// ServerHost is the URL that the local dev server should be accessed from. It does not necessarily represent the
	// localhost address relative to the host machine. For example, if the host machine is running in a sandbox, we
	// want to access the local dev server from some.sandbox.url, not localhost:*. This is used throughout the dev
	// server, but primarily used to proxy requests from the local dev server to the Vite server so that views work
	// remotely.
	ServerHost string
	// Non-nil if the server is running in remote/sandbox mode.
	SandboxState *SandboxState

	ServerStatus      status.ServerStatus
	ServerStatusMutex sync.Mutex
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

// New initializes a new state.
func New(devToken *string) (*State, error) {
	onEvict := func(key string, viteContext ViteContext) {
		if err := viteContext.Process.Kill(); err != nil {
			logger.Error(fmt.Sprintf("could not shutdown existing vite process: %v", err))
		}

		if err := viteContext.Closer.Close(); err != nil {
			logger.Error(fmt.Sprintf("unable to cleanup vite process: %v", err))
		}
	}

	viteContextCache, err := lrucache.NewWithEvict(5, onEvict)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vite context cache")
	}

	return &State{
		EnvCache:     NewStore[string, libapi.Env](nil),
		Runs:         NewRunStore(),
		TaskConfigs:  NewStore[string, discover.TaskConfig](nil),
		AppCondition: NewStore[string, AppCondition](nil),
		ViewConfigs:  NewStore[string, discover.ViewConfig](nil),
		Debouncer:    NewDebouncer(),
		ViteContexts: viteContextCache,
		PortProxy:    network.ViewPortProxy(devToken),
		DevToken:     devToken,
		Logger:       logger.NewStdErrLogger(logger.StdErrLoggerOpts{}),
		ServerStatus: status.ServerDiscovering,
		DevConfig:    conf.NewDevConfig(""), // Set dev config to a zero value initially.
	}, nil
}

func (s *State) GetEnv(ctx context.Context, envSlug string) (libapi.Env, error) {
	if env, ok := s.EnvCache.Get(envSlug); ok {
		return env, nil
	}

	env, err := s.RemoteClient.GetEnv(ctx, envSlug)
	if err != nil {
		return libapi.Env{}, errors.Wrap(err, "error getting environment")
	}
	s.EnvCache.Add(envSlug, env)
	return env, nil
}

func (s *State) GetTaskErrors(ctx context.Context, slug string, envSlug string) (AppCondition, error) {
	key := appConditionKey(slug, envSlug)
	if result, ok := s.AppCondition.Get(key); ok {
		return result, nil
	}

	result := AppCondition{
		RefreshedAt: time.Now(),
	}

	taskConfig, ok := s.TaskConfigs.Get(slug)
	if !ok {
		// Not supported locally.
		kind, _, err := taskConfig.Def.GetKindAndOptions()
		if err != nil {
			return AppCondition{}, errors.Wrap(err, "getting task kind")
		}
		result.Errors = append(result.Errors, dev_errors.AppError{
			Level:   dev_errors.LevelError,
			AppName: taskConfig.Def.GetName(),
			AppKind: "task",
			Reason:  fmt.Sprintf("%v task does not support local execution", kind),
		})
	} else {
		mergedResources, err := resources.MergeRemoteResources(ctx, s.RemoteClient, s.DevConfig, pointers.String(envSlug))
		if err != nil {
			return AppCondition{}, errors.Wrap(err, "merging local and remote resources")
		}

		// Check resource attachments.
		var missingResources []string
		resourceAttachments, err := taskConfig.Def.GetResourceAttachments()
		if err != nil {
			return AppCondition{}, errors.Wrap(err, "getting resource attachments")
		}
		for _, ref := range resourceAttachments {
			if _, ok := resources.LookupResource(mergedResources, ref); !ok {
				missingResources = append(missingResources, ref)
			}
		}
		if len(missingResources) > 0 {
			sort.Strings(missingResources)
			for _, resource := range missingResources {
				result.Errors = append(result.Errors, dev_errors.AppError{
					Level:   dev_errors.LevelWarning,
					AppName: taskConfig.Def.GetName(),
					AppKind: "task",
					Reason:  fmt.Sprintf("Attached resource %q not found in dev config file or remotely.", resource),
				})
			}
		}
	}

	s.AddAppCondition(slug, envSlug, result)
	return result, nil
}

func (s *State) AddAppCondition(slug string, envSlug string, appCondition AppCondition) {
	key := appConditionKey(slug, envSlug)
	s.AppCondition.Add(key, appCondition)
}

func appConditionKey(slug, envSlug string) string {
	if envSlug == "" {
		return slug
	}
	return slug + "-" + envSlug
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

func (s *State) SetServerStatus(status status.ServerStatus) {
	s.ServerStatusMutex.Lock()
	defer s.ServerStatusMutex.Unlock()
	s.ServerStatus = status
}
