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
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/archive"
	"github.com/airplanedev/lib/pkg/deploy/bundlediscover"
	"github.com/dustin/go-humanize"
	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type deployer struct {
	cfg        Config
	logger     logger.LoggerWithLoader
	archiver   archive.Archiver
	repoGetter GitRepoGetter
}

type DeployerOpts struct {
	Archiver   archive.Archiver
	RepoGetter GitRepoGetter
}

func NewDeployer(cfg Config, l logger.LoggerWithLoader, opts DeployerOpts) *deployer {
	a := archive.NewAPIArchiver(l, cfg.Client, &archive.HttpUploader{})
	if opts.Archiver != nil {
		a = opts.Archiver
	}
	var rg GitRepoGetter
	rg = &FileGitRepoGetter{}
	if opts.RepoGetter != nil {
		rg = opts.RepoGetter
	}
	return &deployer{
		cfg:        cfg,
		logger:     l,
		archiver:   a,
		repoGetter: rg,
	}
}

// Deploy creates a deployment.
func (d *deployer) Deploy(ctx context.Context, bundles []bundlediscover.Bundle) error {
	var err error
	if len(d.cfg.ChangedFiles) > 0 {
		bundles, err = d.filterBundlesByChangedFiles(ctx, bundles)
		if err != nil {
			return err
		}
	}

	if len(bundles) == 0 {
		d.logger.Log("Nothing to deploy")
		return nil
	}

	d.printPreDeploySummary(ctx, bundles)

	var uploadIDs map[string]string
	uploadIDs, err = d.tarAndUploadBatch(ctx, bundles)
	if err != nil {
		return err
	}

	var bundlesToDeploy []api.DeployBundle
	gitRoots := make(map[string]bool)
	var repo *git.Repository
	for _, b := range bundles {
		repo, err = d.repoGetter.GetGitRepo(b.RootPath)
		if err != nil {
			d.logger.Debug("failed to get git repo for %s: %v", b.RootPath, err)
		}
		bundleToDeploy := api.DeployBundle{
			UploadID:    uploadIDs[b.RootPath],
			Name:        filepath.Base(b.RootPath),
			TargetFiles: b.TargetPaths,
			BuildContext: api.BuildContext{
				Type:    b.BuildType,
				Version: b.BuildVersion,
				Base:    b.BuildBase,
			},
		}
		bundlesToDeploy = append(bundlesToDeploy, bundleToDeploy)

		// Get the root directory of the git repo with which the bundle is associated.
		var gitRoot string
		if repo != nil {
			w, err := repo.Worktree()
			if err == nil {
				gitRoot = w.Filesystem.Root()
			}
		}
		gitRoots[gitRoot] = true
	}

	// If bundles in a single deploy come from different git repos, we do not
	// include git information with the deploy.
	mismatchedGitRepos := len(gitRoots) > 1
	if mismatchedGitRepos {
		analytics.ReportMessage("deploy created with multiple git repos")
	}

	var gitMeta api.GitMetadata
	if repo != nil && !mismatchedGitRepos {
		gitMeta, err = GetGitMetadata(repo)
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
	}

	resp, err := d.cfg.Client.CreateDeployment(ctx, api.CreateDeploymentRequest{
		Bundles:     bundlesToDeploy,
		GitMetadata: gitMeta,
		EnvSlug:     d.cfg.EnvSlug,
	})
	if err != nil {
		return err
	}

	d.deployLog(ctx, api.LogLevelInfo, deployLogReq{msg: logger.Gray("Creating deployment...")})
	d.logger.Log(logger.Purple(fmt.Sprintf("\nView deployment: %s\n", d.cfg.Client.DeploymentURL(resp.Deployment.ID, d.cfg.EnvSlug))))

	err = d.waitForDeploy(ctx, d.cfg.Client, resp.Deployment.ID)
	if errors.Is(err, context.Canceled) {
		// Since `ctx` is cancelled, use a fresh context to cancel the deployment.
		//nolint: contextcheck
		cerr := d.cfg.Client.CancelDeployment(context.Background(), api.CancelDeploymentRequest{ID: resp.Deployment.ID})
		if cerr != nil {
			d.logger.Warning("Failed to cancel deployment: %v", cerr)
		} else {
			d.logger.Log("Cancelled deployment")
		}
	}

	return err
}

// tarAndUploadBatch concurrently tars and uploads bundles.
func (d *deployer) tarAndUploadBatch(ctx context.Context, bundles []bundlediscover.Bundle) (map[string]string, error) {
	uploadIDs := make(map[string]string)
	var mu = sync.Mutex{}
	g, ctx := errgroup.WithContext(ctx)
	for _, b := range bundles {
		b := b

		g.Go(func() error {
			uploadID, err := d.tarAndUpload(ctx, b.RootPath)
			if err != nil {
				return err
			}
			mu.Lock()
			uploadIDs[b.RootPath] = uploadID
			mu.Unlock()
			return nil
		})
	}

	groupErr := g.Wait()
	return uploadIDs, groupErr
}

func (d *deployer) tarAndUpload(ctx context.Context, root string) (string, error) {
	if err := d.confirmBuildRoot(root); err != nil {
		return "", err
	}

	d.deployLog(ctx, api.LogLevelInfo, deployLogReq{root, logger.Gray("Packaging and uploading %s...", root)})

	uploadID, sizeBytes, err := d.archiver.Archive(ctx, root)
	if err != nil {
		return "", err
	}
	if sizeBytes > 0 {
		d.deployLog(ctx, api.LogLevelInfo, deployLogReq{root, logger.Gray("Uploaded %s build archive.",
			humanize.Bytes(uint64(sizeBytes)),
		)})
	}
	return uploadID, nil
}

// filterBundlesByChangedFiles filters out any bundles that don't have changed files.
func (d *deployer) filterBundlesByChangedFiles(ctx context.Context, bundles []bundlediscover.Bundle) ([]bundlediscover.Bundle, error) {
	var filteredBundles []bundlediscover.Bundle
	for _, b := range bundles {
		contains, err := containsFile(b.RootPath, d.cfg.ChangedFiles)
		if err != nil {
			return nil, err
		}
		if contains {
			filteredBundles = append(filteredBundles, b)
			continue
		}
	}
	if len(bundles) != len(filteredBundles) {
		d.logger.Log("Changed files specified. Filtered %d bundle(s) to %d affected bundle(s)", len(bundles), len(filteredBundles))
	}
	return filteredBundles, nil
}

func (d *deployer) printPreDeploySummary(ctx context.Context, bundles []bundlediscover.Bundle) {
	noun := "bundle"
	if len(bundles) > 1 {
		noun = fmt.Sprintf("%ss", noun)
	}
	if len(bundles) > 0 {
		d.logger.Log("Deploying %v %v:\n", len(bundles), noun)
	}

	for _, b := range bundles {
		d.logger.Log(logger.Bold(b.RootPath))
		d.logger.Log("Type: %s %s", b.BuildType, b.BuildVersion)
		d.logger.Log("")
	}
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
	bundleRoot string
	msg        string
}

func (d *deployer) deployLog(ctx context.Context, level api.LogLevel, req deployLogReq, args ...interface{}) {
	buildMsg := fmt.Sprintf("[%s %s] ", logger.Yellow("deploy"), filepath.Base(req.bundleRoot))
	if req.bundleRoot == "" {
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
