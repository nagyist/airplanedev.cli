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
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	libBuild "github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/archive"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/dustin/go-humanize"
	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type deployer struct {
	buildCreator build.BuildCreator
	cfg          config
	logger       logger.Logger
	loader       logger.Loader
	archiver     archive.Archiver
	repoGetter   GitRepoGetter

	erroredTaskSlugs  map[string]error
	deployedTaskSlugs []string
	mu                sync.Mutex
}

type DeployerOpts struct {
	BuildCreator build.BuildCreator
	Archiver     archive.Archiver
	RepoGetter   GitRepoGetter
}

func NewDeployer(cfg config, l logger.Logger, opts DeployerOpts) *deployer {
	var bc build.BuildCreator
	if cfg.local {
		bc = build.NewLocalBuildCreator()
	}
	if opts.BuildCreator != nil {
		bc = opts.BuildCreator
	}
	a := archive.NewAPIArchiver(l, cfg.client, &archive.HttpUploader{})
	if opts.Archiver != nil {
		a = opts.Archiver
	}
	var rg GitRepoGetter
	rg = &FileGitRepoGetter{}
	if opts.RepoGetter != nil {
		rg = opts.RepoGetter
	}
	return &deployer{
		buildCreator: bc,
		cfg:          cfg,
		logger:       l,
		loader:       logger.NewLoader(logger.LoaderOpts{HideLoader: logger.EnableDebug}),
		archiver:     a,
		repoGetter:   rg,
	}
}

// DeployTasks deploys all taskConfigs as Airplane tasks.
func (d *deployer) DeployTasks(ctx context.Context, taskConfigs []discover.TaskConfig) error {
	var err error
	if len(d.cfg.changedFiles) > 0 {
		taskConfigs, err = d.filterTaskConfigsByChangedFiles(ctx, taskConfigs)
		if err != nil {
			return err
		}
	}

	if len(taskConfigs) == 0 {
		d.logger.Log("No tasks to deploy")
		return nil
	}

	d.printPreDeploySummary(ctx, taskConfigs)

	var uploadIDs map[string]string
	if !d.cfg.local {
		uploadIDs, err = d.tarAndUploadTasks(ctx, taskConfigs)
		if err != nil {
			return err
		}
	}

	var tasksToDeploy []api.DeployTask
	gitRoots := make(map[string]bool)
	var repo *git.Repository
	for _, tc := range taskConfigs {
		repo, err = d.repoGetter.GetGitRepo(tc.TaskEntrypoint)
		if err != nil {
			d.logger.Debug("failed to get git repo for %s: %v", tc.TaskEntrypoint, err)
		}
		taskToDeploy, err := d.getDeployTask(ctx, tc, uploadIDs[tc.Task.ID], repo)
		if err != nil {
			return err
		}
		tasksToDeploy = append(tasksToDeploy, taskToDeploy)

		// Get the root directory of the git repo with which the task is associated.
		var gitRoot string
		if repo != nil {
			w, err := repo.Worktree()
			if err == nil {
				gitRoot = w.Filesystem.Root()
			}
		}
		gitRoots[gitRoot] = true
	}

	// If tasks in a single deploy come from different git repos, we do not
	// include git information with the deploy.
	mismatchedGitRepos := len(gitRoots) > 1
	if mismatchedGitRepos {
		analytics.ReportMessage("deploy created with multiple git repos")
	}

	var gitMeta api.GitMetadata
	if repo != nil && !mismatchedGitRepos {
		gitMeta, err = GetGitMetadata(repo)
		if err != nil {
			analytics.ReportMessage(fmt.Sprintf("failed to gather git metadata at %s: %v", taskConfigs[0].TaskEntrypoint, err))
		}
		gitMeta.User = conf.GetGitUser()
		// Use the env variable provided repo if it exists.
		getGitRepoResp := conf.GetGitRepo()
		if getGitRepoResp.RepoName != "" {
			gitMeta.RepositoryName = getGitRepoResp.RepoName
		}
		if getGitRepoResp.OwnerName != "" {
			gitMeta.RepositoryOwnerName = getGitRepoResp.OwnerName
		}
	}

	resp, err := d.cfg.client.CreateDeployment(ctx, api.CreateDeploymentRequest{
		Tasks:       tasksToDeploy,
		GitMetadata: gitMeta,
	})
	if err != nil {
		return err
	}

	// TODO log URL to deploy

	return waitForDeploy(ctx, d.loader, d.cfg.client, resp.Deployment.ID)
}

