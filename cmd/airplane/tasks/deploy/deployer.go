package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	bundledeploy "github.com/airplanedev/cli/cmd/airplane/root/deploy"
	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/build"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	libBuild "github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/archive"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/dustin/go-humanize"
	"github.com/go-git/go-git/v5"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type deployer struct {
	buildCreator build.BuildCreator
	cfg          config
	logger       logger.LoggerWithLoader
	archiver     archive.Archiver
	repoGetter   bundledeploy.GitRepoGetter
}

type DeployerOpts struct {
	BuildCreator build.BuildCreator
	Archiver     archive.Archiver
	RepoGetter   bundledeploy.GitRepoGetter
}

func NewDeployer(cfg config, l logger.LoggerWithLoader, opts DeployerOpts) *deployer {
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
	var rg bundledeploy.GitRepoGetter
	rg = &bundledeploy.FileGitRepoGetter{}
	if opts.RepoGetter != nil {
		rg = opts.RepoGetter
	}
	return &deployer{
		buildCreator: bc,
		cfg:          cfg,
		logger:       l,
		archiver:     a,
		repoGetter:   rg,
	}
}

// Deploy deploys all configs.
func (d *deployer) Deploy(ctx context.Context, taskConfigs []discover.TaskConfig, viewConfigs []discover.ViewConfig, createdTasks map[string]bool) error {
	var err error
	if len(d.cfg.changedFiles) > 0 {
		taskConfigs, err = d.filterTaskConfigsByChangedFiles(ctx, taskConfigs)
		if err != nil {
			return err
		}
		// TODO implement for views
	}

	if len(taskConfigs) == 0 && len(viewConfigs) == 0 {
		d.logger.Log("Nothing to deploy")
		return nil
	}

	if err := d.printPreDeploySummary(ctx, taskConfigs, viewConfigs, createdTasks); err != nil {
		if err == skippedDeployErr {
			return nil
		}
		return err
	}

	var uploadIDs map[string]string
	if !d.cfg.local {
		uploadIDs, err = d.tarAndUploadBatch(ctx, taskConfigs, viewConfigs)
		if err != nil {
			return err
		}
		d.logger.Debug("Code upload complete")
	}

	var tasksToDeploy []api.DeployTask
	gitRoots := make(map[string]bool)
	var repo *git.Repository
	for _, tc := range taskConfigs {
		repo, err = d.repoGetter.GetGitRepo(tc.TaskEntrypoint)
		if err != nil {
			d.logger.Debug("failed to get git repo for %s: %v", tc.TaskEntrypoint, err)
		} else {
			d.logger.Debug("discovered git repo for %s", tc.TaskEntrypoint)
		}
		taskToDeploy, err := d.getDeployTask(ctx, tc, uploadIDs[tc.TaskID], repo)
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
	var viewsToDeploy []api.DeployView
	for _, vc := range viewConfigs {
		repo, err = d.repoGetter.GetGitRepo(vc.Root)
		if err != nil {
			d.logger.Debug("failed to get git repo for %s: %v", vc.Root, err)
		} else {
			d.logger.Debug("discovered git repo for %s", vc.Root)
		}

		var filePath string
		if repo != nil {
			filePath, err = bundledeploy.GetEntrypointRelativeToGitRoot(repo, vc.Root)
			if err != nil {
				d.logger.Debug("failed to get entrypoint relative to git root %s: %v", vc.Root, err)
			}
		}

		relEntrypoint, err := filepath.Rel(vc.Root, vc.Def.Entrypoint)
		if err != nil {
			return errors.Wrap(err, "relativizing entrypoint")
		}

		// We set a default API host here so that the unit test doesn't crash, but
		// we try to override it if possible
		apiHost := api.Host
		if d.cfg.root != nil && d.cfg.root.Client != nil {
			apiHost = d.cfg.root.Client.Host
		}

		viewToDeploy := api.DeployView{
			ID:          vc.ID,
			UploadID:    uploadIDs[vc.ID],
			GitFilePath: filePath,
			UpdateViewRequest: libapi.UpdateViewRequest{
				Slug:        vc.Def.Slug,
				Name:        vc.Def.Name,
				Description: vc.Def.Description,
				EnvVars:     vc.Def.EnvVars,
			},
			BuildConfig: map[string]interface{}{
				"entrypoint": relEntrypoint,
				"apiHost":    apiHost,
			},
		}

		viewsToDeploy = append(viewsToDeploy, viewToDeploy)

		// Get the root directory of the git repo with which the view is associated.
		var gitRoot string
		if repo != nil {
			w, err := repo.Worktree()
			if err == nil {
				gitRoot = w.Filesystem.Root()
			}
		}
		gitRoots[gitRoot] = true
	}

	// If entities in a single deploy come from different git repos, we do not
	// include git information with the deploy.
	mismatchedGitRepos := len(gitRoots) > 1
	if mismatchedGitRepos {
		analytics.ReportMessage("deploy created with multiple git repos")
	}

	var gitMeta api.GitMetadata
	if repo != nil && !mismatchedGitRepos {
		gitMeta, err = bundledeploy.GetGitMetadata(repo)
		if err != nil {
			analytics.ReportMessage(fmt.Sprintf("failed to gather git metadata: %v", err))
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
		d.logger.Debug("Gathered git metadata for %s", gitMeta.RepositoryName)
	}

	resp, err := d.cfg.client.CreateDeployment(ctx, api.CreateDeploymentRequest{
		Tasks:       tasksToDeploy,
		Views:       viewsToDeploy,
		GitMetadata: gitMeta,
		EnvSlug:     d.cfg.envSlug,
	})
	if err != nil {
		return err
	}

	d.deployLog(ctx, api.LogLevelInfo, deployLogReq{msg: logger.Gray("Creating deployment...")})
	d.logger.Log(logger.Purple(fmt.Sprintf("\nView deployment: %s\n", d.cfg.client.DeploymentURL(resp.Deployment.ID, d.cfg.envSlug))))

	err = d.waitForDeploy(ctx, d.cfg.client, resp.Deployment.ID)
	if errors.Is(err, context.Canceled) {
		// Since `ctx` is cancelled, use a fresh context to cancel the deployment.
		//nolint: contextcheck
		cerr := d.cfg.client.CancelDeployment(context.Background(), api.CancelDeploymentRequest{ID: resp.Deployment.ID})
		if cerr != nil {
			d.logger.Warning("Failed to cancel deployment: %v", cerr)
		} else {
			d.logger.Log("Cancelled deployment")
		}
	}

	return err
}

func (d *deployer) getDeployTask(ctx context.Context, tc discover.TaskConfig, uploadID string, repo *git.Repository) (taskToDeploy api.DeployTask, rErr error) {
	client := d.cfg.client
	kind, _, err := tc.Def.GetKindAndOptions()
	if err != nil {
		return api.DeployTask{}, err
	}
	buildConfig, err := tc.Def.GetBuildConfig()
	if err != nil {
		return api.DeployTask{}, err
	}
	tp := taskDeployedProps{
		source:     string(tc.Source),
		kind:       kind,
		taskID:     tc.TaskID,
		taskSlug:   tc.Def.GetSlug(),
		taskName:   tc.Def.GetName(),
		buildLocal: d.cfg.local,
	}
	start := time.Now()
	defer func() {
		analytics.Track(d.cfg.root.Client, "Task Deployed", map[string]interface{}{
			"source":           tp.source,
			"kind":             tp.kind,
			"task_id":          tp.taskID,
			"task_slug":        tp.taskSlug,
			"task_name":        tp.taskName,
			"build_id":         tp.buildID,
			"errored":          rErr != nil,
			"duration_seconds": time.Since(start).Seconds(),
			"env_slug":         d.cfg.envSlug,
		}, analytics.TrackOpts{
			SkipSlack: true,
		})
	}()

	// Look up the task that we are deploying to get its interpolation mode. A few legacy tasks still
	// use handlebars interpolation.
	interpolationMode := "jst"
	if task, err := client.GetTask(ctx, libapi.GetTaskRequest{
		Slug: tp.taskSlug,
		// This task won't exist if deploying into a new environment. Therefore, we look up the
		// interpolation mode from the default environment. All handlebars tasks will have been
		// deployed into the default environment (unless the default environment was changed,
		// but we choose not to handle that).
		EnvSlug: "",
	}); err != nil {
		// If the task does not exist in the default environment, this task was created after we
		// launched environments which is well after we deprecated handlebars templating. We can
		// assume the interpolation mode is JSTs.
		if _, ok := err.(*libapi.TaskMissingError); !ok {
			return api.DeployTask{}, errors.Wrap(err, "unable to look up interpolation mode")
		}
	} else {
		interpolationMode = task.InterpolationMode
	}

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

			if d.cfg.envSlug != "" {
				return api.DeployTask{}, errors.New("Tasks using handlebars do not support --env")
			}
		}
	}

	err = ensureConfigVarsExist(ctx, client, d.logger, tc.Def, d.cfg.envSlug)
	if err != nil {
		return api.DeployTask{}, err
	}
	d.logger.Debug("Ensured all config vars exist")

	env, err := tc.Def.GetEnv()
	if err != nil {
		return api.DeployTask{}, err
	}

	var image *string
	if ok, err := libBuild.NeedsBuilding(kind); err != nil {
		return api.DeployTask{}, err
	} else if ok {
		buildConfig["shim"] = "true"
		// Normalize entrypoint to `/` regardless of OS.
		// CLI might be run from Windows or not Windows, but remote API is on Linux.
		if ep, ok := buildConfig["entrypoint"].(string); ok {
			buildConfig["entrypoint"] = filepath.ToSlash(ep)
		}

		if d.cfg.local {
			resp, err := d.buildCreator.CreateBuild(ctx, build.Request{
				Client:  client,
				TaskID:  tc.TaskID,
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
	}

	utr, err := tc.Def.GetUpdateTaskRequest(ctx, d.cfg.client, false)
	if err != nil {
		return api.DeployTask{}, err
	}
	if image != nil {
		utr.Image = image
	}
	utr.InterpolationMode = &interpolationMode
	utr.EnvSlug = d.cfg.envSlug

	var filePath string
	if repo != nil {
		filePath, err = bundledeploy.GetEntrypointRelativeToGitRoot(repo, tc.TaskEntrypoint)
		if err != nil {
			d.logger.Debug("failed to get entrypoint relative to git root %s: %v", tc.TaskEntrypoint, err)
		}
	}

	return api.DeployTask{
		TaskID:            tc.TaskID,
		Kind:              kind,
		BuildConfig:       buildConfig,
		UploadID:          uploadID,
		UpdateTaskRequest: utr,
		EnvVars:           env,
		GitFilePath:       filePath,
		Schedules:         tc.Def.GetSchedules(),
	}, nil
}

// tarAndUploadBatch concurrently tars and uploads configs that need building.
func (d *deployer) tarAndUploadBatch(ctx context.Context, taskConfigs []discover.TaskConfig, viewConfigs []discover.ViewConfig) (map[string]string, error) {
	var uploadIDs sync.Map
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
			uploadID, err := d.tarAndUpload(ctx, tc.Def.GetSlug(), tc.TaskRoot)
			if err != nil {
				return err
			}
			_, ok := uploadIDs.Load(tc.TaskID)
			if !ok {
				uploadIDs.Store(tc.TaskID, uploadID)
			}

			return nil
		})
	}

	for _, ac := range viewConfigs {
		ac := ac
		g.Go(func() error {
			uploadID, err := d.tarAndUpload(ctx, ac.Def.Slug, ac.Root)
			if err != nil {
				return err
			}
			_, ok := uploadIDs.Load(ac.ID)
			if !ok {
				uploadIDs.Store(ac.ID, uploadID)
			}
			return nil
		})
	}

	groupErr := g.Wait()

	uploadIDsMap := make(map[string]string)
	uploadIDs.Range(func(key, value interface{}) bool {
		uploadIDsMap[key.(string)] = value.(string)
		return true
	})
	return uploadIDsMap, groupErr
}

