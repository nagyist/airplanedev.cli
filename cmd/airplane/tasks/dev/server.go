package dev

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/resource"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/pkg/errors"
)

func runLocalDevServer(ctx context.Context, cfg taskDevConfig) error {
	authInfo, err := cfg.root.Client.AuthInfo(ctx)
	if err != nil {
		return err
	}
	appURL := cfg.root.Client.AppURL()

	var devEnv api.Env
	devServerHost := fmt.Sprintf("127.0.0.1:%d", cfg.port)
	if cfg.local {
		// Use a special env ID and slug when executing tasks locally and the `--env` flag doesn't apply.
		devEnv = env.NewLocalEnv()
	} else {
		devEnv, err = cfg.root.Client.GetEnv(ctx, cfg.envSlug)
		if err != nil {
			return err
		}
	}

	localExecutor := &dev.LocalExecutor{}

	localClient := &api.Client{
		Host:   devServerHost,
		Token:  cfg.root.Client.Token,
		Source: cfg.root.Client.Source,
		APIKey: cfg.root.Client.APIKey,
		TeamID: cfg.root.Client.TeamID,
	}

	// Use absolute path to dev root to allow the local dev server to more easily calculate relative paths.
	dir := filepath.Dir(cfg.fileOrDir)
	absoluteDir, err := filepath.Abs(dir)
	if err != nil {
		return errors.Wrap(err, "getting absolute directory of dev server root")
	}
	apiServer, err := server.Start(server.Options{
		CLI:         cfg.root,
		LocalClient: localClient,
		DevConfig:   cfg.devConfig,
		Env:         devEnv,
		Executor:    localExecutor,
		Port:        cfg.port,
		Dir:         absoluteDir,
		AuthInfo:    authInfo,
	})
	if err != nil {
		return errors.Wrap(err, "starting local dev server")
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	logger.Log("Discovering tasks and views...")

	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{})
	// Discover local tasks and views in the directory of the file.
	d := &discover.Discoverer{
		TaskDiscoverers: []discover.TaskDiscoverer{
			&discover.DefnDiscoverer{
				Client: localClient,
				Logger: l,
			},
			&discover.CodeTaskDiscoverer{
				Client: localClient,
				Logger: l,
			},
		},
		ViewDiscoverers: []discover.ViewDiscoverer{
			&discover.ViewDefnDiscoverer{
				Client: localClient,
				Logger: l,
			},
			&discover.CodeViewDiscoverer{
				Client: localClient,
				Logger: l,
			},
		},
		EnvSlug: cfg.envSlug,
		Client:  localClient,
	}

	taskConfigs, viewConfigs, err := d.Discover(ctx, cfg.fileOrDir)
	if err != nil {
		// This is a temporary workaround for SQL + REST tasks if a resource referenced in the task definition doesn't
		// exist yet. Ideally, we don't error during discovery, and allow the user to create the resource through the
		// studio.
		var rerr libapi.ResourceMissingError
		if errors.As(err, &rerr) {
			var message string
			// Handle demo db in a special case.
			if rerr.Slug == resource.DemoDBName || rerr.Slug == resource.DemoDBSlug {
				message = "Demo DB resource not found - please create it using `airplane demo create-db`."
			} else {
				message = fmt.Sprintf("Resource with name or slug %s not found, please create it using `airplane dev config set-resource`", rerr.Slug)
				if devEnv.ID != env.LocalEnvID {
					message += fmt.Sprintf(" or create it remotely at %s/settings/resources/new.", cfg.root.Client.AppURL())
				} else {
					message += "."
				}
			}

			return errors.Errorf(message)
		}
		return errors.Wrap(err, "discovering task configs")
	}

	// Print out discovered views and tasks to the user
	taskNoun := "tasks"
	if len(taskConfigs) == 1 {
		taskNoun = "task"
	}
	logger.Log("Found %d %s:", len(taskConfigs), taskNoun)
	for _, task := range taskConfigs {
		logger.Log("- %s", task.Def.GetName())
	}

	logger.Log("")

	viewNoun := "views"
	if len(viewConfigs) == 1 {
		viewNoun = "view"
	}
	logger.Log("Found %d %s:", len(viewConfigs), viewNoun)
	for _, view := range viewConfigs {
		logger.Log("- %s", view.Def.Name)
	}

	// Register discovered tasks with local dev server
	warnings, err := apiServer.RegisterTasksAndViews(ctx, taskConfigs, viewConfigs)
	if err != nil {
		return err
	}
	if len(warnings.UnsupportedApps) > 0 {
		logger.Log(" ")
		logger.Log("Skipping %v unsupported tasks or views:", len(warnings.UnsupportedApps))
		for _, w := range warnings.UnsupportedApps {
			logger.Log("- %s: %s", w.AppName, w.Reason)
		}
	}

	if len(warnings.UnattachedResources) > 0 {
		logger.Log(" ")
		unattachedResourcesMsg := "The following tasks have resource attachments that are not defined in the dev config file"
		if devEnv.ID == env.LocalEnvID {
			unattachedResourcesMsg += "."
		} else {
			unattachedResourcesMsg += fmt.Sprintf(" or remotely in %s.", logger.Bold(devEnv.Name))
		}
		unattachedResourcesMsg += " Please add them through the studio or run `airplane dev config set-resource`."
		logger.Log(unattachedResourcesMsg)
		for _, ur := range warnings.UnattachedResources {
			logger.Log("- %s: %s", ur.TaskName, ur.ResourceSlugs)
		}
	}

	logger.Log("")
	if devEnv.ID == env.LocalEnvID {
		logger.Log("You have not set a fallback environment. All tasks and resources must be available locally. " +
			"You can configure a fallback environment via `--env`.")
	} else {
		logger.Log("Your environment is set to %s.", logger.Bold(devEnv.Name))
		logger.Log("- Any task that is not available locally will execute in your %s environment.", logger.Bold(devEnv.Name))
		logger.Log("- Any resources not declared in your dev config will be loaded from your %s environment.", logger.Bold(devEnv.Name))
	}

	logger.Log("")
	studioURL := fmt.Sprintf("%s/studio?host=http://localhost:%d", appURL, cfg.port)
	logger.Log("Started studio session at %s (^C to quit)", logger.Blue(studioURL))
	logger.Log("Press ENTER to open the studio in the browser.")

	// Execute the flow to open the studio in the browser in a separate goroutine so fmt.Scanln doesn't capture
	// termination signals.
	go func() {
		fmt.Scanln()
		if ok := utils.Open(studioURL); !ok {
			logger.Log("Something went wrong. Try running the command with the --debug flag for more details.")
		}
	}()

	// Wait for termination signal (e.g. Ctrl+C)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := apiServer.Stop(ctx); err != nil {
		return errors.Wrap(err, "stopping api server")
	}

	return nil
}
