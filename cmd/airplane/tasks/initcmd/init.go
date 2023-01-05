// initcmd defines the implementation of the `airplane tasks init` command.
//
// Even though the command is called "init", we can't name the package "init"
// since that conflicts with the Go init function.
package initcmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/flags/flagsiface"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/node"
	"github.com/airplanedev/cli/pkg/rb2wf"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	deployconfig "github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/runtime"
	_ "github.com/airplanedev/lib/pkg/runtime/javascript"
	"github.com/airplanedev/lib/pkg/runtime/python"
	_ "github.com/airplanedev/lib/pkg/runtime/python"
	_ "github.com/airplanedev/lib/pkg/runtime/rest"
	_ "github.com/airplanedev/lib/pkg/runtime/shell"
	_ "github.com/airplanedev/lib/pkg/runtime/sql"
	_ "github.com/airplanedev/lib/pkg/runtime/typescript"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

type config struct {
	root        *cli.Config
	client      *api.Client
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
	Client      *api.Client
	Root        *cli.Config
	FromRunbook string
}

func GetConfig(opts ConfigOpts) config {
	return config{client: opts.Client, root: opts.Root, fromRunbook: opts.FromRunbook}
}

type newTaskInfo struct {
	name       string
	kind       build.TaskKind
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

	cmd.Flags().StringVar(&cfg.from, "slug", "", "Slug of an existing task to generate from.")
	if err := cmd.Flags().MarkHidden("slug"); err != nil {
		logger.Debug("error: %s", err)
	}

	cmd.Flags().StringVar(&cfg.from, "from", "", "Slug of an existing task to initialize.")
	cmd.Flags().StringVar(&cfg.fromRunbook, "from-runbook", "", "Slug of an existing runbook to convert to a task.")

	cmd.Flags().BoolVarP(&cfg.assumeYes, "yes", "y", false, "True to specify automatic yes to prompts.")
	cmd.Flags().BoolVarP(&cfg.assumeNo, "no", "n", false, "True to specify automatic no to prompts.")

	cmd.Flags().BoolVar(&cfg.inline, "inline", false, "If true, the task will be configured with inline configuration")
	cmd.Flags().BoolVar(&cfg.workflow, "workflow", false, "Generate a workflow-runtime task. Implies --inline.")
	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the environment that the `from` task is in. Defaults to your team's default environment.")
	cfg.cmd = cmd

	return cmd
}

func Run(ctx context.Context, cfg config) error {
	inlineSetByUser := cfg.cmd != nil && cfg.cmd.Flags().Changed("inline")
	if !inlineSetByUser {
		if cfg.root.Flagger.Bool(ctx, logger.NewStdErrLogger(logger.StdErrLoggerOpts{}), flagsiface.DefaultInlineConfigTasks) {
			cfg.inline = true
		}
	}

	// Check for mutually exclusive flags.
	if cfg.assumeYes && cfg.assumeNo {
		return errors.New("Cannot specify both --yes and --no")
	}
	if cfg.from != "" && cfg.fromRunbook != "" {
		return errors.New("Cannot specify both --from and --from-runbook")
	}

	// workflows are also inline
	cfg.inline = cfg.inline || cfg.workflow || cfg.fromRunbook != ""

	if strings.HasPrefix(cfg.from, "github.com/") || strings.HasPrefix(cfg.from, "https://github.com/") {
		return initWithExample(ctx, cfg)
	}

	if cfg.from == "" && cfg.fromRunbook == "" {
		// Prompt for new task information.
		if err := promptForNewTask(cfg.file, &cfg.newTaskInfo, cfg.inline, cfg.workflow); err != nil {
			return err
		}
	}

	if cfg.fromRunbook != "" {
		return initWorkflowFromRunbook(ctx, cfg)
	}

	return initWithTaskDef(ctx, cfg)
}

