package open

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/taskdir"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Config is the open config.
type config struct {
	root *cli.Config
	slug string
	file string
	dev  bool
}

// New returns a new open command.
func New(c *cli.Config) *cobra.Command {
	var cfg = config{root: c}
	cmd := &cobra.Command{
		Use:   "open",
		Short: "Opens a task page in browser",
		Example: heredoc.Doc(`
			airplane tasks open <task_slug>
			airplane tasks open -f <task_definition.yml>
		`),
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				cfg.slug = args[0]
			}
			return run(cmd.Root().Context(), cfg)
		},
	}
	cmd.Flags().StringVarP(&cfg.file, "file", "f", "", "Path to a task definition file.")

	// Unhide this flag once we release tasks-as-code.
	cmd.Flags().BoolVar(&cfg.dev, "dev", false, "Dev mode: warning, not guaranteed to work and subject to change.")
	if err := cmd.Flags().MarkHidden("dev"); err != nil {
		logger.Debug("error: %s", err)
	}

	return cmd
}

// Run runs the open command.
func run(ctx context.Context, cfg config) error {
	var client = cfg.root.Client

	slug := cfg.slug
	if slug == "" {
		if cfg.file == "" {
			return errors.New("expected either a task slug or --file")
		}

		dir, err := taskdir.Open(cfg.file, cfg.dev)
		if err != nil {
			return err
		}
		defer dir.Close()

		var def definitions.DefinitionInterface
		if cfg.dev {
			d, err := dir.ReadDefinition_0_3()
			if err != nil {
				return err
			}
			def = &d
		} else {
			d, err := dir.ReadDefinition()
			if err != nil {
				return err
			}
			def = &d
		}

		if def.GetSlug() == "" {
			return errors.Errorf("no task slug found in task definition at %s", cfg.file)
		}
		slug = def.GetSlug()
	}

	task, err := client.GetTask(ctx, api.GetTaskRequest{
		Slug: slug,
	})
	if err != nil {
		return errors.Wrap(err, "get task")
	}

	taskURL := client.TaskURL(task.Slug)
	logger.Log("Opening %s", taskURL)
	if !utils.Open(taskURL) {
		logger.Log("Could not open browser - try copying and pasting the above URL")
	}

	return nil
}
