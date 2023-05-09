// initcmd defines the implementation of the `airplane tasks init` command.
//
// Even though the command is called "init", we can't name the package "init"
// since that conflicts with the Go init function.
package initcmd

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/cli/auth/login"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/cli"
	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/airplanedev/cli/pkg/cli/initcmd"
	"github.com/airplanedev/cli/pkg/cli/prompts"
	"github.com/airplanedev/cli/pkg/definitions"
	"github.com/airplanedev/cli/pkg/runtime"
	_ "github.com/airplanedev/cli/pkg/runtime/javascript"
	_ "github.com/airplanedev/cli/pkg/runtime/python"
	_ "github.com/airplanedev/cli/pkg/runtime/rest"
	_ "github.com/airplanedev/cli/pkg/runtime/shell"
	_ "github.com/airplanedev/cli/pkg/runtime/sql"
	_ "github.com/airplanedev/cli/pkg/runtime/typescript"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	root   *cli.Config
	client api.APIClient
	dryRun bool

	file        string
	from        string
	fromRunbook string

	assumeYes bool
	assumeNo  bool
	envSlug   string
	cmd       *cobra.Command

	inline   bool
	workflow bool

	newTaskInfo newTaskInfo
}

type ConfigOpts struct {
	Client      api.APIClient
	Root        *cli.Config
	FromRunbook string
}

func GetConfig(opts ConfigOpts) config {
	return config{
		client:      opts.Client,
		root:        opts.Root,
		fromRunbook: opts.FromRunbook,
		inline:      true,
	}
}

type newTaskInfo struct {
	name       string
	kind       buildtypes.TaskKind
	kindName   string
	entrypoint string
}

func New(c *cli.Config) *cobra.Command {
	var cfg = GetConfig(ConfigOpts{Client: c.Client, Root: c})

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new task",
		Example: heredoc.Doc(`
			$ airplane tasks init
			$ airplane tasks init --from task_slug
			$ airplane tasks init --from task_slug ./folder/my_task.task.yaml
		`),
		Args: cobra.MaximumNArgs(1),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				cfg.file = args[0]
			}
			return Run(cmd.Root().Context(), cfg)
		},
	}

	cmd.Flags().BoolVarP(&cfg.dryRun, "dry-run", "", false, "True to run a dry run of this command.")
	if err := cmd.Flags().MarkHidden("dry-run"); err != nil {
		logger.Debug("error: %s", err)
	}

	cmd.Flags().StringVar(&cfg.from, "slug", "", "Slug of an existing task to generate from.")
	if err := cmd.Flags().MarkHidden("slug"); err != nil {
		logger.Debug("error: %s", err)
	}

	cmd.Flags().StringVar(&cfg.from, "from", "", "Slug of an existing task to initialize.")
	cmd.Flags().StringVar(&cfg.fromRunbook, "from-runbook", "", "Slug of an existing runbook to convert to a task.")

	cmd.Flags().BoolVarP(&cfg.assumeYes, "yes", "y", false, "True to specify automatic yes to prompts.")
	cmd.Flags().BoolVarP(&cfg.assumeNo, "no", "n", false, "True to specify automatic no to prompts.")

	cmd.Flags().BoolVar(&cfg.inline, "inline", true, "If true, the task will be configured with inline configuration. Only applicable for JavaScript & Python tasks.")
	cmd.Flags().BoolVar(&cfg.workflow, "workflow", false, "Generate a workflow-runtime task.")
	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the environment that the `from` task is in. Defaults to your team's default environment.")
	cfg.cmd = cmd

	return cmd
}

