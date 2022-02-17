package deploy

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/cmd/auth/login"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/version/latest"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	root         *cli.Config
	client       api.APIClient
	paths        []string
	local        bool
	changedFiles utils.NewlineFileValue

	upgradeInterpolation bool

	dev       bool
	assumeYes bool
	assumeNo  bool

	envSlug string
}

func New(c *cli.Config) *cobra.Command {
	var cfg = config{
		root:   c,
		client: c.Client,
	}

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a task",
		Long:  "Deploy code from a local directory to Airplane.",
		Example: heredoc.Doc(`
			airplane tasks deploy ./task.ts
			airplane tasks deploy --local ./task.js
			airplane tasks deploy ./my-task.yml
			airplane tasks deploy my-directory
			airplane tasks deploy ./my-task1.yml ./my-task2.yml
		`),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				cfg.paths = args
			} else {
				return errors.New("expected 1 argument: airplane deploy ./path/to/file")
			}
			return run(cmd.Root().Context(), cfg)
		},
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
	}

	cmd.Flags().BoolVarP(&cfg.local, "local", "L", false, "use a local Docker daemon (instead of an Airplane-hosted builder)")
	cmd.Flags().BoolVar(&cfg.upgradeInterpolation, "jst", false, "Upgrade interpolation to JST")
	cmd.Flags().Var(&cfg.changedFiles, "changed-files", "A file with a list of file paths that were changed, one path per line. Only tasks with changed files will be deployed")
	// Remove dev flag + unhide these flags before release!
	cmd.Flags().BoolVar(&cfg.dev, "dev", false, "Dev mode: warning, not guaranteed to work and subject to change.")
	cmd.Flags().BoolVarP(&cfg.assumeYes, "yes", "y", false, "True to specify automatic yes to prompts.")
	cmd.Flags().BoolVarP(&cfg.assumeNo, "no", "n", false, "True to specify automatic no to prompts.")

	if err := cmd.Flags().MarkHidden("dev"); err != nil {
		logger.Debug("error: %s", err)
	}
	if err := cmd.Flags().MarkHidden("yes"); err != nil {
		logger.Debug("error: %s", err)
	}
	if err := cmd.Flags().MarkHidden("no"); err != nil {
		logger.Debug("error: %s", err)
	}

	// Unhide this flag once we release environments.
	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the environment to query. Defaults to your team's default environment.")
	if err := cmd.Flags().MarkHidden("env"); err != nil {
		logger.Debug("unable to hide --env: %s", err)
	}

	return cmd
}

// Set of properties to track when deploying
type taskDeployedProps struct {
	from       string
	kind       build.TaskKind
	taskID     string
	taskSlug   string
	taskName   string
	buildLocal bool
	buildID    string
}

func run(ctx context.Context, cfg config) error {
	latest.CheckLatest(ctx)

	// Check for mutually exclusive flags.
	if cfg.assumeYes && cfg.assumeNo {
		return errors.New("Cannot specify both --yes and --no")
	}

	l := &logger.StdErrLogger{}
	loader := logger.NewLoader(logger.LoaderOpts{HideLoader: logger.EnableDebug})
	loader.Start()

	d := &discover.Discoverer{
		TaskDiscoverers: []discover.TaskDiscoverer{},
		Client:          cfg.client,
		Logger:          l,
		EnvSlug:         cfg.envSlug,
	}
	var defnDiscoverer *discover.DefnDiscoverer
	if cfg.dev {
		defnDiscoverer := &discover.DefnDiscoverer{
			Client:             cfg.client,
			Logger:             l,
			AssumeYes:          cfg.assumeYes,
			AssumeNo:           cfg.assumeNo,
			MissingTaskHandler: HandleMissingTask(cfg, l, loader),
		}
		d.TaskDiscoverers = append(d.TaskDiscoverers, defnDiscoverer)
	}
	d.TaskDiscoverers = append(d.TaskDiscoverers, &discover.ScriptDiscoverer{})

	taskConfigs, err := d.DiscoverTasks(ctx, cfg.paths...)
	if err != nil {
		return err
	}
	loader.Stop()

	if cfg.dev {
		for i, tc := range taskConfigs {
			taskConfig, err := findDefinitionForScript(ctx, cfg, defnDiscoverer, tc)
			if err != nil {
				return err
			}
			if taskConfig != nil {
				taskConfigs[i] = *taskConfig
			}
		}
	}

	return NewDeployer(cfg, l, DeployerOpts{}).DeployTasks(ctx, taskConfigs)
}

