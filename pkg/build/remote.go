package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cmd/tasks/deploy/archive"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/taskdir/definitions"
	"github.com/airplanedev/cli/pkg/utils"
	libBuild "github.com/airplanedev/lib/pkg/build"
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

type contextKey string

const (
	taskSlugContextKey contextKey = "taskSlug"
)

// registryTokenGetter gets registry tokens and is optimized for concurrent requests.
type registryTokenGetter struct {
	getRegistryTokenMutex sync.Mutex
	cachedRegistryToken   *api.RegistryTokenResponse
}

type remoteBuildCreator struct {
	registryTokenGetter
	archiver archive.Archiver
}

func NewRemoteBuildCreator(archiver archive.Archiver) BuildCreator {
	return &remoteBuildCreator{
		archiver: archiver,
	}
}

func (d *remoteBuildCreator) CreateBuild(ctx context.Context, req Request) (*libBuild.Response, error) {
	ctx = context.WithValue(ctx, taskSlugContextKey, req.Def.GetSlug())
	if err := confirmBuildRoot(req.Root); err != nil {
		return nil, err
	}
	loader := logger.NewLoader(logger.LoaderOpts{HideLoader: logger.EnableDebug})
	defer loader.Stop()
	loader.Start()

	// Before performing a remote build, we must first update kind/kindOptions
	// since the remote build relies on pulling those from the tasks table (for now).
	if err := updateKindAndOptions(ctx, req.Client, req.Def, req.Shim); err != nil {
		return nil, err
	}

	buildLog(ctx, api.LogLevelInfo, loader, logger.Gray("Authenticating with Airplane..."))
	registry, err := d.getRegistryToken(ctx, req.Client)
	if err != nil {
		return nil, err
	}

	buildLog(ctx, api.LogLevelInfo, loader, logger.Gray("Packaging and uploading %s to build task...", req.Root))

	uploadID, sizeBytes, err := d.archiver.Archive(ctx, req.Root)
	if err != nil {
		return nil, err
	}
	if sizeBytes > 0 {
		buildLog(ctx, api.LogLevelInfo, loader, logger.Gray("Uploaded %s build archive.",
			humanize.Bytes(uint64(sizeBytes)),
		))
	}

	build, err := req.Client.CreateBuild(ctx, api.CreateBuildRequest{
		TaskID:         req.TaskID,
		SourceUploadID: uploadID,
		Env:            req.TaskEnv,
		GitMeta:        req.GitMeta,
	})
	if err != nil {
		return nil, errors.Wrap(err, "creating build")
	}
	logger.Debug("Created build with id=%s", build.Build.ID)

	if err := waitForBuild(ctx, loader, req.Client, build.Build.ID); err != nil {
		return nil, err
	}

	imageURL := fmt.Sprintf("%s/task-%s:%s",
		registry.Repo,
		libBuild.SanitizeTaskID(req.TaskID),
		build.Build.ID,
	)

	return &libBuild.Response{
		ImageURL: imageURL,
		BuildID:  build.Build.ID,
	}, nil
}

func (d *registryTokenGetter) getRegistryToken(ctx context.Context, client api.APIClient) (registryToken api.RegistryTokenResponse, err error) {
	d.getRegistryTokenMutex.Lock()
	defer d.getRegistryTokenMutex.Unlock()
	if d.cachedRegistryToken != nil {
		registryToken = *d.cachedRegistryToken
	} else {
		registryToken, err = client.GetRegistryToken(ctx)
		if err != nil {
			return registryToken, errors.Wrap(err, "getting registry token")
		}
		d.cachedRegistryToken = &registryToken
	}
	return registryToken, nil
}

func updateKindAndOptions(ctx context.Context, client api.APIClient, def definitions.DefinitionInterface, shim bool) error {
	task, err := client.GetTask(ctx, def.GetSlug())
	if err != nil {
		return err
	}

	kind, kindOptions, err := def.GetKindAndOptions()
	if err != nil {
		return err
	}

	// Conditionally instruct the remote builder API to perform a shim-based build.
	if shim {
		kindOptions["shim"] = "true"
	}

	// Normalize entrypoint to `/` regardless of OS.
	// CLI might be run from Windows or not Windows, but remote API is on Linux.
	if ep, ok := kindOptions["entrypoint"].(string); ok {
		kindOptions["entrypoint"] = filepath.ToSlash(ep)
	}

	_, err = client.UpdateTask(ctx, api.UpdateTaskRequest{
		Kind:        kind,
		KindOptions: kindOptions,

		// The following fields are not updated until after the build finishes.
		Slug:                       task.Slug,
		Name:                       task.Name,
		Description:                task.Description,
		Image:                      task.Image,
		Command:                    task.Command,
		Arguments:                  task.Arguments,
		Parameters:                 task.Parameters,
		Constraints:                task.Constraints,
		Env:                        task.Env,
		ResourceRequests:           task.ResourceRequests,
		Resources:                  task.Resources,
		Repo:                       task.Repo,
		RequireExplicitPermissions: task.RequireExplicitPermissions,
		Permissions:                task.Permissions,
		Timeout:                    task.Timeout,
	})
	if err != nil {
		return errors.Wrapf(err, "updating task %s", def.GetSlug())
	}

	return nil
}

func waitForBuild(ctx context.Context, loader logger.Loader, client api.APIClient, buildID string) error {
	loader.Start()
	buildLog(ctx, api.LogLevelInfo, loader, logger.Gray("Waiting for builder..."))

	t := time.NewTicker(time.Second)

	var prevToken string
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			r, err := client.GetBuildLogs(ctx, buildID, prevToken)
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

				buildLog(ctx, l.Level, loader, text)
			}

			b, err := client.GetBuild(ctx, buildID)
			if err != nil {
				return errors.Wrap(err, "getting build")
			}

			if b.Build.Status.Stopped() {
				loader.Stop()
				switch b.Build.Status {
				case api.BuildCancelled:
					buildLog(ctx, api.LogLevelInfo, loader, logger.Bold(logger.Yellow("cancelled")))
					return errors.New("Build cancelled")
				case api.BuildFailed:
					buildLog(ctx, api.LogLevelInfo, loader, logger.Bold(logger.Red("failed")))
					return errors.New("Build failed")
				case api.BuildSucceeded:
					buildLog(ctx, api.LogLevelInfo, loader, logger.Bold(logger.Green("succeeded")))
				}

				return nil
			}
			loader.Start()
		}
	}
}

func buildLog(ctx context.Context, level api.LogLevel, loader logger.Loader, msg string, args ...interface{}) {
	taskSlug := ctx.Value(taskSlugContextKey).(string)
	loaderActive := loader.IsActive()
	loader.Stop()
	buildMsg := fmt.Sprintf("[%s %s] ", logger.Yellow("build"), taskSlug)
	if level == api.LogLevelDebug {
		logger.Log(buildMsg+"["+logger.Blue("debug")+"] "+msg, args...)
	} else {
		logger.Log(buildMsg+msg, args...)
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