func Run(ctx context.Context, cfg config) error {
	// Check for mutually exclusive flags.
	if cfg.assumeYes && cfg.assumeNo {
		return errors.New("Cannot specify both --yes and --no")
	}
	if cfg.from != "" && cfg.fromRunbook != "" {
		return errors.New("Cannot specify both --from and --from-runbook")
	}

	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{
		WithLoader:      true,
		StartNotLoading: true,
	})
	defer l.StopLoader()

	// workflows are also inline
	cfg.inline = cfg.inline || cfg.workflow || cfg.fromRunbook != ""

	if cfg.fromRunbook != "" {
		return initcmd.InitWorkflowFromRunbook(ctx, initcmd.InitWorkflowFromRunbookRequest{
			Client:      cfg.client,
			Prompter:    cfg.root.Prompter,
			Logger:      l,
			File:        cfg.file,
			FromRunbook: cfg.fromRunbook,
			Inline:      cfg.inline,
			AssumeYes:   cfg.assumeYes,
			AssumeNo:    cfg.assumeNo,
			EnvSlug:     cfg.envSlug,
		})
	}

	if cfg.from == "" {
		// Prompt for new task information.
		if err := promptForNewTask(cfg.file, &cfg.newTaskInfo, cfg.workflow, cfg.root.Prompter); err != nil {
			return err
		}
	}

	_, err := initcmd.InitTask(ctx, initcmd.InitTaskRequest{
		Client:         cfg.client,
		Prompter:       cfg.root.Prompter,
		Logger:         l,
		DryRun:         cfg.dryRun,
		File:           cfg.file,
		FromTask:       cfg.from,
		AssumeYes:      cfg.assumeYes,
		AssumeNo:       cfg.assumeNo,
		EnvSlug:        cfg.envSlug,
		Inline:         cfg.inline,
		Workflow:       cfg.workflow,
		TaskName:       cfg.newTaskInfo.name,
		TaskKind:       cfg.newTaskInfo.kind,
		TaskKindName:   cfg.newTaskInfo.kindName,
		TaskEntrypoint: cfg.newTaskInfo.entrypoint,
	})

	return err
}

var allKindsByName = map[string]buildtypes.TaskKind{
	"Docker":     buildtypes.TaskKindImage,
	"JavaScript": buildtypes.TaskKindNode,
	"Python":     buildtypes.TaskKindPython,
	"Shell":      buildtypes.TaskKindShell,
	"SQL":        buildtypes.TaskKindSQL,
	"REST":       buildtypes.TaskKindREST,
	"GraphQL":    buildtypes.TaskKindBuiltin,
}

// Determines the set of kind names that are eligible for use as tasks.
// The ordering is important as it defines how they are shown in CLI prompts.
var supportedTaskKindNames = []string{
	"JavaScript",
	"Python",
	"SQL",
	"REST",
	"GraphQL",
	"Shell",
	"Docker",
}

// Determines the set of kind names that are eligible for use as workflows.
// The ordering is important as it defines how they are shown in CLI prompts.
var supportedWorkflowKindNames = []string{
	"JavaScript",
}

func promptForNewTask(file string, info *newTaskInfo, workflow bool, p prompts.Prompter) error {
	defFormat := definitions.GetTaskDefFormat(file)
	ext := filepath.Ext(file)
	base := strings.TrimSuffix(filepath.Base(file), ext)
	if defFormat != definitions.DefFormatUnknown {
		// Trim off the .task part, too
		base = strings.TrimSuffix(base, ".task")
	}
	if base == "." {
		base = ""
	}

	// Ask for a name.
	if err := p.Input(
		"What should this task be called?",
		&info.name,
		prompts.WithDefault(base),
	); err != nil {
		return err
	}

	kindNames := supportedTaskKindNames
	if workflow {
		kindNames = supportedWorkflowKindNames
	}

	// Ask for a kind.
	defaultName := kindNames[0]
	guessKind, err := runtime.SuggestKind(ext)
	if err == nil {
		for _, name := range kindNames {
			if kind := allKindsByName[name]; kind == guessKind {
				defaultName = name
				break
			}
		}
	}

	var selectedKindName string
	if err := p.Input(
		"What kind of task should this be?",
		&selectedKindName,
		prompts.WithSelectOptions(kindNames),
		prompts.WithDefault(defaultName),
	); err != nil {
		return err
	}
	info.kindName = selectedKindName
	info.kind = allKindsByName[selectedKindName]
	if info.kind == "" {
		return errors.Errorf("Unknown kind selected: %q", selectedKindName)
	}

	return nil
}
