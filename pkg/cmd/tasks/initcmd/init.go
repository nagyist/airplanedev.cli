// initcmd defines the implementation of the `airplane tasks init` command.
//
// Even though the command is called "init", we can't name the package "init"
// since that conflicts with the Go init function.
package initcmd

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/cmd/auth/login"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/runtime"
	_ "github.com/airplanedev/lib/pkg/runtime/javascript"
	_ "github.com/airplanedev/lib/pkg/runtime/python"
	_ "github.com/airplanedev/lib/pkg/runtime/shell"
	_ "github.com/airplanedev/lib/pkg/runtime/typescript"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	client *api.Client
	file   string
	slug   string

	codeOnly  bool
	defFormat string
	assumeYes bool
	assumeNo  bool
	envSlug   string

	newTaskInfo newTaskInfo
}

type newTaskInfo struct {
	name       string
	kind       build.TaskKind
	entrypoint string
}

func New(c *cli.Config) *cobra.Command {
	var cfg = config{client: c.Client}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a task definition",
		Example: heredoc.Doc(`
			$ airplane tasks init --slug task-slug
			$ airplane tasks init --slug task-slug ./my/task.js
			$ airplane tasks init --slug task-slug ./my/task.ts
		`),
		Args: cobra.MaximumNArgs(1),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				cfg.file = args[0]
			}
			return run(cmd.Root().Context(), cfg)
		},
	}

	cmd.Flags().StringVar(&cfg.slug, "slug", "", "Slug of an existing task to generate from.")

	cmd.Flags().StringVar(&cfg.slug, "from", "", "Slug of an existing task to initialize.")
	cmd.Flags().BoolVar(&cfg.codeOnly, "code-only", false, "True to skip creating a task definition file; only generates an entrypoint file.")
	cmd.Flags().StringVar(&cfg.defFormat, "def-format", "", `One of "json" or "yaml". Defaults to "yaml".`)
	cmd.Flags().BoolVarP(&cfg.assumeYes, "yes", "y", false, "True to specify automatic yes to prompts.")
	cmd.Flags().BoolVarP(&cfg.assumeNo, "no", "n", false, "True to specify automatic no to prompts.")

	if err := cmd.Flags().MarkHidden("slug"); err != nil {
		logger.Debug("error: %s", err)
	}

	// Unhide this flag once we release environments.
	cmd.Flags().StringVar(&cfg.envSlug, "env", "", "The slug of the environment to query. Defaults to your team's default environment.")
	if err := cmd.Flags().MarkHidden("env"); err != nil {
		logger.Debug("unable to hide --env: %s", err)
	}

	return cmd
}

func run(ctx context.Context, cfg config) error {
	// Check for mutually exclusive flags.
	if cfg.codeOnly && cfg.defFormat != "" {
		return errors.New("Cannot specify both --code-only and --def-format")
	}
	if cfg.assumeYes && cfg.assumeNo {
		return errors.New("Cannot specify both --yes and --no")
	}
	if cfg.codeOnly && cfg.slug == "" {
		return errors.New("Required flag(s) \"slug\" not set")
	}

	// Extrapolate defFormat from the specified file, if it's a definition file.
	defFormat := definitions.GetTaskDefFormat(cfg.file)
	if defFormat != definitions.TaskDefFormatUnknown {
		cfg.defFormat = string(defFormat)
	}

	if cfg.slug == "" {
		// Prompt for new task information.
		if err := promptForNewTask(cfg.file, &cfg.newTaskInfo); err != nil {
			return err
		}
	}

	if cfg.codeOnly {
		return initCodeOnly(ctx, cfg)
	}

	return initWithTaskDef(ctx, cfg)
}