func (d *deployer) tarAndUpload(ctx context.Context, slug, root string) (string, error) {
	if err := d.confirmBuildRoot(root); err != nil {
		return "", err
	}

	d.deployLog(ctx, api.LogLevelInfo, deployLogReq{slug, logger.Gray("Packaging and uploading %s...", root)})

	uploadID, sizeBytes, err := d.archiver.Archive(ctx, root)
	if err != nil {
		return "", err
	}
	if sizeBytes > 0 {
		d.deployLog(ctx, api.LogLevelInfo, deployLogReq{slug, logger.Gray("Uploaded %s build archive.",
			humanize.Bytes(uint64(sizeBytes)),
		)})
	}
	return uploadID, nil
}

// filterTaskConfigsByChangedFiles filters out any tasks that don't have changed files.
func (d *deployer) filterTaskConfigsByChangedFiles(ctx context.Context, taskConfigs []discover.TaskConfig) ([]discover.TaskConfig, error) {
	var filteredTaskConfigs []discover.TaskConfig
	for _, tc := range taskConfigs {
		if tc.TaskRoot != "" {
			contains, err := containsFile(tc.TaskRoot, d.cfg.changedFiles)
			if err != nil {
				return nil, err
			}
			if contains {
				filteredTaskConfigs = append(filteredTaskConfigs, tc)
				continue
			}
		}
		// If the definition file changed, then the task should be included.
		if tc.Def != nil {
			defnFilePath := tc.Def.GetDefnFilePath()
			if defnFilePath != "" {
				equals, err := equalsFile(defnFilePath, d.cfg.changedFiles)
				if err != nil {
					return nil, err
				}
				if equals {
					filteredTaskConfigs = append(filteredTaskConfigs, tc)
				}
			}
		}
	}
	if len(taskConfigs) != len(filteredTaskConfigs) {
		d.logger.Log("Changed files specified. Filtered %d task(s) to %d affected task(s)", len(taskConfigs), len(filteredTaskConfigs))
	}
	return filteredTaskConfigs, nil
}