func initWithTaskDef(ctx context.Context, cfg config) error {
	client := cfg.client

	// workflows are also inline
	cfg.inline = cfg.inline || cfg.workflow
	var def definitions.Definition_0_3
	if cfg.from != "" {
		task, err := client.GetTask(ctx, libapi.GetTaskRequest{
			Slug:    cfg.from,
			EnvSlug: cfg.envSlug,
		})
		if err != nil {
			return err
		}

		if task.Runtime == build.TaskRuntimeWorkflow {
			cfg.workflow = true
			cfg.inline = true
		}

		def, err = definitions.NewDefinitionFromTask_0_3(ctx, cfg.client, task)
		if err != nil {
			return err
		}
	} else {
		if cfg.newTaskInfo.name == "" || cfg.newTaskInfo.kind == "" {
			return errors.New("missing new task info")
		}

		var err error
		slug := utils.MakeSlug(cfg.newTaskInfo.name)
		if cfg.newTaskInfo.kind == build.TaskKindBuiltin {
			switch cfg.newTaskInfo.kindName {
			case "GraphQL":
				def, err = definitions.NewBuiltinDefinition_0_3(
					cfg.newTaskInfo.name,
					slug,
					&definitions.GraphQLDefinition_0_3{
						Operation: `query get_user {
  user(id: $id) {
    id
	name
	email
  }
}`,
						Variables: map[string]interface{}{
							"id": 1,
						},
						RetryFailures: true,
					},
				)
			default:
				return errors.Errorf("don't know how to initialize task kind=builtin name=%s", cfg.newTaskInfo.kindName)
			}
		} else {
			def, err = definitions.NewDefinition_0_3(
				cfg.newTaskInfo.name,
				slug,
				cfg.newTaskInfo.kind,
				cfg.newTaskInfo.entrypoint,
			)
		}
		if err != nil {
			return err
		}
	}

	kind, err := def.Kind()
	if err != nil {
		return err
	}

	if cfg.workflow {
		def.Runtime = build.TaskRuntimeWorkflow
	}

	localExecutionSupported := false
	if entrypoint, err := def.Entrypoint(); err == definitions.ErrNoEntrypoint {
		// no-op
	} else if err != nil {
		return err
	} else {
		if cfg.file != "" && !definitions.IsTaskDef(cfg.file) {
			entrypoint = cfg.file
		}

		if filepath.Ext(entrypoint) == "tsx" || filepath.Ext(entrypoint) == "jsx" {
			logger.Log("You are trying to deploy a React file. Use `airplane views init` if you'd like to initialize a view.")
			if ok, err := utils.ConfirmWithAssumptions("Are you sure you'd like to continue?", cfg.assumeYes, cfg.assumeNo); err != nil {
				return err
			} else if !ok {
				logger.Log("Exiting flow")
				return nil
			}
		}

		for {
			if cfg.assumeYes && cfg.file != "" {
				entrypoint = cfg.file
			} else {
				entrypoint, err = promptForEntrypoint(def.GetSlug(), kind, entrypoint, cfg)
				if err != nil {
					return err
				}
			}

			if fsx.Exists(entrypoint) {
				var question string
				if !cfg.inline {
					question = fmt.Sprintf("Are you sure you want to link %s? You should only link existing Airplane scripts.", entrypoint)
					if kind == build.TaskKindSQL {
						question = fmt.Sprintf("Would you like to overwrite %s?", entrypoint)
					}
				} else {
					question = fmt.Sprintf("Would you like to overwrite %s?", entrypoint)
				}
				if ok, err := utils.ConfirmWithAssumptions(question, cfg.assumeYes, cfg.assumeNo); err != nil {
					return err
				} else if ok {
					break
				}
			} else {
				break
			}
		}

		if err := def.SetEntrypoint(entrypoint); err != nil {
			return err
		}
		absEntrypoint, err := filepath.Abs(entrypoint)
		if err != nil {
			return errors.Wrap(err, "determining absolute entrypoint")
		}
		if err := def.SetAbsoluteEntrypoint(absEntrypoint); err != nil {
			return err
		}

		r, err := runtime.Lookup(entrypoint, kind)
		if err != nil {
			return errors.Wrapf(err, "unable to init %q - check that your CLI is up to date", entrypoint)
		}
		localExecutionSupported = r.SupportsLocalExecution()

		if kind == build.TaskKindSQL {
			query, err := def.SQL.GetQuery()
			if err != nil {
				// Create a generic entrypoint.
				if err := createEntrypoint(r, entrypoint, nil); err != nil {
					return errors.Wrapf(err, "unable to create entrypoint")
				}
			} else {
				// Write the query to the entrypoint.
				if err := writeEntrypoint(entrypoint, []byte(query), 0644); err != nil {
					return errors.Wrapf(err, "unable to create entrypoint")
				}
			}
			logger.Step("Created %s", entrypoint)
		} else if cfg.inline {
			if err := createInlineEntrypoint(r, entrypoint, &def); err != nil {
				return errors.Wrapf(err, "unable to create entrypoint")
			}
			logger.Step("Created %s", entrypoint)
		} else {
			// Create entrypoint, without comment link, if it doesn't exist.
			if !fsx.Exists(entrypoint) {
				if err := createEntrypoint(r, entrypoint, nil); err != nil {
					return errors.Wrapf(err, "unable to create entrypoint")
				}
				logger.Step("Created %s", entrypoint)
			}
		}
	}

	var resp *writeDefnFileResponse
	if !cfg.inline {
		resp, err = writeDefnFile(&def, cfg)
		if err != nil {
			return err
		}
		if resp == nil {
			return nil
		}
	} else {
		entrypoint, _ := def.Entrypoint()
		resp = &writeDefnFileResponse{
			DefnFile:       entrypoint,
			EntrypointFile: entrypoint,
		}
	}

	if err := runKindSpecificInstallation(ctx, cfg, kind, def); err != nil {
		return err
	}

	suggestNextSteps(suggestNextStepsRequest{
		defnFile:           resp.DefnFile,
		entrypoint:         resp.EntrypointFile,
		showLocalExecution: localExecutionSupported,
		kind:               kind,
		isNew:              cfg.from == "",
	})
	return nil
}

