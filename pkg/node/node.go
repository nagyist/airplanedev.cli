package node

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/pkg/errors"
	"github.com/tidwall/jsonc"
)

type NodeDependencies struct {
	Dependencies    []string
	DevDependencies []string
}

type PackageJSONOptions = struct {
	Dependencies NodeDependencies
}

// CreatePackageJSON ensures there is a package.json in path with the provided dependencies installed.
// If package.json exists in cwd, use it.
// If package.json exists in parent directory, ask user if they want to use that or create a new one.
// If package.json doesn't exist, create a new one.
// Returns the path to the directory where the package.json is created/found.
func CreatePackageJSON(directory string, packageJSONOptions PackageJSONOptions, p prompts.Prompter) (string, error) {
	// Check if there's a package.json in the current or parent directory of entrypoint
	packageJSONDirPath, ok := fsx.Find(directory, "package.json")
	useYarn := utils.ShouldUseYarn(packageJSONDirPath)

	var selectedPackageJSONDir string
	if ok {
		if packageJSONDirPath == directory {
			selectedPackageJSONDir = packageJSONDirPath
		} else {
			opts := []string{
				"Yes (recommended)",
				"No, create a nested project in my working directory",
			}
			useExisting := opts[0]
			var surveyResp string
			formattedPath, err := formatFilepath(directory, filepath.Join(packageJSONDirPath, "package.json"), defaultMaxParentDirs)
			if err != nil {
				return "", err
			}
			if err := p.Input(
				fmt.Sprintf("Found an existing project with package.json at %s. Use this project?", formattedPath),
				&surveyResp,
				prompts.WithSelectOptions(opts),
				prompts.WithDefault(useExisting),
			); err != nil {
				return "", err
			}
			if surveyResp == useExisting {
				selectedPackageJSONDir = packageJSONDirPath
			} else {
				selectedPackageJSONDir = directory
				if err := createPackageJSONFile(selectedPackageJSONDir); err != nil {
					return "", err
				}
			}
		}
	} else {
		selectedPackageJSONDir = directory
		if err := createPackageJSONFile(selectedPackageJSONDir); err != nil {
			return "", err
		}
	}

	return selectedPackageJSONDir, addAllPackages(selectedPackageJSONDir, useYarn, packageJSONOptions.Dependencies)
}

func addAllPackages(packageJSONDirPath string, useYarn bool, dependencies NodeDependencies) error {
	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{WithLoader: true})
	defer l.StopLoader()
	packageJSONPath := filepath.Join(packageJSONDirPath, "package.json")
	existingDeps, err := build.ListDependencies(packageJSONPath)
	if err != nil {
		return err
	}

	existingDepNames := make([]string, 0, len(existingDeps))
	for dep := range existingDeps {
		existingDepNames = append(existingDepNames, dep)
	}

	// TODO: Select versions to install instead of installing latest.
	// Put these in lib and use same ones for airplane tasks/views dev.
	packagesToAdd := getPackagesToAdd(dependencies.Dependencies, existingDepNames)
	devPackagesToAdd := getPackagesToAdd(dependencies.DevDependencies, existingDepNames)

	if len(packagesToAdd) > 0 || len(devPackagesToAdd) > 0 {
		l.Step("Installing dependencies...")
	}

	if len(packagesToAdd) > 0 {
		if err := addPackages(l, packageJSONDirPath, packagesToAdd, false, utils.InstallOptions{
			Yarn: useYarn,
		}); err != nil {
			return errors.Wrap(err, "installing dependencies")
		}
	}

	if len(devPackagesToAdd) > 0 {
		if err := addPackages(l, packageJSONDirPath, devPackagesToAdd, true, utils.InstallOptions{
			Yarn: useYarn,
		}); err != nil {
			return errors.Wrap(err, "installing dev dependencies")
		}
	}
	return nil
}

func getPackagesToAdd(packagesToCheck, existingDeps []string) []string {
	packagesToAdd := []string{}
	for _, pkg := range packagesToCheck {
		hasPackage := false
		for _, d := range existingDeps {
			if d == pkg {
				hasPackage = true
				break
			}
		}
		if !hasPackage {
			packagesToAdd = append(packagesToAdd, pkg)
		}
	}
	return packagesToAdd
}