var skippedDeployErr = errors.New("Skipped deploy")

func (d *deployer) printPreDeploySummary(ctx context.Context, taskConfigs []discover.TaskConfig, viewConfigs []discover.ViewConfig, createdTasks map[string]bool) error {
	noun := "task"
	if len(taskConfigs) > 1 {
		noun = fmt.Sprintf("%ss", noun)
	}
	if len(taskConfigs) > 0 {
		d.logger.Log("Deploying %v %v:\n", len(taskConfigs), noun)
	}

	var hasDiff bool
	for _, tc := range taskConfigs {
		slug := tc.Def.GetSlug()
		kind, _, _ := tc.Def.GetKindAndOptions()

		d.logger.Log(logger.Bold(slug))
		d.logger.Log("Type: %s", kind)

		// Log root directory only if there's an entrypoint.
		if _, err := tc.Def.Entrypoint(); err == definitions.ErrNoEntrypoint {
			// nothing
		} else if err != nil {
			return err
		} else {
			d.logger.Log("Root directory: %s", relpath(tc.TaskRoot))
		}

		// Log definition file if this came from a definition file.
		if tc.Source == discover.ConfigSourceDefn {
			defPath := relpath(tc.Def.GetDefnFilePath())
			d.logger.Log("Definition file: %s", defPath)
		}

		d.logger.Log("URL: %s", d.cfg.client.TaskURL(slug, d.cfg.envSlug))

		// Skip printing diff if this didn't come from a defn file.
		if tc.Source == discover.ConfigSourceDefn {
			difflines, err := d.getDefinitionDiff(ctx, tc, createdTasks[tc.TaskID])
			if err != nil {
				return err
			}

			if len(difflines) == 1 {
				// If it's just one line, it's not actually a diff.
				d.logger.Log(difflines[0])
			} else if len(difflines) > 1 {
				// Otherwise, indent it for readability.
				for _, line := range difflines {
					d.logger.Log("  %s", line)
				}
				hasDiff = true
			}
		}

		d.logger.Log("")
	}

	noun = "view"
	if len(viewConfigs) > 1 {
		noun = fmt.Sprintf("%ss", noun)
	}
	if len(viewConfigs) > 0 {
		d.logger.Log("Deploying %v %v:\n", len(viewConfigs), noun)
	}
	// TODO diff view configs and update hasDiff if any views have changed.
	for _, vc := range viewConfigs {
		d.logger.Log(logger.Bold(vc.Def.Slug))
		d.logger.Log("Root directory: %s", relpath(vc.Root))
		d.logger.Log("")
	}

	if hasDiff {
		return d.confirmDeployment(ctx)
	}

	return nil
}

