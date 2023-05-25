package dev

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	build "github.com/airplanedev/cli/pkg/build/clibuild"
	"github.com/airplanedev/cli/pkg/cli/analytics"
	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/airplanedev/cli/pkg/cli/dev"
	"github.com/airplanedev/cli/pkg/cli/server"
	"github.com/airplanedev/cli/pkg/cli/server/filewatcher"
	"github.com/airplanedev/cli/pkg/cli/server/state"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/pkg/errors"
)

func runLocalDevServer(ctx context.Context, cfg taskDevConfig) error {
	analytics.Track(cfg.root.Client, "Studio started", nil)

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

	serverHost := ""
	var devToken *string
	var ln net.Listener
	localClientOpts := api.ClientOpts{
		Token:  cfg.root.Client.Token(),
		Source: cfg.root.Client.Source(),
		APIKey: cfg.root.Client.APIKey(),
		TeamID: cfg.root.Client.TeamID(),
	}

	if cfg.tunnel {
		var subdomain string
		devToken, subdomain, ln, err = configureTunnel(ctx, cfg.root.Client, authInfo)
		if err != nil {
			return err
		}
		localClientOpts.TunnelToken = devToken
		localClientOpts.Host = subdomain
		serverHost = fmt.Sprintf("https://%s", subdomain)
	} else if cfg.sandbox {
		devToken, err = configureSandbox(ctx, cfg.root.Client, cfg.namespace, cfg.key)
		if err != nil {
			return err
		}
	}

	apiServer, port, err := server.Start(server.Options{
		Port:     cfg.port,
		Sandbox:  cfg.sandbox,
		Listener: ln,
		Token:    devToken,
	})
	if err != nil {
		return errors.Wrap(err, "starting local dev server")
	}

	// The passed in --server-host always takes precedence.
	if cfg.serverHost != "" {
		serverHost = cfg.serverHost
	}

	if localClientOpts.Host == "" {
		localClientOpts.Host = fmt.Sprintf("127.0.0.1:%d", port)
	}

	localClient := api.NewClient(localClientOpts)

	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{})
	discoveryEnvVars := dev.GetDiscoveryEnvVars(cfg.devConfig)
	// Discover local tasks and views in the directory of the file.
	d := &discover.Discoverer{
		TaskDiscoverers: []discover.TaskDiscoverer{
			&discover.DefnDiscoverer{
				Client:                  localClient,
				Logger:                  l,
				DisableNormalize:        true,
				DoNotVerifyMissingTasks: true,
			},
			&discover.CodeTaskDiscoverer{
				Client:                  localClient,
				Logger:                  l,
				DoNotVerifyMissingTasks: true,
				Env:                     discoveryEnvVars,
			},
		},
		ViewDiscoverers: []discover.ViewDiscoverer{
			&discover.ViewDefnDiscoverer{
				Client:                  localClient,
				Logger:                  l,
				DoNotVerifyMissingViews: true,
			},
			&discover.CodeViewDiscoverer{
				Client:                  localClient,
				Logger:                  l,
				DoNotVerifyMissingViews: true,
				Env:                     discoveryEnvVars,
			},
		},
		EnvSlug: cfg.envSlug,
		Client:  localClient,
	}

	bd := build.BundleDiscoverer(localClient, l, "")
	var sandboxState *state.SandboxState
	if cfg.sandbox {
		sandboxState = state.NewSandboxState(l)
		sandboxState.RebuildSynchronously(ctx, bd, absoluteDir)
	}

	var envSlug *string
	if cfg.useFallbackEnv {
		// Make sure to get a pointer to the actual slug, in case someone passes in `--env ""`.
		envSlug = pointers.String(remoteEnv.Slug)
	}
	apiServer.RegisterState(&state.State{
		Flagger:              cfg.root.Flagger,
		LocalClient:          localClient,
		RemoteClient:         cfg.root.Client,
		InitialRemoteEnvSlug: envSlug,
		DevConfig:            cfg.devConfig,
		// TODO: can we pass ctx here? This was left as-is during the lib/cli merge.
		//nolint:contextcheck
		Executor:         dev.NewLocalExecutor(),
		Dir:              absoluteDir,
		AuthInfo:         authInfo,
		Discoverer:       d,
		BundleDiscoverer: bd,
		StudioURL:        *appURL,
		SandboxState:     sandboxState,
		ServerHost:       serverHost,
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	fmt.Fprint(os.Stderr, "Discovering tasks and views... ")
	taskConfigs, viewConfigs, err := apiServer.DiscoverTasksAndViews(ctx, cfg.fileOrDir)
	if err != nil {
		logger.Log("")
		return err
	}
	// Print out discovered views and tasks to the user
	numTasks := 0
	for range taskConfigs {
		numTasks++
	}

	taskNoun := "tasks"
	if numTasks == 1 {
		taskNoun = "task"
	}

	viewNoun := "views"
	if len(viewConfigs) == 1 {
		viewNoun = "view"
	}
	logger.Log(
		"Registered %s %s and %s %s.",
		logger.Green(strconv.Itoa(numTasks)),
		logger.Green(taskNoun),
		logger.Green(strconv.Itoa(len(viewConfigs))),
		logger.Green(viewNoun),
	)

	// Register discovered tasks with local dev server
	if err := apiServer.RegisterTasksAndViews(ctx, state.DiscoverOpts{
		Tasks: taskConfigs,
		Views: viewConfigs,
	}); err != nil {
		return err
	}

	logger.Log("")
	if cfg.useFallbackEnv {
		logger.Log("Your fallback environment is set to %s.", logger.Bold(remoteEnv.Name))
		logger.Log("- Any task not registered locally will execute in %s.", logger.Bold(remoteEnv.Name))
		logger.Log("- Any resource or config not declared in your dev config will be loaded from %s.", logger.Bold(remoteEnv.Name))
	} else {
		logger.Log("All tasks, resources, and configs must be available locally. You can configure a fallback environment via `--env`.")
	}

	// Start watching for changes and reload apps, unless it's disabled.
	if cfg.disableWatchMode {
		logger.Log("")
		logger.Log("Changes require restarting the studio to take effect.")
	} else {
		fileWatcher := filewatcher.NewAppWatcher(filewatcher.AppWatcherOpts{
			PollInterval: time.Millisecond * 200,
			Callback: func(e filewatcher.Event) error {
				return apiServer.ReloadApps(context.Background(), cfg.fileOrDir, e)
			},
		})
		var toWatch []string
		if cfg.devConfigPath != "" && fsx.Exists(cfg.devConfigPath) {
			toWatch = append(toWatch, cfg.devConfigPath)
		}
		err := fileWatcher.Watch(cfg.fileOrDir, toWatch...)
		if err != nil {
			return errors.Wrap(err, "starting filewatcher")
		}
		defer fileWatcher.Stop()
	}

	logger.Log("")
	studioHost := serverHost
	if studioHost == "" {
		studioHost = fmt.Sprintf("http://localhost:%d", port)
	}

	studioURL := fmt.Sprintf("%s/studio?__airplane_host=%s&__env=%s", appURL, studioHost, remoteEnv.Slug)
	logger.Log("Started studio session at %s (^C to quit)", logger.Blue(studioURL))

	// Execute the flow to open the studio in the browser in a separate goroutine so bufio.NewReader doesn't capture
	// termination signals.
	if !cfg.sandbox {
		logger.Log("Press ENTER to open the studio in the browser.")
		logger.Log("")
		go func() {
			// Use bufio.NewReader to properly handle interrupts on Windows. fmt.Scanln() does not return an error
			// when interrupted on Windows, and so there's no way to distinguish between a user pressing enter vs
			// Ctrl+C here.
			_, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
			if err != nil {
				return
			}

			if ok := utils.Open(studioURL); !ok {
				logger.Log("Something went wrong. Try running the command with the --debug flag for more details.")
			}
		}()
	}

	// Wait for termination signal (e.g. Ctrl+C)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//nolint: contextcheck
	if err := apiServer.Stop(ctx); err != nil {
		return errors.Wrap(err, "stopping api server")
	}

	return nil
}
