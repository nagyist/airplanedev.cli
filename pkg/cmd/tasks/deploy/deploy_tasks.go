package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/build"
	"github.com/airplanedev/cli/pkg/cmd/tasks/deploy/discover"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	libBuild "github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type deployer struct {
	buildCreator build.BuildCreator
	cfg          config
	logger       logger.Logger

	erroredTaskSlugs  map[string]error
	deployedTaskSlugs []string
	mu                sync.Mutex
}

type DeployerOpts struct {
	BuildCreator build.BuildCreator
}

func NewDeployer(cfg config, logger logger.Logger, opts DeployerOpts) *deployer {
	var bc build.BuildCreator
	if cfg.local {
		bc = build.NewLocalBuildCreator()
	} else {
		bc = build.NewRemoteBuildCreator()
	}
	if opts.BuildCreator != nil {
		bc = opts.BuildCreator
	}
	return &deployer{
		buildCreator:     bc,
		erroredTaskSlugs: make(map[string]error),
		cfg:              cfg,
		logger:           logger,
	}
}

// DeployTasks deploys all taskConfigs as Airplane tasks.
// It concurrently builds (if needed) and updates tasks.
func (d *deployer) DeployTasks(ctx context.Context, taskConfigs []discover.TaskConfig) error {
	if len(d.cfg.changedFiles) > 0 {
		// Filter out any tasks that don't have changed files.
		var filteredTaskConfigs []discover.TaskConfig
		for _, tc := range taskConfigs {
			contains, err := containsFile(tc.TaskRoot, d.cfg.changedFiles)
			if err != nil {
				return err
			}
			if contains {
				filteredTaskConfigs = append(filteredTaskConfigs, tc)
			}
		}
		if len(taskConfigs) != len(filteredTaskConfigs) {
			d.logger.Log("Changed files specified. Filtered %d task(s) to %d affected task(s)", len(taskConfigs), len(filteredTaskConfigs))
		}
		taskConfigs = filteredTaskConfigs
	}

	if len(taskConfigs) == 0 {
		d.logger.Log("No tasks to deploy")
		return nil
	}

	// Print out a summary before deploying.
	noun := "task"
	if len(taskConfigs) > 1 {
		noun = fmt.Sprintf("%ss", noun)
	}
	d.logger.Log("Deploying %v %v:\n", len(taskConfigs), noun)
	for _, tc := range taskConfigs {
		d.logger.Log(logger.Bold(tc.Task.Slug))
		d.logger.Log("Type: %s", tc.Task.Kind)
		d.logger.Log("Root directory: %s", relpath(tc.TaskRoot))
		if tc.WorkingDirectory != tc.TaskRoot {
			d.logger.Log("Working directory: %s", relpath(tc.WorkingDirectory))
		}
		d.logger.Log("URL: %s", d.cfg.client.TaskURL(tc.Task.Slug))
		d.logger.Log("")
	}

	g := new(errgroup.Group)
	// Concurrently deploy the tasks.
	for _, tc := range taskConfigs {
		tc := tc
		g.Go(func() error {
			err := d.deployTask(ctx, d.cfg, tc)
			d.mu.Lock()
			defer d.mu.Unlock()
			if err != nil {
				if !errors.As(err, &runtime.ErrNotLinked{}) {
					d.erroredTaskSlugs[tc.Task.Slug] = err
					return err
				}
			} else {
				d.deployedTaskSlugs = append(d.deployedTaskSlugs, tc.Task.Slug)
			}
			return nil
		})
	}

	groupErr := g.Wait()

	// All of the deploys have finished.
	for taskSlug, err := range d.erroredTaskSlugs {
		d.logger.Log("\n" + logger.Bold(taskSlug))
		d.logger.Log("Status: " + logger.Bold(logger.Red("failed")))
		d.logger.Error(err.Error())
	}
	for _, slug := range d.deployedTaskSlugs {
		d.logger.Log("\n" + logger.Bold(slug))
		d.logger.Log("Status: %s", logger.Bold(logger.Green("succeeded")))
		d.logger.Log("Execute the task: %s", d.cfg.client.TaskURL(slug))
	}

	return groupErr
}

