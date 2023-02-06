package dev

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/build"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/cli/pkg/server/filewatcher"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
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

	localClientDevServerHost := ""
	studioUIHost := ""
	var studioUIToken *string
	var ln net.Listener
	if cfg.tunnel {
		// Obtain user-specific ngrok auth token.
		tokenResp, err := cfg.root.Client.GetTunnelToken(ctx)
		if err != nil {
			return errors.Wrap(err, "unable to acquire tunnel token")
		}

		randString := utils.RandomString(20, utils.CharsetAlphaNumeric)
		studioUIToken = &randString
		localClientDevServerHost = fmt.Sprintf("%s.t.airplane.sh", authInfo.User.ID)
		ln, err = ngrok.Listen(ctx,
			config.HTTPEndpoint(
				config.WithDomain(localClientDevServerHost),
			),
			ngrok.WithAuthtoken(tokenResp.Token),
		)
		if err != nil {
			return errors.Wrap(err, "failed to start tunnel")
		}
		studioUIHost = fmt.Sprintf("https://%s", localClientDevServerHost)
		if err := cfg.root.Client.SetDevSecret(ctx, randString); err != nil {
			return errors.Wrap(err, "setting dev token")
		}
	}

	apiServer, port, err := server.Start(server.Options{
		Port:     cfg.port,
		Expose:   cfg.sandbox,
		Listener: ln,
		Token:    studioUIToken,
	})
	if err != nil {
		return errors.Wrap(err, "starting local dev server")
	}

	if !cfg.tunnel {
		localClientDevServerHost = fmt.Sprintf("127.0.0.1:%d", port)
		studioUIHost = fmt.Sprintf("http://localhost:%d", port)
	}

	localClient := api.NewClient(api.ClientOpts{
		Host:        localClientDevServerHost,
		Token:       cfg.root.Client.Token,
		TunnelToken: studioUIToken,
		Source:      cfg.root.Client.Source,
		APIKey:      cfg.root.Client.APIKey,
		TeamID:      cfg.root.Client.TeamID,
	})

	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{})
	// Discover local tasks and views in the directory of the file.
	d := &discover.Discoverer{
		TaskDiscoverers: []discover.TaskDiscoverer{
			&discover.DefnDiscoverer{
				Client:                  &localClient,
				Logger:                  l,
				DisableNormalize:        true,
				DoNotVerifyMissingTasks: true,
			},
			&discover.CodeTaskDiscoverer{
				Client:                  &localClient,
				Logger:                  l,
				DoNotVerifyMissingTasks: true,
			},
		},
		ViewDiscoverers: []discover.ViewDiscoverer{
			&discover.ViewDefnDiscoverer{
				Client:                  &localClient,
				Logger:                  l,
				DoNotVerifyMissingViews: true,
			},
			&discover.CodeViewDiscoverer{
				Client:                  &localClient,
				Logger:                  l,
				DoNotVerifyMissingViews: true,
			},
		},
		EnvSlug: cfg.envSlug,
		Client:  &localClient,
	}

	bd := build.BundleDiscoverer(&localClient, l, "")
	var sandboxState *state.SandboxState
	if cfg.sandbox {
		sandboxState = state.NewSandboxState(l)
		sandboxState.Rebuild(ctx, bd, absoluteDir)
	}

	apiServer.RegisterState(&state.State{
		LocalClient:      &localClient,
		RemoteClient:     cfg.root.Client,
		RemoteEnv:        remoteEnv,
		UseFallbackEnv:   cfg.useFallbackEnv,
		DevConfig:        cfg.devConfig,
		Executor:         dev.NewLocalExecutor(absoluteDir),
		Dir:              absoluteDir,
		AuthInfo:         authInfo,
		Discoverer:       d,
		BundleDiscoverer: bd,
		StudioURL:        *appURL,
		SandboxState:     sandboxState,
		ServerHost:       cfg.serverHost,
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

	// The --server-host flag takes precedence over the tunnel and local hosts.
	if cfg.serverHost != "" {
		studioUIHost = cfg.serverHost
	}

	logger.Log("")
	studioURL := fmt.Sprintf("%s/studio?__airplane_host=%s&__env=%s", appURL, studioUIHost, remoteEnv.Slug)
	logger.Log("Started studio session at %s (^C to quit)", logger.Blue(studioURL))

	// Execute the flow to open the studio in the browser in a separate goroutine so fmt.Scanln doesn't capture
	// termination signals.
	if !cfg.sandbox {
		logger.Log("Press ENTER to open the studio in the browser.")
		logger.Log("")
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
	//nolint: contextcheck
	if err := apiServer.Stop(ctx); err != nil {
		return errors.Wrap(err, "stopping api server")
	}

	return nil
}