func initWorkflowFromRunbook(ctx context.Context, cfg config) error {
	var entrypoint string
	var err error

	if cfg.assumeYes && cfg.file != "" {
		entrypoint = cfg.file
	} else {
		entrypoint, err = promptForEntrypoint(cfg.fromRunbook, build.TaskKindNode, entrypoint, cfg)
		if err != nil {
			return err
		}
	}

	entrypointDir := filepath.Dir(entrypoint)
	if err := os.MkdirAll(entrypointDir, 0744); err != nil {
		return errors.Wrap(err, "creating output directory")
	}

	// Create a definition that can be used to generate/update the package config.
	def := definitions.Definition_0_3{
		Node: &definitions.NodeDefinition_0_3{
			NodeVersion: "18",
			Base:        build.BuildBaseSlim,
		},
	}
	absEntrypoint, err := filepath.Abs(entrypoint)
	if err != nil {
		return errors.Wrap(err, "determining absolute entrypoint")
	}
	if err := def.SetAbsoluteEntrypoint(absEntrypoint); err != nil {
		return err
	}

	if err := runKindSpecificInstallation(ctx, cfg, build.TaskKindNode, def); err != nil {
		return err
	}

	converter := rb2wf.NewRunbookConverter(
		cfg.client,
		entrypointDir,
		filepath.Base(entrypoint),
	)
	err = converter.Convert(ctx, cfg.fromRunbook)
	if err != nil {
		return err
	}

	suggestNextSteps(suggestNextStepsRequest{
		entrypoint: entrypoint,
		kind:       build.TaskKindNode,
		isNew:      true,
	})

	return nil
}

type writeDefnFileResponse struct {
	DefnFile       string
	EntrypointFile string
}