func initWithTaskDef(ctx context.Context, cfg config) error {
	client := cfg.client

	// Check for a valid defFormat, add in a default if necessary.
	if cfg.defFormat == "" {
		cfg.defFormat = "yaml"
	}
	if cfg.defFormat != "yaml" && cfg.defFormat != "json" {
		return errors.Errorf("Invalid \"def-format\" specified: %s", cfg.defFormat)
	}

	var def definitions.Definition_0_3
	if cfg.slug != "" {
		task, err := client.GetTask(ctx, libapi.GetTaskRequest{
			Slug:    cfg.slug,
			EnvSlug: cfg.envSlug,
		})
		if err != nil {
			return err
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
		def, err = definitions.NewDefinition_0_3(cfg.newTaskInfo.name,
			utils.MakeSlug(cfg.newTaskInfo.name), cfg.newTaskInfo.kind, cfg.newTaskInfo.entrypoint)
		if err != nil {
			return err
		}
	}

	entrypoint, err := def.Entrypoint()
	if err == definitions.ErrNoEntrypoint {
		// no-op
	} else if err != nil {
		return err
	} else {
		kind, err := def.Kind()
		if err != nil {
			return err
		}

		if entrypoint == "" {
			entrypoint, err = promptForNewFileName(def.GetSlug(), kind)
			if err != nil {
				return err
			}
			if err := def.SetEntrypoint(entrypoint); err != nil {
				return err
			}
		}

		r, err := runtime.Lookup(entrypoint, kind)
		if err != nil {
			return errors.Wrapf(err, "unable to init %q - check that your CLI is up to date", entrypoint)
		}

		if kind == build.TaskKindSQL {
			doCreateEntrypoint := true
			if fsx.Exists(entrypoint) {
				question := fmt.Sprintf("Would you like to overwrite %s?", entrypoint)
				if ok, err := utils.ConfirmWithAssumptions(question, cfg.assumeYes, cfg.assumeNo); err != nil {
					return err
				} else if !ok {
					doCreateEntrypoint = false
				}
			}

			if doCreateEntrypoint {
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
			}
		} else {
			// Create entrypoint, without comment link, if it doesn't exist.
			if !fsx.Exists(entrypoint) {
				if err := createEntrypoint(r, entrypoint, nil); err != nil {
					return errors.Wrapf(err, "unable to create entrypoint")
				}
				logger.Step("Created %s", entrypoint)
			} else {
				logger.Step("%s already exists", entrypoint)
			}
		}
	}

	// Create task defn file.
	defFn := cfg.file
	if !definitions.IsTaskDef(defFn) {
		defFn = fmt.Sprintf("%s.task.%s", def.Slug, cfg.defFormat)
	}
	if fsx.Exists(defFn) {
		// If it exists, check for existence of this file before overwriting it.
		question := fmt.Sprintf("Would you like to overwrite %s?", defFn)
		if ok, err := utils.ConfirmWithAssumptions(question, cfg.assumeYes, cfg.assumeNo); err != nil {
			return err
		} else if !ok {
			// User answered "no", so bail here.
			return nil
		}
	}

	buf, err := def.Marshal(definitions.TaskDefFormat(cfg.defFormat))
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(defFn, buf, 0644); err != nil {
		return err
	}
	logger.Step("Created %s", defFn)
	suggestNextSteps(defFn)
	return nil
}

func initCodeOnly(ctx context.Context, cfg config) error {
	client := cfg.client

	task, err := client.GetTask(ctx, libapi.GetTaskRequest{
		Slug:    cfg.slug,
		EnvSlug: cfg.envSlug,
	})
	if err != nil {
		return err
	}

	if cfg.file == "" {
		cfg.file, err = promptForNewFileName(task.Slug, task.Kind)
		if err != nil {
			return err
		}
	}

	r, err := runtime.Lookup(cfg.file, task.Kind)
	if err != nil {
		return errors.Wrapf(err, "unable to init %q - check that your CLI is up to date", cfg.file)
	}

	if fsx.Exists(cfg.file) {
		if slug, ok := runtime.Slug(cfg.file); ok && slug == task.Slug {
			logger.Step("%s is already linked to %s", cfg.file, cfg.slug)
			suggestNextSteps(cfg.file)
			return nil
		}

		patch, err := patch(cfg.slug, cfg.file)
		if err != nil {
			return err
		}

		if !patch {
			logger.Log("You canceled linking %s to %s", cfg.file, cfg.slug)
			return nil
		}

		buf, err := ioutil.ReadFile(cfg.file)
		if err != nil {
			return err
		}
		code := prependComment(buf, runtime.Comment(r, task.URL))
		// Note: 0644 is ignored because file already exists. Uses a reasonable default just in case.
		if err := ioutil.WriteFile(cfg.file, code, 0644); err != nil {
			return err
		}
		logger.Step("Linked %s to %s", cfg.file, cfg.slug)

		suggestNextSteps(cfg.file)
		return nil
	}

	if err := createEntrypoint(r, cfg.file, &task); err != nil {
		return err
	}
	logger.Step("Created %s", cfg.file)
	suggestNextSteps(cfg.file)
	return nil
}

// prependComment handles writing the linking comment to source code, accounting for shebangs
// (which have to appear first in the file).
func prependComment(source []byte, comment string) []byte {
	var buf bytes.Buffer

	// Regardless of task type, look for a shebang and put comment after it if detected.
	hasShebang := len(source) >= 2 && source[0] == '#' && source[1] == '!'
	appendAfterFirstNewline := hasShebang

	appendComment := func() {
		buf.WriteString(comment)
		buf.WriteRune('\n')
		buf.WriteRune('\n')
	}

	prepended := false
	if !appendAfterFirstNewline {
		appendComment()
		prepended = true
	}
	for _, char := range string(source) {
		buf.WriteRune(char)
		if char == '\n' && appendAfterFirstNewline && !prepended {
			appendComment()
			prepended = true
		}
	}
	return buf.Bytes()
}

func suggestNextSteps(file string) {
	logger.Suggest(
		"⚡ To execute the task locally:",
		"airplane dev %s",
		file,
	)
	logger.Suggest(
		"🛫 To deploy your task to Airplane:",
		"airplane deploy %s",
		file,
	)
}

// Patch asks the user if he would like to patch a file
// and add the airplane special comment.
func patch(slug, file string) (ok bool, err error) {
	err = survey.AskOne(
		&survey.Confirm{
			Message: fmt.Sprintf("Would you like to link %s to %s?", file, slug),
			Help:    "Linking this file will add a special airplane comment.",
			Default: true,
		},
		&ok,
	)
	return
}

func promptForNewFileName(slug string, kind build.TaskKind) (string, error) {
	fileName := slug + runtime.SuggestExt(kind)

	if cwdIsHome, err := cwdIsHome(); err != nil {
		return "", err
	} else if cwdIsHome {
		// Suggest a subdirectory to avoid putting a file directly into home directory.
		fileName = filepath.Join("airplane", fileName)
	}

	if err := survey.AskOne(
		&survey.Input{
			Message: "Where should the script be created?",
			Default: fileName,
		},
		&fileName,
	); err != nil {
		return "", err
	}
	return fileName, nil
}

var namesByKind = map[build.TaskKind]string{
	build.TaskKindImage:  "Docker",
	build.TaskKindNode:   "Node",
	build.TaskKindPython: "Python",
	build.TaskKindShell:  "Shell",

	build.TaskKindSQL:  "SQL",
	build.TaskKindREST: "REST",
}

var orderedKindNames = []string{
	"SQL",
	"REST",
	"Node",
	"Python",
	"Shell",
	"Docker",
}

func promptForNewTask(file string, info *newTaskInfo) error {
	defFormat := definitions.GetTaskDefFormat(file)
	ext := filepath.Ext(file)
	base := strings.TrimSuffix(file, ext)
	if defFormat != definitions.TaskDefFormatUnknown {
		// Trim off the .task part, too
		base = strings.TrimSuffix(base, ".task")
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

	// Ask for a kind.
	var defaultKind interface{}
	guessKind, err := runtime.SuggestKind(ext)
	if err != nil {
		defaultKind = orderedKindNames[0]
	} else {
		defaultKind = namesByKind[guessKind]
	}

	var selectedKindName string
	if err := survey.AskOne(
		&survey.Select{
			Message: "What kind of task should this be?",
			Options: orderedKindNames,
			Default: defaultKind,
		},
		&selectedKindName,
	); err != nil {
		return err
	}
	for kind, name := range namesByKind {
		if name == selectedKindName {
			info.kind = kind
			break
		}
	}
	if info.kind == "" {
		return errors.Errorf("Unknown kind selected: %s", selectedKindName)
	}

	// Ask for an entrypoint, maybe.
	if info.kind != build.TaskKindREST && info.kind != build.TaskKindImage {
		if file != "" && !definitions.IsTaskDef(file) {
			info.entrypoint = file
		} else {
			fileName := utils.MakeSlug(info.name) + runtime.SuggestExt(info.kind)
			if cwdIsHome, err := cwdIsHome(); err != nil {
				return err
			} else if cwdIsHome {
				// Suggest a subdirectory to avoid putting a file directly into home directory.
				fileName = filepath.Join("airplane", fileName)
			}

			if err := survey.AskOne(
				&survey.Input{
					Message: "Where should the script be created?",
					Default: fileName,
				},
				&info.entrypoint,
			); err != nil {
				return err
			}
		}
	}

	return nil
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

func writeEntrypoint(path string, b []byte, fileMode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := ioutil.WriteFile(path, b, fileMode); err != nil {
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
