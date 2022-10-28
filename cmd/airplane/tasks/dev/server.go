package dev

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/cli/pkg/server/filewatcher"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/pkg/errors"
)

func runLocalDevServer(ctx context.Context, cfg taskDevConfig) error {
	authInfo, err := cfg.root.Client.AuthInfo(ctx)
	if err != nil {
		return err
	}
	appURL := cfg.root.Client.AppURL()

	// Always fetch the remote environment (default if cfg.envSlug is empty) to allow us to add default remote
	// resources.
	remoteEnv, err := cfg.root.Client.GetEnv(ctx, cfg.envSlug)
	if err != nil {
		return err
	}

	devServerHost := fmt.Sprintf("127.0.0.1:%d", cfg.port)
	localClient := &api.Client{
		Host:   devServerHost,
		Token:  cfg.root.Client.Token,
		Source: cfg.root.Client.Source,
		APIKey: cfg.root.Client.APIKey,
		TeamID: cfg.root.Client.TeamID,
	}

	// Use absolute path to dev root to allow the local dev server to more easily calculate relative paths.
	dir := cfg.fileOrDir
	fileInfo, err := os.Stat(cfg.fileOrDir)
	if err != nil {
		return errors.Wrapf(err, "describing file information for %s", cfg.fileOrDir)
	}
	if !fileInfo.IsDir() {
		dir = filepath.Dir(cfg.fileOrDir)
	}

	absoluteDir, err := filepath.Abs(dir)
	if err != nil {
		return errors.Wrap(err, "getting absolute directory of dev server root")
	}

	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{})
	// Discover local tasks and views in the directory of the file.
	d := &discover.Discoverer{
		TaskDiscoverers: []discover.TaskDiscoverer{
			&discover.DefnDiscoverer{
				Client:           localClient,
				Logger:           l,
				DisableNormalize: true,
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

	localExecutor := &dev.LocalExecutor{}
	apiServer, err := server.Start(server.Options{
		LocalClient:    localClient,
		RemoteClient:   cfg.root.Client,
		RemoteEnv:      remoteEnv,
		UseFallbackEnv: cfg.useFallbackEnv,
		DevConfig:      cfg.devConfig,
		Executor:       localExecutor,
		Port:           cfg.port,
		Dir:            absoluteDir,
		AuthInfo:       authInfo,
		Discoverer:     d,
	})
	if err != nil {
		return errors.Wrap(err, "starting local dev server")
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	fmt.Fprint(os.Stderr, "Discovering tasks, workflows, and views...")
	taskConfigs, viewConfigs, err := apiServer.DiscoverTasksAndViews(ctx, cfg.fileOrDir)
	if err != nil {
		logger.Log("")
		return err
	}
	// Print out discovered views and tasks to the user
	numTasks := 0
	numWorkflows := 0
	for _, tc := range taskConfigs {
		if tc.Def.GetRuntime() == build.TaskRuntimeWorkflow {
			numWorkflows++
		} else {
			numTasks++
		}
	}

	taskNoun := "tasks"
	if numTasks == 1 {
		taskNoun = "task"
	}

	workflowNoun := "workflows"
	if numWorkflows == 1 {
		workflowNoun = "workflow"
	}

	viewNoun := "views"
	if len(viewConfigs) == 1 {
		viewNoun = "view"
	}
	logger.Log(
		"registered %s %s, %s %s, and %s %s.",
		logger.Green(strconv.Itoa(numTasks)),
		logger.Green(taskNoun),
		logger.Green(strconv.Itoa(numWorkflows)),
		logger.Green(workflowNoun),
		logger.Green(strconv.Itoa(len(viewConfigs))),
		logger.Green(viewNoun),
	)

	// Register discovered tasks with local dev server
	warnings, err := apiServer.RegisterTasksAndViews(ctx, server.DiscoverOpts{
		Tasks: taskConfigs,
		Views: viewConfigs,
	})
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
		if cfg.useFallbackEnv {
			unattachedResourcesMsg += fmt.Sprintf(" or remotely in %s.", logger.Bold(remoteEnv.Name))
		} else {
			unattachedResourcesMsg += "."
		}
		unattachedResourcesMsg += " Please add them through the studio or run `airplane dev config set-resource`."
		logger.Log(unattachedResourcesMsg)
		for _, ur := range warnings.UnattachedResources {
			logger.Log("- %s: %s", ur.TaskName, ur.ResourceSlugs)
		}
	}

	logger.Log("")
	if cfg.useFallbackEnv {
		logger.Log("Your fallback environment is set to %s.", logger.Bold(remoteEnv.Name))
		logger.Log("- Any task not registered locally will execute in %s.", logger.Bold(remoteEnv.Name))
		logger.Log("- Any resource not declared in your dev config will be loaded from %s.", logger.Bold(remoteEnv.Name))
	} else {
		logger.Log("All tasks and resources must be available locally. You can configure a fallback environment via `--env`.")
	}

	// Start watching for changes and reload apps, unless it's disabled.
	if cfg.disableWatchMode {
		logger.Log("")
		logger.Log("Changes require restarting the studio to take effect.")
	} else {
		fileWatcher := filewatcher.NewAppWatcher(filewatcher.AppWatcherOpts{
			PollInterval: time.Millisecond * 200,
			IsValid:      filewatcher.IsValidDefinitionFile,
			Callback: func(e filewatcher.Event) error {
				return apiServer.ReloadApps(context.Background(), e.Path, cfg.fileOrDir, e)
			},
		})
		err := fileWatcher.Watch(cfg.fileOrDir)
		if err != nil {
			return errors.Wrap(err, "starting filewatcher")
		}
		defer fileWatcher.Stop()
	}

	logger.Log("")
	studioURL := fmt.Sprintf("%s/studio?host=http://localhost:%d&__env=%s", appURL, cfg.port, remoteEnv.Slug)
	logger.Log("Started studio session at %s (^C to quit)", logger.Blue(studioURL))
	logger.Log("Press ENTER to open the studio in the browser.")
	logger.Log("")

	// Execute the flow to open the studio in the browser in a separate goroutine so fmt.Scanln doesn't capture
	// termination signals.
	if !cfg.nonInteractive {
		go func() {
			fmt.Scanln()
			if ok := utils.Open(studioURL); !ok {
				logger.Log("Something went wrong. Try running the command with the --debug flag for more details.")
			}
		}()
	}

	// Wait for termination signal (e.g. Ctrl+C)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := apiServer.Stop(ctx); err != nil {
		return errors.Wrap(err, "stopping api server")
	}

	return nil
}