func writeDefnFile(def *definitions.Definition_0_3, cfg config) (*writeDefnFileResponse, error) {
	// Create task defn file.
	defnFilename := cfg.file
	if !definitions.IsTaskDef(cfg.file) {
		defaultDefnFn := fmt.Sprintf("%s.task.yaml", def.Slug)
		entrypoint, _ := def.Entrypoint()
		fn, err := promptForNewDefinition(defaultDefnFn, entrypoint)
		if err != nil {
			return nil, err
		}
		defnFilename = fn
	}
	if fsx.Exists(defnFilename) {
		// If it exists, check for existence of this file before overwriting it.
		question := fmt.Sprintf("Would you like to overwrite %s?", defnFilename)
		if ok, err := utils.ConfirmWithAssumptions(question, cfg.assumeYes, cfg.assumeNo); err != nil {
			return nil, err
		} else if !ok {
			// User answered "no", so bail here.
			return nil, nil
		}
	}

	// Adjust entrypoint to be relative to the task defn.
	entrypoint, err := def.Entrypoint()
	if err == definitions.ErrNoEntrypoint {
		// no-op
	} else if err != nil {
		return nil, err
	} else {
		absEntrypoint, err := filepath.Abs(entrypoint)
		if err != nil {
			return nil, errors.Wrap(err, "determining absolute entrypoint")
		}

		absDefnFn, err := filepath.Abs(defnFilename)
		if err != nil {
			return nil, errors.Wrap(err, "determining absolute definition file")
		}

		defnDir := filepath.Dir(absDefnFn)
		relpath, err := filepath.Rel(defnDir, absEntrypoint)
		if err != nil {
			return nil, errors.Wrap(err, "determining relative entrypoint")
		}
		if err := def.SetEntrypoint(relpath); err != nil {
			return nil, err
		}
	}

	buf, err := def.GenerateCommentedFile(definitions.GetTaskDefFormat(defnFilename))
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(defnFilename, buf, 0644); err != nil {
		return nil, err
	}
	logger.Step("Created %s", defnFilename)
	return &writeDefnFileResponse{
		DefnFile:       defnFilename,
		EntrypointFile: entrypoint,
	}, nil
}

type suggestNextStepsRequest struct {
	defnFile           string
	entrypoint         string
	showLocalExecution bool
	kind               build.TaskKind
	isNew              bool
}

func suggestNextSteps(req suggestNextStepsRequest) {
	// Update the next steps for inline code config
	if req.isNew {
		steps := []string{}
		switch req.kind {
		case build.TaskKindSQL:
			steps = append(steps, fmt.Sprintf("Add the name of a database resource to %s", req.defnFile))
			steps = append(steps, fmt.Sprintf("Write your query in %s", req.entrypoint))
		case build.TaskKindREST:
			steps = append(steps, fmt.Sprintf("Add the name of a REST resource to %s", req.defnFile))
			steps = append(steps, fmt.Sprintf("Specify the details of your REST request in %s", req.defnFile))
		case build.TaskKindBuiltin:
			steps = append(steps, fmt.Sprintf("Add the name of a resource to %s", req.defnFile))
			steps = append(steps, fmt.Sprintf("Specify the details of your request in %s", req.defnFile))
		case build.TaskKindImage:
			steps = append(steps, fmt.Sprintf("Add the name of a Docker image to %s", req.defnFile))
		default:
			steps = append(steps, fmt.Sprintf("Write your task logic in %s", req.entrypoint))
		}
		if req.defnFile != "" {
			steps = append(steps, fmt.Sprintf("Add a description, parameters, and more details in %s", req.defnFile))
		}
		logger.SuggestSteps("✅ To complete your task:", steps...)
	}

	file := req.defnFile
	if req.defnFile == "" {
		file = req.entrypoint
	}
	if req.showLocalExecution {
		logger.Suggest(
			"⚡ To develop the task locally:",
			"airplane dev %s",
			file,
		)
	}
	logger.Suggest(
		"🛫 To deploy your task to Airplane:",
		"airplane deploy %s",
		file,
	)
}