func addPackages(l logger.Logger, packageJSONDirPath string, packageNames []string, dev bool, opts utils.InstallOptions) error {
	installArgs := []string{"add"}
	if dev {
		if opts.Yarn {
			installArgs = append(installArgs, "--dev")
		} else {
			installArgs = append(installArgs, "--save-dev")
		}
	}
	installArgs = append(installArgs, packageNames...)
	if opts.NoBinLinks {
		installArgs = append(installArgs, "--no-bin-links")
	}
	var cmd *exec.Cmd
	if opts.Yarn {
		cmd = exec.Command("yarn", installArgs...)
		l.Debug("Adding packages using yarn")
	} else {
		cmd = exec.Command("npm", installArgs...)
		l.Debug("Adding packages using npm")
	}
	if opts.NoBinLinks {
		l.Debug("Installing with --no-bin-links")
	}

	cmd.Dir = packageJSONDirPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		errString := string(output)

		if !opts.NoBinLinks && strings.Contains(errString, utils.SymlinkErrString) {
			// Try installation again with NoBinLinks to get passed the symlink error.
			opts.NoBinLinks = true
			return addPackages(l, packageJSONDirPath, packageNames, dev, opts)
		}

		return errors.New(errString)
	}
	l.Step(fmt.Sprintf("Installed %s", strings.Join(packageNames, ", ")))
	return nil
}

//go:embed scaffolding/package.json
var packageJsonTemplateStr string

func createPackageJSONFile(cwd string) error {
	tmpl, err := template.New("packageJson").Parse(packageJsonTemplateStr)
	if err != nil {
		return errors.Wrap(err, "parsing package.json template")
	}
	normalizedCwd := strings.ReplaceAll(strings.ToLower(filepath.Base(cwd)), " ", "-")
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, map[string]interface{}{
		"name": normalizedCwd,
	}); err != nil {
		return errors.Wrap(err, "executing package.json template")
	}

	if err := os.WriteFile("package.json", buf.Bytes(), 0644); err != nil {
		return errors.Wrap(err, "writing package.json")
	}
	logger.Step("Created package.json")
	return nil
}

//go:embed scaffolding/viewTSConfig.json
var defaultViewTSConfig []byte

func CreateViewTSConfig(configDir string, p prompts.Prompter) error {
	return mergeTSConfig(configDir, defaultViewTSConfig, MergeStrategyPreferTemplate, p)
}

//go:embed scaffolding/taskTSConfig.json
var defaultTaskTSConfig []byte

func CreateTaskTSConfig(configDir string, p prompts.Prompter) error {
	return mergeTSConfig(configDir, defaultTaskTSConfig, MergeStrategyPreferExisting, p)
}

type MergeStrategy string

const (
	MergeStrategyPreferExisting MergeStrategy = "existing"
	MergeStrategyPreferTemplate MergeStrategy = "template"
)