func (d *deployer) deployTask(ctx context.Context, cfg config, tc discover.TaskConfig) (rErr error) {
	client := cfg.client
	tp := taskDeployedProps{
		from:       string(tc.From),
		kind:       tc.Kind,
		taskID:     tc.Task.ID,
		taskSlug:   tc.Task.Slug,
		taskName:   tc.Task.Name,
		buildLocal: cfg.local,
	}
	start := time.Now()
	defer func() {
		analytics.Track(cfg.root, "Task Deployed", map[string]interface{}{
			"from":             tp.from,
			"kind":             tp.kind,
			"task_id":          tp.taskID,
			"task_slug":        tp.taskSlug,
			"task_name":        tp.taskName,
			"build_id":         tp.buildID,
			"errored":          rErr != nil,
			"duration_seconds": time.Since(start).Seconds(),
		})
	}()

	interpolationMode := tc.Task.InterpolationMode
	if interpolationMode != "jst" {
		if cfg.upgradeInterpolation {
			d.logger.Warning(`Your task is being migrated from handlebars to Airplane JS Templates.
More information: https://apn.sh/jst-upgrade`)
			interpolationMode = "jst"
			if err := tc.Def.UpgradeJST(); err != nil {
				return err
			}
		} else {
			d.logger.Warning(`Tasks are migrating from handlebars to Airplane JS Templates! Your task has not
been automatically upgraded because of potential backwards-compatibility issues
(e.g. uploads will be passed to your task as an object with a url field instead
of just the url string).

To upgrade, update your task to support the new format and re-deploy with --jst.
More information: https://apn.sh/jst-upgrade`)
		}
	}

	err := ensureConfigVarsExist(ctx, client, tc.Def)
	if err != nil {
		return err
	}

	kind, _, err := tc.Def.GetKindAndOptions()
	if err != nil {
		return err
	}
	var image *string
	if ok, err := libBuild.NeedsBuilding(kind); err != nil {
		return err
	} else if ok {
		gitMeta, err := getGitMetadata(tc.TaskFilePath)
		if err != nil {
			d.logger.Debug("failed to gather git metadata: %v", err)
			analytics.ReportError(errors.Wrap(err, "failed to gather git metadata"))
		}
		gitMeta.User = conf.GetGitUser()
		gitMeta.Repository = conf.GetGitRepo()

		env, err := tc.Def.GetEnv()
		if err != nil {
			return err
		}
		resp, err := d.buildCreator.CreateBuild(ctx, build.Request{
			Client:  client,
			TaskID:  tc.Task.ID,
			Root:    tc.TaskRoot,
			Def:     tc.Def,
			TaskEnv: env,
			Shim:    true,
			GitMeta: gitMeta,
		})
		if err != nil {
			return err
		}
		tp.buildID = resp.BuildID
		image = &resp.ImageURL
	}

	utr, err := tc.Def.GetUpdateTaskRequest(ctx, client, &tc.Task)
	if err != nil {
		return err
	}

	if image != nil {
		utr.Image = image
	}
	utr.BuildID = pointers.String(tp.buildID)
	utr.InterpolationMode = interpolationMode

	_, err = client.UpdateTask(ctx, utr)
	return err
}

func getGitMetadata(taskFilePath string) (api.BuildGitMeta, error) {
	meta := api.BuildGitMeta{}

	repo, err := git.PlainOpenWithOptions(filepath.Dir(taskFilePath), &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return meta, nil
		}
		return meta, err
	}

	w, err := repo.Worktree()
	if err != nil {
		return meta, err
	}
	pathRelativeToGitRoot, err := filepath.Rel(w.Filesystem.Root(), taskFilePath)
	if err != nil {
		return meta, err
	}
	meta.FilePath = pathRelativeToGitRoot

	status, err := w.Status()
	if err != nil {
		return meta, err
	}
	meta.IsDirty = !status.IsClean()

	h, err := repo.Head()
	if err != nil {
		return meta, err
	}

	commit, err := repo.CommitObject(h.Hash())
	if err != nil {
		return meta, err
	}
	meta.CommitHash = commit.Hash.String()
	meta.CommitMessage = commit.Message
	if meta.User != "" {
		meta.User = commit.Author.Name
	}

	ref := h.Name().String()
	if h.Name().IsBranch() {
		ref = strings.TrimPrefix(ref, "refs/heads/")
	}
	meta.Ref = ref

	return meta, nil
}

// containsFile returns true if the directory contains at least one of the files.
func containsFile(dir string, filePaths []string) (bool, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false, errors.Wrapf(err, "calculating absolute path of directory %s", dir)
	}
	for _, cf := range filePaths {
		absCF, err := filepath.Abs(cf)
		if err != nil {
			return false, errors.Wrapf(err, "calculating absolute path of file %s", cf)
		}
		changedFileDir := filepath.Dir(absCF)
		if strings.HasPrefix(changedFileDir, absDir) {
			return true, nil
		}
	}
	return false, nil
}

// Relpath returns the relative using root and the cwd.
func relpath(root string) string {
	if path, err := os.Getwd(); err == nil {
		if rp, err := filepath.Rel(path, root); err == nil {
			if len(rp) == 0 || rp == "." {
				// "." can be missed easily, change it to ./
				return "./"
			}
			return "./" + rp
		}
	}
	return root
}