func (d *deployer) getDeployTask(ctx context.Context, tc discover.TaskConfig, uploadID string, repo *git.Repository) (taskToDeploy api.DeployTask, rErr error) {
	client := d.cfg.client
	kind, buildConfig, err := tc.Def.GetKindAndOptions()
	if err != nil {
		return api.DeployTask{}, err
	}
	tp := taskDeployedProps{
		from:       string(tc.From),
		kind:       kind,
		taskID:     tc.Task.ID,
		taskSlug:   tc.Task.Slug,
		taskName:   tc.Task.Name,
		buildLocal: d.cfg.local,
	}
	start := time.Now()
	defer func() {
		analytics.Track(d.cfg.root, "Task Deployed", map[string]interface{}{
			"from":             tp.from,
			"kind":             tp.kind,
			"task_id":          tp.taskID,
			"task_slug":        tp.taskSlug,
			"task_name":        tp.taskName,
			"build_id":         tp.buildID,
			"errored":          rErr != nil,
			"duration_seconds": time.Since(start).Seconds(),
			"env_slug":         d.cfg.envSlug,
		})
	}()

	interpolationMode := tc.Task.InterpolationMode
	if interpolationMode != "jst" {
		if d.cfg.upgradeInterpolation {
			d.logger.Warning(`Your task is being migrated from handlebars to Airplane JS Templates.
More information: https://apn.sh/jst-upgrade`)
			interpolationMode = "jst"
			if err := tc.Def.UpgradeJST(); err != nil {
				return api.DeployTask{}, err
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

	err = ensureConfigVarsExist(ctx, client, tc.Def, d.cfg.envSlug)
	if err != nil {
		return api.DeployTask{}, err
	}

	env, err := tc.Def.GetEnv()
	if err != nil {
		return api.DeployTask{}, err
	}
	if buildConfig != nil {
		buildConfig["shim"] = "true"
		// Normalize entrypoint to `/` regardless of OS.
		// CLI might be run from Windows or not Windows, but remote API is on Linux.
		if ep, ok := buildConfig["entrypoint"].(string); ok {
			buildConfig["entrypoint"] = filepath.ToSlash(ep)
		}
	}

	var image *string
	if ok, err := libBuild.NeedsBuilding(kind); err != nil {
		return api.DeployTask{}, err
	} else if ok && d.cfg.local {
		resp, err := d.buildCreator.CreateBuild(ctx, build.Request{
			Client:  client,
			TaskID:  tc.Task.ID,
			Root:    tc.TaskRoot,
			Def:     tc.Def,
			TaskEnv: env,
			Shim:    true,
		})
		if err != nil {
			return api.DeployTask{}, err
		}
		image = &resp.ImageURL
	}

	utr, err := tc.Def.GetUpdateTaskRequest(ctx, d.cfg.client, &tc.Task)
	if err != nil {
		return api.DeployTask{}, err
	}
	if image != nil {
		utr.Image = image
	}
	utr.InterpolationMode = interpolationMode
	utr.EnvSlug = d.cfg.envSlug

	var filePath string
	if repo != nil {
		filePath, err = GetEntrypointRelativeToGitRoot(repo, tc.TaskEntrypoint)
		if err != nil {
			d.logger.Debug("failed to get entrypoint relative to git root %s: %v", tc.TaskEntrypoint, err)
		}
	}

	return api.DeployTask{
		TaskID:            tc.Task.ID,
		Kind:              kind,
		BuildConfig:       buildConfig,
		UploadID:          uploadID,
		UpdateTaskRequest: utr,
		EnvVars:           env,
		GitFilePath:       filePath,
		InterpolationMode: interpolationMode,
	}, nil
}

// tarAndUploadTasks concurrently tars and uploads tasks that need building.
func (d *deployer) tarAndUploadTasks(ctx context.Context, taskConfigs []discover.TaskConfig) (map[string]string, error) {
	uploadIDs := make(map[string]string)
	g, ctx := errgroup.WithContext(ctx)
	for _, tc := range taskConfigs {
		tc := tc
		kind, _, err := tc.Def.GetKindAndOptions()
		if err != nil {
			return nil, err
		}
		needsBuilding, err := libBuild.NeedsBuilding(kind)
		if err != nil {
			return nil, err
		}
		if !needsBuilding {
			continue
		}

		g.Go(func() error {
			uploadID, err := d.tarAndUpload(ctx, tc.Task.Slug, tc.TaskRoot)
			if err != nil {
				return err
			}
			uploadIDs[tc.Task.ID] = uploadID
			return nil
		})
	}
	groupErr := g.Wait()
	return uploadIDs, groupErr
}

func (d *deployer) tarAndUpload(ctx context.Context, taskSlug, taskRoot string) (string, error) {
	if err := confirmBuildRoot(taskRoot); err != nil {
		return "", err
	}

	deployLog(ctx, api.LogLevelInfo, d.loader, deployLogReq{taskSlug, logger.Gray("Packaging and uploading %s...", taskRoot)})

	uploadID, sizeBytes, err := d.archiver.Archive(ctx, taskRoot)
	if err != nil {
		return "", err
	}
	if sizeBytes > 0 {
		deployLog(ctx, api.LogLevelInfo, d.loader, deployLogReq{taskSlug, logger.Gray("Uploaded %s build archive.",
			humanize.Bytes(uint64(sizeBytes)),
		)})
	}
	return uploadID, nil
}

// filterTaskConfigsByChangedFiles filters out any tasks that don't have changed files.
func (d *deployer) filterTaskConfigsByChangedFiles(ctx context.Context, taskConfigs []discover.TaskConfig) ([]discover.TaskConfig, error) {
	var filteredTaskConfigs []discover.TaskConfig
	for _, tc := range taskConfigs {
		contains, err := containsFile(tc.TaskRoot, d.cfg.changedFiles)
		if err != nil {
			return nil, err
		}
		if contains {
			filteredTaskConfigs = append(filteredTaskConfigs, tc)
		}
	}
	if len(taskConfigs) != len(filteredTaskConfigs) {
		d.logger.Log("Changed files specified. Filtered %d task(s) to %d affected task(s)", len(taskConfigs), len(filteredTaskConfigs))
	}
	return filteredTaskConfigs, nil
}

func (d *deployer) printPreDeploySummary(ctx context.Context, taskConfigs []discover.TaskConfig) {
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
}

func waitForDeploy(ctx context.Context, loader logger.Loader, client api.APIClient, deploymentID string) error {
	loader.Start()
	defer loader.Stop()
	deployLog(ctx, api.LogLevelInfo, loader, deployLogReq{msg: logger.Gray("Waiting for deployer...")})

	t := time.NewTicker(time.Second)

	var prevToken string
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			r, err := client.GetDeploymentLogs(ctx, deploymentID, prevToken)
			if err != nil {
				return errors.Wrap(err, "getting build logs")
			}

			if len(r.Logs) > 0 {
				prevToken = r.PrevPageToken
			}

			api.SortLogs(r.Logs)
			for _, l := range r.Logs {
				text := l.Text
				if strings.HasPrefix(l.Text, "[builder] ") {
					text = logger.Gray(strings.TrimPrefix(text, "[builder] "))
				}

				deployLog(ctx, l.Level, loader, deployLogReq{l.TaskSlug, text})
			}

			d, err := client.GetDeployment(ctx, deploymentID)
			if err != nil {
				return errors.Wrap(err, "getting deployment")
			}

			switch {
			case d.FailedAt != nil:
				deployLog(ctx, api.LogLevelInfo, loader, deployLogReq{msg: logger.Bold(logger.Red("failed: %s", d.FailedReason))})
				return errors.New("Deploy failed")
			case d.SucceededAt != nil:
				deployLog(ctx, api.LogLevelInfo, loader, deployLogReq{msg: logger.Bold(logger.Green("succeeded"))})
				return nil
			case d.CancelledAt != nil:
				deployLog(ctx, api.LogLevelInfo, loader, deployLogReq{msg: logger.Bold(logger.Red("cancelled"))})
				return errors.New("Deploy cancelled")
			}
		}
	}
}

type deployLogReq struct {
	taskSlug string
	msg      string
}

func deployLog(ctx context.Context, level api.LogLevel, loader logger.Loader, req deployLogReq, args ...interface{}) {
	loaderActive := loader.IsActive()
	loader.Stop()
	buildMsg := fmt.Sprintf("[%s %s] ", logger.Yellow("deploy"), req.taskSlug)
	if req.taskSlug == "" {
		buildMsg = fmt.Sprintf("[%s] ", logger.Yellow("deploy"))
	}
	if level == api.LogLevelDebug {
		logger.Log(buildMsg+"["+logger.Blue("debug")+"] "+req.msg, args...)
	} else {
		logger.Log(buildMsg+req.msg, args...)
	}
	if loaderActive {
		loader.Start()
	}
}

func confirmBuildRoot(root string) error {
	if home, err := os.UserHomeDir(); err != nil {
		return errors.Wrap(err, "getting home dir")
	} else if home != root {
		return nil
	}
	logger.Warning("This task's root is your home directory — deploying will attempt to upload the entire directory.")
	logger.Warning("Consider moving your task entrypoint to a subdirectory.")
	if ok, err := utils.Confirm("Are you sure?"); err != nil {
		return err
	} else if !ok {
		return errors.New("aborting build")
	}
	return nil
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