func mergeTSConfig(configDir string, configFile []byte, strategy MergeStrategy, p prompts.Prompter) error {
	configPath, err := formatTSConfigPath(configDir)
	if err != nil {
		return errors.Wrap(err, "getting tsconfig path")
	}

	if fsx.Exists(configPath) {
		templateTSConfig, err := parseTSConfig(configFile)
		if err != nil {
			return errors.Wrap(err, "parsing template tsconfig")
		}

		logger.Debug("Found existing tsconfig.json...")

		existingFile, err := os.ReadFile(configPath)
		if err != nil {
			return errors.Wrap(err, "reading existing tsconfig.json")
		}
		existingTSConfig, err := parseTSConfig(existingFile)
		if err != nil {
			return errors.Wrap(err, "parsing existing tsconfig")
		}

		newTSConfig := map[string]interface{}{}
		if strategy == MergeStrategyPreferExisting {
			mergeMapsRecursively(newTSConfig, templateTSConfig)
			mergeMapsRecursively(newTSConfig, existingTSConfig)
		} else {
			mergeMapsRecursively(newTSConfig, existingTSConfig)
			mergeMapsRecursively(newTSConfig, templateTSConfig)
		}

		changesNeeded, err := printTSConfigChanges(configPath, existingTSConfig, newTSConfig)
		if err != nil {
			return err
		}

		if changesNeeded {
			if ok, err := p.Confirm(
				fmt.Sprintf("Would you like to override %s with these changes?", configPath),
				prompts.WithDefault(true),
			); err != nil {
				return err
			} else if !ok {
				return nil
			}

			configFile, err = json.MarshalIndent(newTSConfig, "", "  ")
			if err != nil {
				return errors.Wrap(err, "marshalling tsconfig.json file")
			}

			if err := os.WriteFile(configPath, configFile, 0644); err != nil {
				return errors.Wrap(err, "writing tsconfig.json")
			}
			logger.Step(fmt.Sprintf("Updated %s", configPath))
		}
	} else {
		if err := os.WriteFile(configPath, configFile, 0644); err != nil {
			return errors.Wrap(err, "writing tsconfig.json")
		}
		logger.Step(fmt.Sprintf("Created %s", configPath))
	}

	return nil
}

func parseTSConfig(configBytes []byte) (map[string]interface{}, error) {
	tsConfig := map[string]interface{}{}

	// tsconfig files allow comments and trailing commas and thus aren't strict JSON;
	// use the jsonc library to remove these extra features before unmarshalling.
	configJSONBytes := jsonc.ToJSON(configBytes)
	err := json.Unmarshal(configJSONBytes, &tsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling tsconfig")
	}

	return tsConfig, nil
}

func mergeMapsRecursively(dest, src map[string]interface{}) {
	for key, value := range src {
		if subMap, isSubMap := value.(map[string]interface{}); isSubMap {
			if destSubMap, ok := dest[key]; !ok {
				dest[key] = map[string]interface{}{}
			} else if _, ok := destSubMap.(map[string]interface{}); !ok {
				dest[key] = map[string]interface{}{}
			}
			mergeMapsRecursively(dest[key].(map[string]interface{}), subMap)
		} else {
			dest[key] = src[key]
		}
	}
}

// prints changes between two maps and returns whether there are differences
func printTSConfigChanges(configPath string, oldConfig, newConfig map[string]interface{}) (bool, error) {
	oldBytes, err := json.MarshalIndent(oldConfig, "", "  ")
	if err != nil {
		return false, errors.Wrap(err, "marshalling old tsconfig")
	}
	oldStr := string(oldBytes)

	newBytes, err := json.MarshalIndent(newConfig, "", "  ")
	if err != nil {
		return false, errors.Wrap(err, "marshalling new tsconfig")
	}
	newStr := string(newBytes)

	if oldStr == newStr {
		return false, nil
	}

	edits := myers.ComputeEdits(span.URIFromPath(configPath), oldStr, newStr)
	diff := fmt.Sprint(gotextdiff.ToUnified(configPath, fmt.Sprintf("%s (updated)", configPath), oldStr, edits))

	logger.Log(
		"\nSome updates to your tsconfig are needed for Airplane tasks and/or views:\n%s",
		diff,
	)
	return true, nil
}

const defaultMaxParentDirs = 2

func formatTSConfigPath(configDir string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "getting working directory")
	}
	return formatFilepath(cwd, filepath.Join(configDir, "tsconfig.json"), defaultMaxParentDirs)
}

// Attempts to get the relative path for the base path and target path. If the target path is more
// than maxParentDirs parent directories above the base path, return the absolute path of the target path.
func formatFilepath(basepath, targpath string, maxParentDirs int) (string, error) {
	relpath, err := filepath.Rel(basepath, targpath)
	if err != nil {
		return "", errors.Wrap(err, "getting relative path")
	}
	if strings.Count(relpath, "..") > maxParentDirs {
		return filepath.Abs(targpath)
	} else {
		return relpath, nil
	}
}