func promptForEntrypoint(slug string, kind build.TaskKind, defaultEntrypoint string, cfg config) (string, error) {
	entrypoint := defaultEntrypoint
	if entrypoint == "" {
		var err error
		entrypoint, err = getEntrypointFile(slug, kind, cfg)
		if err != nil {
			return "", err
		}
	} else if fsx.Exists(entrypoint) {
		fileInfo, err := os.Stat(entrypoint)
		if err != nil {
			return "", errors.Wrapf(err, "describing %s", entrypoint)
		}
		if fileInfo.IsDir() {
			// The user provided a directory. Create an entrypoint in that directory.
			entrypointFile, err := getEntrypointFile(slug, kind, cfg)
			if err != nil {
				return "", err
			}
			entrypoint = filepath.Join(entrypoint, entrypointFile)
		}
	}
	// Ensure that the file has the correct extension for an inline entrypoint.
	if cfg.inline {
		entrypoint = modifyEntrypointForInline(kind, entrypoint)
	}

	exts := runtime.SuggestExts(kind)
	if err := survey.AskOne(
		&survey.Input{
			Message: "Where is the script for this task?",
			Default: entrypoint,
			Suggest: func(toComplete string) []string {
				files, _ := filepath.Glob(toComplete + "*")
				return files
			},
		},
		&entrypoint,
		survey.WithValidator(func(val interface{}) error {
			if len(exts) == 0 {
				return nil
			}
			if str, ok := val.(string); ok {
				for _, ext := range exts {
					if strings.HasSuffix(str, ext) {
						return nil
					}
				}
				return errors.Errorf("File must have a valid extension: %s", exts)
			}
			return errors.New("expected string")
		}),
	); err != nil {
		return "", err
	}

	// Ensure that the selected file has the correct extension for an inline entrypoint.
	if cfg.inline {
		entrypoint = modifyEntrypointForInline(kind, entrypoint)
	}

	directory := filepath.Dir(entrypoint)
	if err := createFolder(directory); err != nil {
		return "", errors.Wrapf(err, "Error creating directory for script.")
	}

	return entrypoint, nil
}

func getEntrypointFile(slug string, kind build.TaskKind, cfg config) (string, error) {
	exts := runtime.SuggestExts(kind)
	entrypoint := slug
	if kind == build.TaskKindNode && len(exts) > 1 {
		// Special case JavaScript tasks and make their extensions '.ts'
		entrypoint += ".ts"
	} else {
		entrypoint += exts[0]
	}

	if cwdIsHome, err := cwdIsHome(); err != nil {
		return "", err
	} else if cwdIsHome {
		// Suggest a subdirectory to avoid putting a file directly into home directory.
		entrypoint = filepath.Join("airplane", entrypoint)
	}

	if cfg.inline {
		entrypoint = modifyEntrypointForInline(kind, entrypoint)
	}
	return entrypoint, nil
}

func promptForNewDefinition(defaultFilename, entrypoint string) (string, error) {
	entrypointDir := filepath.Dir(entrypoint)
	defaultFilename = filepath.Join(entrypointDir, defaultFilename)

	var filename string
	if err := survey.AskOne(
		&survey.Input{
			Message: "Where should the definition file be created?",
			Default: defaultFilename,
			Suggest: func(toComplete string) []string {
				files, _ := filepath.Glob(toComplete + "*")
				return files
			},
		},
		&filename,
		survey.WithValidator(func(val interface{}) error {
			if str, ok := val.(string); ok {
				if definitions.IsTaskDef(str) {
					return nil
				}
				return errors.Errorf("Definition file must have extension .task.yaml or .task.json")
			}
			return errors.New("expected string")
		}),
	); err != nil {
		return "", err
	}

	directory := filepath.Dir(filename)
	if err := createFolder(directory); err != nil {
		return "", errors.Wrapf(err, "Error creating directory for definition file.")
	}
	return filename, nil
}

