package state

import (
	"context"
	"os"
	"strings"
	"time"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/cli/server/status"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	"github.com/airplanedev/cli/pkg/runtime"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

type ReloadPathOpts struct {
	Path    string
	OldPath string
}

func (s *State) ReloadPath(ctx context.Context, opts ReloadPathOpts) error {
	fileInfo, err := os.Stat(opts.Path)
	if err != nil {
		return errors.Wrapf(err, "describing %s", opts.Path)
	}
	shouldRefreshDir := fileInfo.IsDir()

	reload := func() {
		if opts.Path == s.DevConfig.Path {
			if err := s.DevConfig.Update(); err != nil {
				logger.Error("Loading dev config file: %s", err.Error())
			}
		}

		pathsToDiscover := []string{opts.Path}

		for _, tC := range s.LocalTasks.Items() {
			var shouldRefreshTask bool
			// Refresh any tasks that have resource attachments
			if opts.Path == s.DevConfig.Path {
				ra, err := tC.Def.GetResourceAttachments()
				if err != nil {
					logger.Debug("Error getting resource attachments for task %s: %v", tC.Def.GetName(), err)
					continue
				}

				if len(ra) > 0 {
					shouldRefreshTask = true
				}
			}

			// Refresh any tasks that have the modified entrypoint.
			shouldRefreshTask = shouldRefreshTask || tC.TaskEntrypoint == opts.Path
			if shouldRefreshTask {
				pathsToDiscover = append(pathsToDiscover, tC.Def.GetDefnFilePath())
			}
		}

		// Refresh any views that have the modified entrypoint.
		for _, vC := range s.LocalViews.Items() {
			if vC.Def.Entrypoint == opts.Path {
				pathsToDiscover = append(pathsToDiscover, vC.Def.DefnFilePath)
			}
		}

		slices.Sort(pathsToDiscover)
		pathsToDiscover = utils.UniqueStrings(pathsToDiscover)

		discoveredAt := time.Now()
		taskConfigs, viewConfigs, err := s.DiscoverTasksAndViews(ctx, pathsToDiscover...)
		if err != nil {
			logger.Error(err.Error())
		}

		// Delete any tasks or views defined in the old path
		if opts.OldPath != "" {
			for _, tC := range s.LocalTasks.Items() {
				if tC.Def.GetDefnFilePath() == opts.OldPath {
					s.LocalTasks.Delete(tC.Def.Slug)
				}
			}
			for _, vC := range s.LocalViews.Items() {
				if vC.Def.DefnFilePath == opts.OldPath {
					s.LocalViews.Delete(vC.Def.Slug)
				}
			}
		}

		err = s.RegisterTasksAndViews(ctx, DiscoverOpts{
			Tasks:        taskConfigs,
			Views:        viewConfigs,
			OverwriteAll: shouldRefreshDir,
			DiscoveredAt: discoveredAt,
		})
		LogNewApps(taskConfigs, viewConfigs)
		if err != nil {
			logger.Error(err.Error())
		}
	}

	dfn, ok := s.Debouncers.Get(opts.Path)
	if !ok {
		dfn = utils.Debounce(time.Second, reload)
		s.Debouncers.Add(opts.Path, dfn)
	}
	// kick off a debounced version of the reload
	// debounce is non-blocking and will execute reload() in a separate goroutine
	dfn()

	return nil
}

func (s *State) DiscoverTasksAndViews(ctx context.Context, paths ...string) ([]discover.TaskConfig, []discover.ViewConfig, error) {
	if s.Discoverer == nil {
		return []discover.TaskConfig{}, []discover.ViewConfig{}, errors.New("discoverer not initialized")
	}
	taskConfigs, viewConfigs, err := s.Discoverer.Discover(ctx, paths...)
	if err != nil {
		return []discover.TaskConfig{}, []discover.ViewConfig{}, errors.Wrap(err, "discovering tasks and views")
	}

	return taskConfigs, viewConfigs, err
}

type DiscoverOpts struct {
	Tasks []discover.TaskConfig
	Views []discover.ViewConfig
	// OverwriteAll will clear out existing tasks and views and replace them with the new ones
	OverwriteAll bool
	// DiscoveredAt is the time when discovery was started. If zero, defaults to the current time.
	DiscoveredAt time.Time
}

// RegisterTasksAndViews generates a mapping of slug to task and view configs and stores the mappings in the server
// state. Task registration must occur after the local dev server has started because the task discoverer hits the
// /v0/tasks/getMetadata endpoint.
func (s *State) RegisterTasksAndViews(ctx context.Context, opts DiscoverOpts) error {
	if opts.DiscoveredAt.IsZero() {
		opts.DiscoveredAt = time.Now()
	}

	// Always invalidate the EntityCondition cache.
	s.TaskConditions.ReplaceItems(map[string]EntityCondition{})

	taskConfigs := map[string]TaskState{}
	for _, cfg := range opts.Tasks {
		taskConfigs[cfg.Def.GetSlug()] = TaskState{
			TaskConfig: cfg,
			UpdatedAt:  time.Now(),
		}
	}
	viewConfigs := map[string]ViewState{}
	for _, cfg := range opts.Views {
		viewConfigs[cfg.Def.Slug] = ViewState{
			ViewConfig: cfg,
			UpdatedAt:  opts.DiscoveredAt,
		}
	}
	if opts.OverwriteAll {
		s.LocalTasks.ReplaceItems(taskConfigs)
		s.LocalViews.ReplaceItems(viewConfigs)
	} else {
		s.LocalTasks.AddMany(taskConfigs)
		s.LocalViews.AddMany(viewConfigs)
	}

	s.SetServerStatus(status.ServerReady)

	return nil
}

func supportsLocalExecution(name string, entrypoint string, kind buildtypes.TaskKind) bool {
	r, err := runtime.Lookup(entrypoint, kind)
	if err != nil {
		logger.Debug("%s does not support local execution: %v", name, err)
		return false
	}
	// Check if task kind can be locally developed.
	return r.SupportsLocalExecution()
}

// LogNewApps prints the names of the tasks/views that were discovered
func LogNewApps(tasks []discover.TaskConfig, views []discover.ViewConfig) {
	taskNames := make([]string, len(tasks))
	for i, task := range tasks {
		taskNames[i] = task.Def.GetName()
	}
	taskNoun := "tasks"
	if len(tasks) == 1 {
		taskNoun = "task"
	}
	time := time.Now().Format(logger.TimeFormatNoDate)
	if len(tasks) > 0 {
		logger.Log("%v Loaded %s: %v", logger.Yellow(time), taskNoun, strings.Join(taskNames, ", "))
	}
	viewNoun := "views"
	if len(views) == 1 {
		viewNoun = "view"
	}
	viewNames := make([]string, len(views))
	for i, view := range views {
		viewNames[i] = view.Def.Name
	}
	if len(views) > 0 {
		logger.Log("%v Loaded %s: %v", logger.Yellow(time), viewNoun, strings.Join(viewNames, ", "))
	}
}