func HandleMissingTask(cfg config, l logger.Logger, loader logger.Loader) func(ctx context.Context, def definitions.DefinitionInterface) (*libapi.Task, error) {
	return func(ctx context.Context, def definitions.DefinitionInterface) (*libapi.Task, error) {
		if !utils.CanPrompt() {
			return nil, nil
		}

		isActive := loader.IsActive()
		loader.Stop()

		question := fmt.Sprintf("Task with slug %s does not exist. Would you like to create a new task?", def.GetSlug())
		ok, err := utils.ConfirmWithAssumptions(question, cfg.assumeYes, cfg.assumeNo)

		if isActive {
			loader.Start()
		}

		if err != nil {
			return nil, err
		} else if !ok {
			// User answered "no", so bail here.
			return nil, nil
		}

		l.Log("Creating task...")
		utr, err := def.GetUpdateTaskRequest(ctx, cfg.client, nil)
		if err != nil {
			return nil, err
		}

		_, err = cfg.client.CreateTask(ctx, api.CreateTaskRequest{
			Slug:             utr.Slug,
			Name:             utr.Name,
			Description:      utr.Description,
			Image:            utr.Image,
			Command:          utr.Command,
			Arguments:        utr.Arguments,
			Parameters:       utr.Parameters,
			Constraints:      utr.Constraints,
			EnvVars:          utr.Env,
			ResourceRequests: utr.ResourceRequests,
			Resources:        utr.Resources,
			Kind:             utr.Kind,
			KindOptions:      utr.KindOptions,
			Repo:             utr.Repo,
			Timeout:          utr.Timeout,
			EnvSlug:          cfg.envSlug,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "creating task %s", def.GetSlug())
		}

		task, err := cfg.client.GetTask(ctx, libapi.GetTaskRequest{
			Slug:    def.GetSlug(),
			EnvSlug: cfg.envSlug,
		})
		if err != nil {
			return nil, errors.Wrap(err, "fetching created task")
		}
		return &task, nil
	}
}

// Look for a defn file that matches this task config, in the directory where the entrypoint is
// located & also in the current directory. Returns nil if the task config wasn't discovered via
// the script discoverer. Used to find relevant definition files if the user accidentally deploys a
// script file when a defn file exists.
func findDefinitionForScript(ctx context.Context, cfg config, defnDiscoverer *discover.DefnDiscoverer, taskConfig discover.TaskConfig) (*discover.TaskConfig, error) {
	if taskConfig.From != discover.TaskConfigSourceScript {
		return nil, nil
	}

	dirs := []string{
		filepath.Dir(taskConfig.TaskEntrypoint),
		".",
	}
	for _, dir := range dirs {
		contents, err := ioutil.ReadDir(dir)
		if err != nil {
			return nil, errors.Wrapf(err, "reading directory %s", dir)
		}

		for _, fileInfo := range contents {
			// Ignore subdirectories.
			if fileInfo.IsDir() {
				continue
			}

			path := filepath.Join(dir, fileInfo.Name())
			if slug, err := defnDiscoverer.IsAirplaneTask(ctx, path); err != nil {
				return nil, err
			} else if slug != taskConfig.Task.Slug {
				continue
			}
			question := fmt.Sprintf("A definition file for task %s exists (%s).\nWould you like to use it?", taskConfig.Task.Slug, relpath(path))
			ok, err := utils.ConfirmWithAssumptions(question, cfg.assumeYes, cfg.assumeNo)
			if err != nil {
				return nil, err
			} else if !ok {
				return nil, nil
			}

			tc, err := defnDiscoverer.GetTaskConfig(ctx, taskConfig.Task, path)
			if err != nil {
				return nil, err
			}
			tc.From = discover.TaskConfigSourceDefn
			return &tc, nil
		}
	}

	return nil, nil
}