func (d *deployer) getDefinitionDiff(ctx context.Context, taskConfig discover.TaskConfig, isNew bool) ([]string, error) {
	if isNew {
		return []string{"(new task)"}, nil
	}

	task, err := d.cfg.client.GetTask(ctx, libapi.GetTaskRequest{
		Slug:    taskConfig.Def.GetSlug(),
		EnvSlug: d.cfg.envSlug,
	})
	if err != nil {
		if _, ok := err.(*libapi.TaskMissingError); ok {
			// The task is being promoted into a new environment, proceed as normal.
			return []string{"(task created in new environment)"}, nil
		}
		return nil, err
	}

	defPath := relpath(taskConfig.Def.GetDefnFilePath())
	defPath = strings.TrimPrefix(defPath, "./")

	oldDef, err := definitions.NewDefinitionFromTask_0_3(ctx, d.cfg.client, task)
	if err != nil {
		return nil, err
	}
	oldYAML, err := oldDef.Marshal(definitions.DefFormatYAML)
	if err != nil {
		return nil, errors.Wrap(err, "Error marshalling current task definition")
	}
	oldYAMLStr := string(oldYAML)
	oldLabel := fmt.Sprintf("a/%s", defPath)

	newYAML, err := taskConfig.Def.Marshal(definitions.DefFormatYAML)
	if err != nil {
		return nil, errors.Wrap(err, "Error marshalling new task definition")
	}
	newYAMLStr := string(newYAML)
	newLabel := fmt.Sprintf("b/%s", defPath)

	edits := myers.ComputeEdits(span.URI(oldLabel), oldYAMLStr, newYAMLStr)
	edits = gotextdiff.LineEdits(oldYAMLStr, edits)
	diff := fmt.Sprint(gotextdiff.ToUnified(oldLabel, newLabel, oldYAMLStr, edits))
	if diff == "" {
		return []string{"(no changes to task definition)"}, nil
	}

	// Log deletes in red & additions in green.
	difflines := strings.Split(diff, "\n")
	pretty := make([]string, len(difflines))
	for i, line := range difflines {
		if strings.HasPrefix(line, "-") {
			pretty[i] = logger.Red("%s", line)
		} else if strings.HasPrefix(line, "+") {
			pretty[i] = logger.Green("%s", line)
		} else {
			pretty[i] = line
		}
	}

	return pretty, nil
}

