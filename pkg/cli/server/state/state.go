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

	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	libhttp "github.com/airplanedev/cli/pkg/cli/apiclient/http"
	"github.com/airplanedev/cli/pkg/cli/dev"
	"github.com/airplanedev/cli/pkg/cli/devconf"
	"github.com/airplanedev/cli/pkg/cli/flags/flagsiface"
	libparams "github.com/airplanedev/cli/pkg/cli/parameters"
	resources "github.com/airplanedev/cli/pkg/cli/resources/cliresources"
	"github.com/airplanedev/cli/pkg/cli/server/dev_errors"
	"github.com/airplanedev/cli/pkg/cli/server/network"
	"github.com/airplanedev/cli/pkg/cli/server/status"
	"github.com/airplanedev/cli/pkg/deploy/bundlediscover"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	lrucache "github.com/hashicorp/golang-lru/v2"
	"github.com/pkg/errors"
)

type ViteContext struct {
	Closer    io.Closer
	ServerURL string
	Process   *os.Process
}

type TaskState struct {
	discover.TaskConfig

	UpdatedAt time.Time
}

type ViewState struct {
	discover.ViewConfig

	UpdatedAt time.Time
}

type State struct {
	Flagger              flagsiface.Flagger
	LocalClient          *api.Client
	RemoteClient         api.APIClient
	InitialRemoteEnvSlug *string
	StudioURL            url.URL

	EnvCache Store[string, libapi.Env]

	// Directory from which tasks and views were discovered
	Dir      string
	Executor dev.Executor

	Runs       *runsStore
	LocalTasks Store[string, TaskState]
	LocalViews Store[string, ViewState]

	// TaskConditions holds info about tasks such as errors to display.
	TaskConditions Store[string, EntityCondition]

	Discoverer       *discover.Discoverer
	BundleDiscoverer *bundlediscover.Discoverer

	// Debouncers maps paths to debouncing functions.
	Debouncers Store[string, func()]

	DevConfig *devconf.DevConfig
	// ViteContexts is an in-memory cache that maps view slugs to vite contexts.
	ViteContexts *lrucache.Cache[string, ViteContext]
	Logger       logger.Logger

	AuthInfo api.AuthInfoResponse

	// We cache the CLI's version since github rate limits checks
	// When we add hot reload and it's long running, we should expire/periodically refresh this.
	version   *VersionMetadata
	versionMu sync.Mutex

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

type EntityCondition struct {
	Errors []dev_errors.EntityError
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
		EnvCache:       NewStore[string, libapi.Env](nil),
		Runs:           NewRunStore(),
		LocalTasks:     NewStore[string, TaskState](nil),
		TaskConditions: NewStore[string, EntityCondition](nil),
		LocalViews:     NewStore[string, ViewState](nil),
		Debouncers:     NewStore[string, func()](nil),
		ViteContexts:   viteContextCache,
		PortProxy:      network.ViewPortProxy(devToken),
		DevToken:       devToken,
		Logger:         logger.NewStdErrLogger(logger.StdErrLoggerOpts{}),
		ServerStatus:   status.ServerDiscovering,
		DevConfig:      devconf.NewDevConfig(""), // Set dev config to a zero value initially.
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

func (s *State) GetTaskErrors(ctx context.Context, slug string, envSlug string) (EntityCondition, error) {
	key := appConditionKey(slug, envSlug)
	if result, ok := s.TaskConditions.Get(key); ok {
		return result, nil
	}

	result := EntityCondition{}

	taskConfig, ok := s.LocalTasks.Get(slug)
	if !ok {
		// Not supported locally.
		kind, _, err := taskConfig.Def.GetKindAndOptions()
		if err != nil {
			return EntityCondition{}, errors.Wrap(err, "getting task kind")
		}
		result.Errors = append(result.Errors, dev_errors.EntityError{
			Level:  dev_errors.LevelError,
			Name:   taskConfig.Def.GetName(),
			Kind:   "task",
			Reason: fmt.Sprintf("%v task does not support local execution", kind),
		})
	} else {
		// Is local execution supported?
		kind, _, err := taskConfig.Def.GetKindAndOptions()
		if err != nil {
			return EntityCondition{}, errors.Wrap(err, "getting task kind")
		}
		supported := supportsLocalExecution(taskConfig.Def.GetName(), taskConfig.TaskEntrypoint, kind)
		if !supported {
			result.Errors = append(result.Errors, dev_errors.EntityError{
				Level:  dev_errors.LevelError,
				Name:   taskConfig.Def.GetName(),
				Kind:   "task",
				Reason: fmt.Sprintf("%v tasks cannot be executed locally.", kind.UserFriendlyTaskKind()),
			})
		}

		mergedResources, err := resources.MergeRemoteResources(ctx, s.RemoteClient, s.DevConfig, pointers.String(envSlug))
		if err != nil {
			return EntityCondition{}, errors.Wrap(err, "merging local and remote resources")
		}

		// Check resource attachments.
		var missingResources []string
		resourceAttachments, err := taskConfig.Def.GetResourceAttachments()
		if err != nil {
			return EntityCondition{}, errors.Wrap(err, "getting resource attachments")
		}
		for _, ref := range resourceAttachments {
			if _, ok := resources.LookupResource(mergedResources, ref); !ok {
				missingResources = append(missingResources, ref)
			}
		}
		if len(missingResources) > 0 {
			sort.Strings(missingResources)
			for _, resource := range missingResources {
				reason := fmt.Sprintf("Attached resource %q not found in dev config file", resource)
				if envSlug != "" {
					reason += fmt.Sprintf(" or remotely in env %q", envSlug)
				}
				reason += "."
				result.Errors = append(result.Errors, dev_errors.EntityError{
					Level:  dev_errors.LevelWarning,
					Name:   taskConfig.Def.GetName(),
					Kind:   "task",
					Reason: reason,
				})
			}
		}
	}

	s.AddTaskCondition(slug, envSlug, result)
	return result, nil
}

func (s *State) AddTaskCondition(slug string, envSlug string, appCondition EntityCondition) {
	key := appConditionKey(slug, envSlug)
	s.TaskConditions.Add(key, appCondition)
}

func appConditionKey(slug, envSlug string) string {
	if envSlug == "" {
		return slug
	}
	return slug + "-" + envSlug
}

func sanitizeInputs(run *dev.LocalRun) error {
	parameters := libapi.Parameters{}
	if run.Parameters != nil {
		parameters = *run.Parameters
	}
	sanitized, err := libparams.SanitizeParamValues(run.ParamValues, parameters)
	if err != nil {
		return errors.Wrap(err, "sanitizing param values")
	}
	run.ParamValues = sanitized
	return nil
}

func (s *State) AddRun(taskSlug string, runID string, run dev.LocalRun) {
	s.Runs.Add(taskSlug, runID, run)
}

func (s *State) GetRun(ctx context.Context, runID string) (dev.LocalRun, error) {
	run, err := s.GetRunInternal(ctx, runID)
	if err != nil {
		return dev.LocalRun{}, err
	}
	if s.Flagger != nil && s.Flagger.Bool(ctx, s.Logger, flagsiface.SanitizeInputs) {
		if err := sanitizeInputs(&run); err != nil {
			return dev.LocalRun{}, err
		}
	}
	return run, nil
}

func (s *State) GetRunInternal(ctx context.Context, runID string) (dev.LocalRun, error) {
	run, ok := s.Runs.Get(runID)
	if !ok {
		return dev.LocalRun{}, libhttp.NewErrNotFound("Run with id %q not found", runID)
	}
	return run, nil
}

func (s *State) GetRunDescendants(ctx context.Context, runID string) ([]dev.LocalRun, error) {
	descendants := s.Runs.GetDescendants(runID)
	if s.Flagger != nil && s.Flagger.Bool(ctx, s.Logger, flagsiface.SanitizeInputs) {
		for i := range descendants {
			if err := sanitizeInputs(&descendants[i]); err != nil {
				return nil, err
			}
		}
	}
	return descendants, nil
}

func (s *State) UpdateRun(runID string, f func(run *dev.LocalRun) error) (dev.LocalRun, error) {
	return s.Runs.Update(runID, f)
}

func (s *State) GetRunHistory(ctx context.Context, taskID string) ([]dev.LocalRun, error) {
	history := s.Runs.GetRunHistory(taskID)
	if s.Flagger != nil && s.Flagger.Bool(ctx, s.Logger, flagsiface.SanitizeInputs) {
		for i := range history {
			if err := sanitizeInputs(&history[i]); err != nil {
				return nil, err
			}
		}
	}
	return history, nil
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

func (s *State) SetServerStatus(status status.ServerStatus) {
	s.ServerStatusMutex.Lock()
	defer s.ServerStatusMutex.Unlock()
	s.ServerStatus = status
}