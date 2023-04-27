package initcmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	libapi "github.com/airplanedev/cli/pkg/api"
	api "github.com/airplanedev/cli/pkg/api/cliapi"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	deployconfig "github.com/airplanedev/cli/pkg/deploy/config"
	"github.com/airplanedev/cli/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/node"
	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/airplanedev/cli/pkg/python"
	"github.com/airplanedev/cli/pkg/runtime"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/fsx"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

type InitTaskRequest struct {
	Client           api.APIClient
	Prompter         prompts.Prompter
	DryRun           bool
	WorkingDirectory string

	File     string
	FromTask string

	AssumeYes bool
	AssumeNo  bool
	EnvSlug   string

	Inline   bool
	Workflow bool

	TaskName        string
	TaskSlug        string
	TaskKind        buildtypes.TaskKind
	TaskKindName    string
	TaskEntrypoint  string
	TaskDescription string
	TaskNodeFlavor  NodeFlavor

	// ease of testing
	suffixCharset string
}

type NodeFlavor string

const (
	NodeFlavorTypeScript = "TypeScript"
	NodeFlavorJavaScript = "JavaScript"
)

func InitTask(ctx context.Context, req InitTaskRequest) (InitResponse, error) {
	if req.suffixCharset == "" {
		req.suffixCharset = utils.CharsetLowercaseNumeric
	}
	client := req.Client

	if req.WorkingDirectory == "" {
		wd, err := filepath.Abs(".")
		if err != nil {
			return InitResponse{}, err
		}
		req.WorkingDirectory = wd
	} else {
		wd, err := filepath.Abs(req.WorkingDirectory)
		if err != nil {
			return InitResponse{}, err
		}
		req.WorkingDirectory = wd
	}
	ret, err := newInitResponse(req.WorkingDirectory)
	if err != nil {
		return InitResponse{}, err
	}

	var def definitions.Definition
	if req.FromTask != "" {
		task, err := client.GetTask(ctx, libapi.GetTaskRequest{
			Slug:    req.FromTask,
			EnvSlug: req.EnvSlug,
		})
		if err != nil {
			return InitResponse{}, err
		}

		if task.Runtime == buildtypes.TaskRuntimeWorkflow {
			req.Workflow = true
			req.Inline = true
		}

		resp, err := client.ListResourceMetadata(ctx)
		if err != nil {
			return InitResponse{}, err
		}

		def, err = definitions.NewDefinitionFromTask(task, resp.Resources)
		if err != nil {
			return InitResponse{}, err
		}
	} else {
		if req.TaskName == "" {
			return InitResponse{}, errors.New("missing new task name")
		}
		if req.TaskKind == "" {
			return InitResponse{}, errors.New("missing new task kind")
		}

		var err error
		slug := req.TaskSlug
		if slug == "" {
			slug = utils.MakeSlug(req.TaskName)
		}
		if req.TaskKind == buildtypes.TaskKindBuiltin {
			switch req.TaskKindName {
			case "GraphQL":
				def = definitions.NewBuiltinDefinition(
					req.TaskName,
					slug,
					&definitions.GraphQLDefinition{
						Operation: `query GetUser($id: ID) {
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
				return InitResponse{}, errors.Errorf("don't know how to initialize task kind=builtin name=%s", req.TaskKindName)
			}
		} else {
			def, err = definitions.NewDefinition(
				req.TaskName,
				slug,
				req.TaskKind,
				req.TaskEntrypoint,
			)
		}
		if err != nil {
			return InitResponse{}, err
		}
	}
	ret.NewTaskDefinition = &def

	kind, err := def.Kind()
	if err != nil {
		return InitResponse{}, err
	}
	if !isInlineSupportedKind(kind) {
		req.Inline = false
	}
	if req.Workflow {
		def.Runtime = buildtypes.TaskRuntimeWorkflow
	}
	def.Description = req.TaskDescription

	localExecutionSupported := false
	if entrypoint, err := def.Entrypoint(); err == definitions.ErrNoEntrypoint {
		// no-op
	} else if err != nil {
		return InitResponse{}, err
	} else {
		if req.File != "" && !definitions.IsTaskDef(req.File) {
			entrypoint = req.File
		}

		if filepath.Ext(entrypoint) == "tsx" || filepath.Ext(entrypoint) == "jsx" {
			errorMsg := "You are trying to initialize a task in a React file. Use `airplane views init` if you'd like to initialize a view."
			if req.Prompter == nil {
				return InitResponse{}, errors.New(errorMsg)
			} else {
				logger.Log(errorMsg)
				if ok, err := req.Prompter.ConfirmWithAssumptions("Are you sure you'd like to continue?", req.AssumeYes, req.AssumeNo); err != nil {
					return InitResponse{}, err
				} else if !ok {
					logger.Log("Exiting flow")
					return InitResponse{}, nil
				}
			}
		}

		if req.AssumeYes && req.File != "" {
			entrypoint = req.File
		} else {
			entrypoint, err = promptForEntrypoint(promptForEntrypointRequest{
				prompter:          req.Prompter,
				slug:              def.GetSlug(),
				kind:              kind,
				defaultEntrypoint: entrypoint,
				inline:            req.Inline,
				nodeFlavor:        req.TaskNodeFlavor,
			})
			if err != nil {
				return InitResponse{}, err
			}
		}

		var absEntrypoint string
		var shouldPrintEntrypointToStdOut bool

		absEntrypoint = entrypoint
		if !filepath.IsAbs(absEntrypoint) {
			abs, err := filepath.Abs(filepath.Join(req.WorkingDirectory, entrypoint))
			if err != nil {
				return InitResponse{}, errors.Wrap(err, "determining absolute entrypoint")
			}
			absEntrypoint = abs
		}

		if req.Prompter == nil {
			// Add a suffix to it.
			absEntrypoint, err = trySuffix(absEntrypoint, addEntrypointSuffixFunc(req), 3, req.suffixCharset)
			if err != nil {
				return InitResponse{}, errors.Wrap(err, "finding entrypoint")
			}
			entrypoint = filepath.Base(absEntrypoint)
		} else {
			for {
				if fsx.Exists(absEntrypoint) {
					shouldOverwrite, shouldPrintToStdOut, err := shouldOverwriteTaskEntrypoint(req, absEntrypoint, kind)
					if err != nil {
						return InitResponse{}, err
					}
					shouldPrintEntrypointToStdOut = shouldPrintToStdOut
					if shouldOverwrite || shouldPrintEntrypointToStdOut {
						break
					}
				} else {
					break
				}

				entrypoint, err = promptForEntrypoint(promptForEntrypointRequest{
					prompter:          req.Prompter,
					slug:              def.GetSlug(),
					kind:              kind,
					defaultEntrypoint: entrypoint,
					inline:            req.Inline,
					nodeFlavor:        req.TaskNodeFlavor,
				})
				if err != nil {
					return InitResponse{}, err
				}

				absEntrypoint = entrypoint
				if !filepath.IsAbs(absEntrypoint) {
					abs, err := filepath.Abs(filepath.Join(req.WorkingDirectory, entrypoint))
					if err != nil {
						return InitResponse{}, errors.Wrap(err, "determining absolute entrypoint")
					}
					absEntrypoint = abs
				}
			}
		}

		if err := def.SetEntrypoint(entrypoint); err != nil {
			return InitResponse{}, err
		}
		if err := def.SetAbsoluteEntrypoint(absEntrypoint); err != nil {
			return InitResponse{}, err
		}

		r, err := runtime.Lookup(entrypoint, kind)
		if err != nil {
			return InitResponse{}, errors.Wrapf(err, "unable to init %q - check that your CLI is up to date", entrypoint)
		}
		localExecutionSupported = r.SupportsLocalExecution()

		if !req.DryRun {
			if kind == buildtypes.TaskKindSQL {
				query, err := def.SQL.GetQuery()
				if err != nil {
					// Create a generic entrypoint.
					if err := createTaskEntrypoint(r, absEntrypoint, nil); err != nil {
						return InitResponse{}, errors.Wrapf(err, "unable to create entrypoint")
					}
				} else {
					// Write the query to the entrypoint.
					if err := writeTaskEntrypoint(absEntrypoint, []byte(query), 0644); err != nil {
						return InitResponse{}, errors.Wrapf(err, "unable to create entrypoint")
					}
				}
				logger.Step("Created %s", entrypoint)
			} else if req.Inline {
				if err := createInlineEntrypoint(r, absEntrypoint, &def, shouldPrintEntrypointToStdOut); err != nil {
					return InitResponse{}, errors.Wrapf(err, "unable to create entrypoint")
				}
				if shouldPrintEntrypointToStdOut {
					logger.Step("Printed task to stdout. Copy task configuration to %s.", entrypoint)
				} else {
					logger.Step("Created %s", entrypoint)
				}
			} else {
				// Create entrypoint, without comment link, if it doesn't exist.
				if !fsx.Exists(absEntrypoint) {
					if err := createTaskEntrypoint(r, absEntrypoint, nil); err != nil {
						return InitResponse{}, errors.Wrapf(err, "unable to create entrypoint")
					}
					logger.Step("Created %s", entrypoint)
				}
			}
		}
		ret.AddCreatedFile(absEntrypoint)
	}

	var resp *writeDefnFileResponse
	if !req.Inline {
		resp, err = writeTaskDefnFile(&def, req)
		if err != nil {
			return InitResponse{}, err
		}
		if resp == nil {
			return ret, nil
		}
		ret.AddCreatedFile(resp.DefnFile)
		def.SetDefnFilePath(resp.DefnFile)
	} else {
		entrypoint, _ := def.Entrypoint()
		resp = &writeDefnFileResponse{
			DefnFile:       entrypoint,
			EntrypointFile: entrypoint,
		}
	}

	if err := runKindSpecificInstallation(ctx, runKindSpecificInstallationRequest{
		Prompter:     req.Prompter,
		DryRun:       req.DryRun,
		Inline:       req.Inline,
		Kind:         kind,
		Def:          def,
		InitResponse: &ret,
	}); err != nil {
		return InitResponse{}, err
	}

	if req.DryRun {
		logger.Log("Running with --dry-run. This command would have created or updated the following files:\n%s", ret.String())
	}

	suggestNextTaskSteps(suggestNextTaskStepsRequest{
		defnFile:           resp.DefnFile,
		entrypoint:         resp.EntrypointFile,
		showLocalExecution: localExecutionSupported,
		kind:               kind,
		isNew:              req.FromTask == "",
	})

	return ret, nil
}

func shouldOverwriteTaskEntrypoint(req InitTaskRequest, entrypoint string, kind buildtypes.TaskKind) (shouldOverwrite, shouldPrintToStdOut bool, err error) {
	if req.Inline {
		overwriteOption := fmt.Sprintf("Overwrite %s.", entrypoint)
		if req.FromTask != "" {
			overwriteOption = fmt.Sprintf("Overwrite %s with configuration from %s.", entrypoint, req.FromTask)
		}
		shouldPrintToStdOutOption := "Print to stdout instead of writing to a file."
		if req.FromTask != "" {
			shouldPrintToStdOutOption = fmt.Sprintf("Print %s to stdout instead of writing to a file.", req.FromTask)
		}
		chooseDifferentFileOption := "Write the configuration to a different file."
		if req.FromTask != "" {
			chooseDifferentFileOption = fmt.Sprintf("Write the configuration for %s to a different file.", req.FromTask)
		}
		options := []string{
			overwriteOption,
			shouldPrintToStdOutOption,
			chooseDifferentFileOption,
		}
		var selectedOption string
		if err := req.Prompter.Input(
			fmt.Sprintf("%s already exists. What would you like to do?", entrypoint),
			&selectedOption,
			prompts.WithSelectOptions(options),
			prompts.WithDefault(options[0]),
		); err != nil {
			return false, false, err
		}
		if selectedOption == shouldPrintToStdOutOption {
			return false, true, nil
		}
		if selectedOption == overwriteOption {
			return true, false, nil
		}
	} else {
		question := fmt.Sprintf("Are you sure you want to link %s? You should only link existing Airplane scripts.", entrypoint)
		if kind == buildtypes.TaskKindSQL {
			question = fmt.Sprintf("Would you like to overwrite %s?", entrypoint)
		}
		if ok, err := req.Prompter.ConfirmWithAssumptions(question, req.AssumeYes, req.AssumeNo); err != nil {
			return false, false, err
		} else if ok {
			return true, false, nil
		}
	}
	return false, false, nil
}

type writeDefnFileResponse struct {
	DefnFile       string
	EntrypointFile string
}

func writeTaskDefnFile(def *definitions.Definition, req InitTaskRequest) (*writeDefnFileResponse, error) {
	// Create task defn file.
	defnFilename := req.File
	if !definitions.IsTaskDef(req.File) {
		defaultDefnFn := fmt.Sprintf("%s.task.yaml", def.Slug)
		entrypoint, _ := def.Entrypoint()
		if req.Prompter != nil {
			fn, err := promptForNewDefinition(defaultDefnFn, entrypoint, req.Prompter)
			if err != nil {
				return nil, err
			}
			defnFilename = fn
		} else {
			defnFilename = defaultDefnFn
		}
	}
	defnFilename = filepath.Join(req.WorkingDirectory, defnFilename)
	if fsx.Exists(defnFilename) {
		// If it exists, check for existence of this file before overwriting it.
		if req.Prompter != nil {
			question := fmt.Sprintf("Would you like to overwrite %s?", defnFilename)
			if ok, err := req.Prompter.ConfirmWithAssumptions(question, req.AssumeYes, req.AssumeNo); err != nil {
				return nil, err
			} else if !ok {
				// User answered "no", so bail here.
				return nil, nil
			}
		} else {
			var err error
			defnFilename, err = trySuffix(defnFilename, nil, 3, req.suffixCharset)
			if err != nil {
				return nil, err
			}
		}
	}

	// Adjust entrypoint to be relative to the task defn.
	entrypoint, err := def.Entrypoint()
	if err == definitions.ErrNoEntrypoint {
		// no-op
	} else if err != nil {
		return nil, err
	} else {
		absEntrypoint, err := def.GetAbsoluteEntrypoint()
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

	if !req.DryRun {
		if err := os.WriteFile(defnFilename, buf, 0644); err != nil {
			return nil, err
		}
	}
	logger.Step("Created %s", defnFilename)
	return &writeDefnFileResponse{
		DefnFile:       defnFilename,
		EntrypointFile: entrypoint,
	}, nil
}

type promptForEntrypointRequest struct {
	prompter          prompts.Prompter
	slug              string
	kind              buildtypes.TaskKind
	defaultEntrypoint string
	inline            bool
	nodeFlavor        NodeFlavor
}

func promptForEntrypoint(req promptForEntrypointRequest) (string, error) {
	entrypoint := req.defaultEntrypoint
	if entrypoint == "" {
		var err error
		entrypoint, err = getEntrypointFile(req)
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
			entrypointFile, err := getEntrypointFile(req)
			if err != nil {
				return "", err
			}
			entrypoint = filepath.Join(entrypoint, entrypointFile)
		}
	}
	// Ensure that the file has the correct extension for an inline entrypoint.
	if req.inline {
		entrypoint = modifyEntrypointForInline(req.kind, entrypoint)
	}

	if req.prompter != nil {
		exts := runtime.SuggestExts(req.kind)
		if err := req.prompter.Input(
			"Where is the script for this task?",
			&entrypoint,
			prompts.WithDefault(entrypoint),
			prompts.WithSuggest(func(toComplete string) []string {
				files, _ := filepath.Glob(toComplete + "*")
				return files
			}),
			prompts.WithValidator(func(val interface{}) error {
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
	}

	// Ensure that the selected file has the correct extension for an inline entrypoint.
	if req.inline {
		entrypoint = modifyEntrypointForInline(req.kind, entrypoint)
	}

	directory := filepath.Dir(entrypoint)
	if err := createFolder(directory); err != nil {
		return "", errors.Wrapf(err, "Error creating directory for script.")
	}

	return entrypoint, nil
}

func getEntrypointFile(req promptForEntrypointRequest) (string, error) {
	exts := runtime.SuggestExts(req.kind)
	entrypoint := req.slug
	if req.kind == buildtypes.TaskKindNode && len(exts) > 1 {
		if req.nodeFlavor == NodeFlavorTypeScript {
			entrypoint += ".ts"
		} else if req.nodeFlavor == NodeFlavorJavaScript {
			entrypoint += ".js"
		} else {
			// Special case JavaScript tasks and make their extensions '.ts'
			entrypoint += ".ts"
		}
	} else {
		entrypoint += exts[0]
	}

	if cwdIsHome, err := cwdIsHome(); err != nil {
		return "", err
	} else if cwdIsHome {
		// Suggest a subdirectory to avoid putting a file directly into home directory.
		entrypoint = filepath.Join("airplane", entrypoint)
	}

	if req.inline {
		entrypoint = modifyEntrypointForInline(req.kind, entrypoint)
	}
	return entrypoint, nil
}

func promptForNewDefinition(defaultFilename, entrypoint string, p prompts.Prompter) (string, error) {
	entrypointDir := filepath.Dir(entrypoint)
	defaultFilename = filepath.Join(entrypointDir, defaultFilename)

	var filename string
	if err := p.Input(
		"Where should the definition file be created?",
		&filename,
		prompts.WithDefault(defaultFilename),
		prompts.WithSuggest(func(toComplete string) []string {
			files, _ := filepath.Glob(toComplete + "*")
			return files
		}),
		prompts.WithValidator(func(val interface{}) error {
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

var inlineSupportedKinds = []buildtypes.TaskKind{buildtypes.TaskKindNode, buildtypes.TaskKindPython}

func isInlineSupportedKind(kind buildtypes.TaskKind) bool {
	return slices.Contains(inlineSupportedKinds, kind)
}

func createTaskEntrypoint(r runtime.Interface, entrypoint string, task *libapi.Task) error {
	code, fileMode, err := r.Generate(apiTaskToRuntimeTask(task))
	if err != nil {
		return err
	}

	return writeTaskEntrypoint(entrypoint, code, fileMode)
}

func createInlineEntrypoint(r runtime.Interface, entrypoint string, def *definitions.Definition, printToStdOut bool) error {
	code, fileMode, err := r.GenerateInline(def)
	if err != nil {
		return err
	}
	if printToStdOut {
		return printEntrypointToStdOut(code)
	}

	return writeTaskEntrypoint(entrypoint, code, fileMode)
}

func modifyEntrypointForInline(kind buildtypes.TaskKind, entrypoint string) string {
	if !isInlineSupportedKind(kind) {
		return entrypoint
	}

	ext := filepath.Ext(entrypoint)
	entrypointWithoutExt := strings.TrimSuffix(entrypoint, ext)

	if kind == buildtypes.TaskKindNode && !strings.HasSuffix(entrypointWithoutExt, ".airplane") {
		return fmt.Sprintf("%s.airplane%s", entrypointWithoutExt, ext)
	}
	if kind == buildtypes.TaskKindPython && !strings.HasSuffix(entrypointWithoutExt, "_airplane") {
		return fmt.Sprintf("%s_airplane%s", entrypointWithoutExt, ext)
	}
	return entrypoint
}

func addEntrypointSuffixFunc(req InitTaskRequest) func(string, string) string {
	if req.TaskKind == buildtypes.TaskKindPython && req.Inline {
		return func(s, suffix string) string {
			ext := filepath.Ext(s)
			if strings.HasSuffix(s, "_airplane"+ext) {
				base := strings.TrimSuffix(s, "_airplane"+ext)
				return fmt.Sprintf("%s_%s_airplane%s", base, suffix, ext)
			}
			return fsx.AddFileSuffix(s, suffix)
		}
	}

	return nil
}

func writeTaskEntrypoint(path string, b []byte, fileMode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(path, b, fileMode); err != nil {
		return err
	}

	return nil
}

func printEntrypointToStdOut(b []byte) error {
	logger.Log(string(b))
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

type runKindSpecificInstallationRequest struct {
	Prompter     prompts.Prompter
	DryRun       bool
	Inline       bool
	Kind         buildtypes.TaskKind
	Def          definitions.Definition
	InitResponse *InitResponse
}

func runKindSpecificInstallation(ctx context.Context, req runKindSpecificInstallationRequest) error {
	switch req.Kind {
	case buildtypes.TaskKindNode:
		entrypoint, err := req.Def.GetAbsoluteEntrypoint()
		if err != nil {
			return err
		}
		packageJSONDir, packageJSONCreated, err := node.CreatePackageJSON(filepath.Dir(entrypoint), node.PackageJSONOptions{
			Dependencies: node.NodeDependencies{
				Dependencies:    []string{"airplane"},
				DevDependencies: []string{"@types/node"},
			},
		}, req.Prompter, req.DryRun)
		if err != nil {
			return err
		}
		if req.InitResponse != nil {
			req.InitResponse.AddFile(packageJSONCreated, filepath.Join(packageJSONDir, "package.json"))
		}

		_, nodeVersion, buildBase, err := req.Def.GetBuildType()
		if err != nil {
			return err
		}
		if nodeVersion == "" {
			nodeVersion = buildtypes.DefaultNodeVersion
		}

		airplaneConfigCreated, err := createOrUpdateAirplaneConfig(packageJSONDir, deployconfig.AirplaneConfig{
			Javascript: deployconfig.JavaScriptConfig{
				NodeVersion: string(nodeVersion),
				Base:        string(buildBase),
			},
		}, req.DryRun)
		if err != nil {
			return err
		}
		if req.InitResponse != nil {
			req.InitResponse.AddFile(airplaneConfigCreated, filepath.Join(packageJSONDir, "airplane.yaml"))
		}

		if filepath.Ext(entrypoint) == ".ts" || filepath.Ext(entrypoint) == ".tsx" {
			// Create/update tsconfig in the same directory as the package.json file
			tsConfigCreated, err := node.CreateTaskTSConfig(packageJSONDir, req.Prompter, req.DryRun)
			if err != nil {
				return err
			}
			if req.InitResponse != nil {
				req.InitResponse.AddFile(tsConfigCreated, filepath.Join(packageJSONDir, "tsconfig.json"))
			}
		}

		return nil
	case buildtypes.TaskKindPython:
		var requirementsTxtDir string
		entrypoint, err := req.Def.GetAbsoluteEntrypoint()
		if err != nil {
			return err
		}
		var deps []python.PythonDependency
		if req.Inline {
			deps = []python.PythonDependency{
				{Name: "airplanesdk", Version: "~=0.3.14"},
			}
		}
		requirementsTxtDir, requirementsTxtCreated, err := python.CreateRequirementsTxt(filepath.Dir(entrypoint), python.RequirementsTxtOptions{
			Dependencies: deps,
		}, req.Prompter, req.DryRun)
		if err != nil {
			return err
		}
		if req.InitResponse != nil {
			req.InitResponse.AddFile(requirementsTxtCreated, filepath.Join(requirementsTxtDir, "requirements.txt"))
		}

		_, pythonVersion, buildBase, err := req.Def.GetBuildType()
		if err != nil {
			return err
		}
		if pythonVersion == "" {
			pythonVersion = buildtypes.DefaultPythonVersion
		}

		airplaneConfigCreated, err := createOrUpdateAirplaneConfig(requirementsTxtDir, deployconfig.AirplaneConfig{
			Python: deployconfig.PythonConfig{
				Version: string(pythonVersion),
				Base:    string(buildBase),
			},
		}, req.DryRun)
		if err != nil {
			return err
		}
		if req.InitResponse != nil {
			req.InitResponse.AddFile(airplaneConfigCreated, filepath.Join(requirementsTxtDir, "airplane.yaml"))
		}
		return nil
	default:
		return nil
	}
}

// createOrUpdateAirplaneConfig creates or updates an existing airplane.yaml. Returns true if a
// file was created.
func createOrUpdateAirplaneConfig(root string, cfg deployconfig.AirplaneConfig, dryRun bool) (bool, error) {
	var existingConfig deployconfig.AirplaneConfig
	var err error
	existingConfigFilePath := filepath.Join(root, deployconfig.FileName)
	hasExistingConfigFile := fsx.Exists(existingConfigFilePath)
	if hasExistingConfigFile {
		existingConfig, err = deployconfig.NewAirplaneConfigFromFile(existingConfigFilePath)
		if err != nil {
			return false, err
		}
	}

	if !dryRun {
		f, err := os.OpenFile(existingConfigFilePath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return false, err
		}

		if err := writeNewAirplaneConfig(f, getNewAirplaneConfigOptions{
			cfg:               cfg,
			existingConfig:    existingConfig,
			hasExistingConfig: hasExistingConfigFile,
		}); err != nil {
			return false, err
		}
	}

	return !hasExistingConfigFile, nil
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
				logger.Warning("> javascript:")
				logger.Warning(">   nodeVersion: \"18\"")
			}
			if opts.cfg.Python.Version != "" && opts.existingConfig.Python.Version == "" {
				logger.Warning("We recommend specifying a python.version in your %s.", deployconfig.FileName)
				logger.Warning("> python:")
				logger.Warning(">   version: \"3.10\"")
			}
			return nil
		}
	}

	e := yaml.NewEncoder(writer)
	defer e.Close() //nolint:errcheck
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