func (d *deployer) confirmDeployment(ctx context.Context) error {
	if !utils.CanPrompt() {
		// Deploy without confirmation.
		return nil
	}
	wasActive := d.logger.StopLoader()
	question := "Are you sure you want to deploy?"
	ok, err := utils.ConfirmWithAssumptions(question, d.cfg.assumeYes, d.cfg.assumeNo)
	if wasActive {
		d.logger.StartLoader()
	}

	if err != nil {
		return err
	} else if !ok {
		// User answered "no", so bail here.
		return skippedDeployErr
	}
	return nil
}

func (d *deployer) waitForDeploy(ctx context.Context, client api.APIClient, deploymentID string) error {
	d.deployLog(ctx, api.LogLevelInfo, deployLogReq{msg: logger.Gray("Waiting for deployer...")})

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

				d.deployLog(ctx, l.Level, deployLogReq{l.TaskSlug, text})
			}

			deployment, err := client.GetDeployment(ctx, deploymentID)
			if err != nil {
				return errors.Wrap(err, "getting deployment")
			}

			switch {
			case deployment.FailedAt != nil:
				d.deployLog(ctx, api.LogLevelInfo, deployLogReq{msg: logger.Bold(logger.Red("failed: %s", deployment.FailedReason))})
				return errors.New("Deploy failed")
			case deployment.SucceededAt != nil:
				d.deployLog(ctx, api.LogLevelInfo, deployLogReq{msg: logger.Bold(logger.Green("succeeded"))})
				return nil
			case deployment.CancelledAt != nil:
				d.deployLog(ctx, api.LogLevelInfo, deployLogReq{msg: logger.Bold(logger.Red("cancelled"))})
				return errors.New("Deploy cancelled")
			}
		}
	}
}

type deployLogReq struct {
	taskSlug string
	msg      string
}

func (d *deployer) deployLog(ctx context.Context, level api.LogLevel, req deployLogReq, args ...interface{}) {
	buildMsg := fmt.Sprintf("[%s %s] ", logger.Yellow("deploy"), req.taskSlug)
	if req.taskSlug == "" {
		buildMsg = fmt.Sprintf("[%s] ", logger.Yellow("deploy"))
	}
	if level == api.LogLevelDebug {
		d.logger.Log(buildMsg+"["+logger.Blue("debug")+"] "+req.msg, args...)
	} else {
		d.logger.Log(buildMsg+req.msg, args...)
	}
}

func (d *deployer) confirmBuildRoot(root string) error {
	if home, err := os.UserHomeDir(); err != nil {
		return errors.Wrap(err, "getting home dir")
	} else if home != root {
		return nil
	}
	d.logger.Warning("This root is your home directory — deploying will attempt to upload the entire directory.")
	d.logger.Warning("Consider moving your task entrypoint to a subdirectory.")
	wasActive := d.logger.StopLoader()
	if wasActive {
		defer d.logger.StartLoader()
	}
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

// equalsFile returns true if the target file is equal to one of the files.
func equalsFile(target string, files []string) (bool, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false, errors.Wrapf(err, "calculating absolute path of file %s", target)
	}
	for _, cf := range files {
		absCF, err := filepath.Abs(cf)
		if err != nil {
			return false, errors.Wrapf(err, "calculating absolute path of file %s", cf)
		}
		if absTarget == absCF {
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