func createFolder(directory string) error {
	if _, err := os.Stat(directory); err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		// Directory doesn't exist, make it.
		if err := os.MkdirAll(directory, 0755); err != nil {
			return err
		}
	}
	return nil
}

var allKindsByName = map[string]build.TaskKind{
	"Docker":     build.TaskKindImage,
	"JavaScript": build.TaskKindNode,
	"Python":     build.TaskKindPython,
	"Shell":      build.TaskKindShell,
	"SQL":        build.TaskKindSQL,
	"REST":       build.TaskKindREST,
	"GraphQL":    build.TaskKindBuiltin,
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

func promptForNewTask(file string, info *newTaskInfo, inline bool, workflow bool) error {
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
	if err := survey.AskOne(
		&survey.Input{
			Message: "What should this task be called?",
			Default: base,
		},
		&info.name,
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
	if err := survey.AskOne(
		&survey.Select{
			Message: "What kind of task should this be?",
			Options: kindNames,
			Default: defaultName,
		},
		&selectedKindName,
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

var inlineSupportedKinds = []build.TaskKind{build.TaskKindNode, build.TaskKindPython}

func isInlineSupportedKind(kind build.TaskKind) bool {
	return slices.Contains(inlineSupportedKinds, kind)
}

func cwdIsHome() (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	return cwd == home, nil
}

func createEntrypoint(r runtime.Interface, entrypoint string, task *libapi.Task) error {
	code, fileMode, err := r.Generate(apiTaskToRuntimeTask(task))
	if err != nil {
		return err
	}

	return writeEntrypoint(entrypoint, code, fileMode)
}

func createInlineEntrypoint(r runtime.Interface, entrypoint string, def *definitions.Definition_0_3) error {
	code, fileMode, err := r.GenerateInline(def)
	if err != nil {
		return err
	}

	return writeEntrypoint(entrypoint, code, fileMode)
}

func modifyEntrypointForInline(kind build.TaskKind, entrypoint string) string {
	if !isInlineSupportedKind(kind) {
		return entrypoint
	}

	ext := filepath.Ext(entrypoint)
	entrypointWithoutExt := strings.TrimSuffix(entrypoint, ext)

	if kind == build.TaskKindNode && !strings.HasSuffix(entrypointWithoutExt, ".airplane") {
		return fmt.Sprintf("%s.airplane%s", entrypointWithoutExt, ext)
	}
	if kind == build.TaskKindPython && !strings.HasSuffix(entrypointWithoutExt, "_airplane") {
		return fmt.Sprintf("%s_airplane%s", entrypointWithoutExt, ext)
	}
	return entrypoint
}

func writeEntrypoint(path string, b []byte, fileMode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(path, b, fileMode); err != nil {
		return err
	}

	return nil
}

func apiTaskToRuntimeTask(task *libapi.Task) *runtime.Task {
	if task == nil {
		return nil
	}
	t := &runtime.Task{
		URL: task.URL,
	}
	for _, p := range task.Parameters {
		t.Parameters = append(t.Parameters, runtime.Parameter{
			Name: p.Name,
			Slug: p.Slug,
			Type: runtime.Type(p.Type),
		})
	}
	return t
}

func runKindSpecificInstallation(ctx context.Context, cfg config, kind build.TaskKind, def definitions.Definition_0_3) error {
	switch kind {
	case build.TaskKindNode:
		entrypoint, err := def.GetAbsoluteEntrypoint()
		if err != nil {
			return err
		}
		packageJSONDir, err := node.CreatePackageJSON(filepath.Dir(entrypoint), node.PackageJSONOptions{
			Dependencies: node.NodeDependencies{
				Dependencies:    []string{"airplane"},
				DevDependencies: []string{"@types/node"},
			},
		})
		if err != nil {
			return err
		}

		if cfg.root.Flagger.Bool(ctx, logger.NewStdErrLogger(logger.StdErrLoggerOpts{}), flagsiface.AirplaneConfg) {
			_, nodeVersion, buildBase, err := def.GetBuildType()
			if err != nil {
				return err
			}
			if nodeVersion == "" {
				nodeVersion = build.DefaultNodeVersion
			}

			if err := createOrUpdateAirplaneConfig(packageJSONDir, deployconfig.AirplaneConfig{
				Javascript: deployconfig.JavaScriptConfig{
					NodeVersion: string(nodeVersion),
					Base:        string(buildBase),
				},
			}); err != nil {
				return err
			}
		}
		// Create/update tsconfig in the same directory as the package.json file
		if err := node.CreateTaskTSConfig(packageJSONDir); err != nil {
			return err
		}
		return nil
	case build.TaskKindPython:
		if cfg.root.Flagger.Bool(ctx, logger.NewStdErrLogger(logger.StdErrLoggerOpts{}), flagsiface.AirplaneConfg) {
			entrypoint, err := def.GetAbsoluteEntrypoint()
			if err != nil {
				return err
			}
			runtime := python.Runtime{}
			root, err := runtime.Root(entrypoint)
			if err != nil {
				return err
			}

			_, pythonVersion, buildBase, err := def.GetBuildType()
			if err != nil {
				return err
			}
			if pythonVersion == "" {
				pythonVersion = build.DefaultPythonVersion
			}
			if err := createOrUpdateAirplaneConfig(root, deployconfig.AirplaneConfig{
				Python: deployconfig.PythonConfig{
					Version: string(pythonVersion),
					Base:    string(buildBase),
				},
			}); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

// createOrUpdateAirplaneConfig creates or updates an existing airplane.yaml.
func createOrUpdateAirplaneConfig(root string, cfg deployconfig.AirplaneConfig) error {
	var existingConfig deployconfig.AirplaneConfig
	var err error
	existingConfigFilePath := filepath.Join(root, deployconfig.FileName)
	hasExistingConfigFile := fsx.Exists(existingConfigFilePath)
	if hasExistingConfigFile {
		existingConfig, err = deployconfig.NewAirplaneConfigFromFile(existingConfigFilePath)
		if err != nil {
			return err
		}
	}

	f, err := os.OpenFile(existingConfigFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	return writeNewAirplaneConfig(f, getNewAirplaneConfigOptions{
		cfg:               cfg,
		existingConfig:    existingConfig,
		hasExistingConfig: hasExistingConfigFile,
	})
}

type getNewAirplaneConfigOptions struct {
	cfg               deployconfig.AirplaneConfig
	existingConfig    deployconfig.AirplaneConfig
	hasExistingConfig bool
}

// writeNewAirplaneConfig is a helper called from createOrUpdateAirplaneConfig.
func writeNewAirplaneConfig(writer io.Writer, opts getNewAirplaneConfigOptions) error {
	if opts.hasExistingConfig {
		existingBuf, _ := yaml.Marshal(&opts.existingConfig)
		if string(existingBuf) != "{}\n" {
			// The existing config is not empty. Don't update it, but possibly log
			// some helpful hints.
			if opts.cfg.Javascript.NodeVersion != "" && opts.existingConfig.Javascript.NodeVersion == "" {
				logger.Warning("We recommend specifying a javascript.nodeVersion in your %s.", deployconfig.FileName)
			}
			if opts.cfg.Python.Version != "" && opts.existingConfig.Python.Version == "" {
				logger.Warning("We recommend specifying a python.version in your %s.", deployconfig.FileName)
			}
			return nil
		}
	}

	e := yaml.NewEncoder(writer)
	defer e.Close()
	e.SetIndent(2)
	if err := e.Encode(&opts.cfg); err != nil {
		return errors.Wrapf(err, "writing %s", deployconfig.FileName)
	}

	if opts.hasExistingConfig {
		logger.Step("Updated %s", deployconfig.FileName)
	} else {
		logger.Step("Created %s", deployconfig.FileName)
	}
	return nil
}
