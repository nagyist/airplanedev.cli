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
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/AlecAivazis/survey/v2"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/airplanedev/lib/pkg/runtime/javascript"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
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
func CreatePackageJSON(directory string, packageJSONOptions PackageJSONOptions) (string, error) {
	// Check if there's a package.json in the current or parent directory of entrypoint
	packageJSONDirPath, ok := fsx.Find(directory, "package.json")
	useYarn := utils.ShouldUseYarn(packageJSONDirPath)

	var selectedPackageJSONDir string
	if ok {
		if packageJSONDirPath == directory {
			selectedPackageJSONDir = packageJSONDirPath
		} else {
			opts := []string{
				"Yes",
				"No, create package.json in my working directory",
			}
			useExisting := opts[0]
			var surveyResp string
			formattedPath, err := formatFilepath(directory, filepath.Join(packageJSONDirPath, "package.json"), defaultMaxParentDirs)
			if err != nil {
				return "", err
			}
			if err := survey.AskOne(
				&survey.Select{
					Message: fmt.Sprintf("Found existing package.json at %s. Use this to manage dependencies?", formattedPath),
					Options: opts,
					Default: useExisting,
				},
				&surveyResp,
			); err != nil {
				return "", err
			}
			if surveyResp == useExisting {
				selectedPackageJSONDir = packageJSONDirPath
			} else {
				// Create a new package.json in the current directory.
				if err := createPackageJSONFile(directory); err != nil {
					return "", err
				}
				selectedPackageJSONDir = directory
			}
		}
	} else {
		selectedPackageJSONDir = directory
	}

	return selectedPackageJSONDir, addAllPackages(selectedPackageJSONDir, useYarn, packageJSONOptions.Dependencies)
}

// CreateOrUpdateAirplaneConfig creates or updates an existing airplane.config.yaml.
func CreateOrUpdateAirplaneConfig(directory string, cfg config.AirplaneConfig) error {
	var existingConfig config.AirplaneConfig
	var existingConfigFilePath string
	var err error
	existingConfigFileDir, hasExistingConfigFile := fsx.Find(directory, config.FileName)
	if hasExistingConfigFile {
		existingConfigFilePath = filepath.Join(existingConfigFileDir, config.FileName)
		existingConfig, err = config.NewAirplaneConfigFromFile(existingConfigFilePath)
		if err != nil {
			return err
		}
	}

	// Calculate node version
	runtime := javascript.Runtime{}
	root, err := runtime.Root(directory)
	if err != nil {
		return err
	}
	existingNodeVersion, err := runtime.Version(root)
	if err != nil {
		return err
	}
	correctNodeVersionSet := cfg.NodeVersion != "" && cfg.NodeVersion == existingNodeVersion

	// Calculate build base
	existingBuildBase := existingConfig.Base
	correctBuildBaseSet := cfg.Base != "" && cfg.Base == existingBuildBase

	if correctBuildBaseSet && correctNodeVersionSet {
		// Correct values already set.
		return nil
	}

	if cfg.NodeVersion != "" && existingNodeVersion != "" && cfg.NodeVersion != existingNodeVersion {
		cfg.NodeVersion = existingConfig.NodeVersion
		logger.Warning("Failed set Node.js version %s: conflicts with existing version %s", cfg.NodeVersion, existingNodeVersion)
	} else if cfg.NodeVersion == "" {
		cfg.NodeVersion = build.DefaultNodeVersion
	}
	if cfg.Base != "" && existingBuildBase != "" && cfg.Base != existingBuildBase {
		cfg.Base = existingConfig.Base
		logger.Warning("Failed set base %s: conflicts with existing base %s", cfg.Base, existingBuildBase)
	}

	pathToWrite := filepath.Join(root, config.FileName)
	if hasExistingConfigFile {
		pathToWrite = existingConfigFilePath
	}

	buf, err := yaml.Marshal(&cfg)
	if err != nil {
		fmt.Printf("Error while Marshaling. %v", err)
	}

	if err := os.WriteFile(pathToWrite, buf, 0644); err != nil {
		return errors.Wrapf(err, "writing %s", config.FileName)
	}

	if hasExistingConfigFile {
		logger.Step("Updated %s", config.FileName)
	} else {
		logger.Step("Created %s", config.FileName)
	}
	return nil

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

func CreateViewTSConfig(configDir string) error {
	return mergeTSConfig(configDir, defaultViewTSConfig, MergeStrategyPreferTemplate)
}

//go:embed scaffolding/taskTSConfig.json
var defaultTaskTSConfig []byte

func CreateTaskTSConfig(configDir string) error {
	return mergeTSConfig(configDir, defaultTaskTSConfig, MergeStrategyPreferExisting)
}

type MergeStrategy string

const (
	MergeStrategyPreferExisting MergeStrategy = "existing"
	MergeStrategyPreferTemplate MergeStrategy = "template"
)

func mergeTSConfig(configDir string, configFile []byte, strategy MergeStrategy) error {
	configPath, err := formatTSConfigPath(configDir)
	if err != nil {
		return errors.Wrap(err, "getting tsconfig path")
	}

	if fsx.Exists(configPath) {
		templateTSConfig := map[string]interface{}{}
		err := json.Unmarshal(configFile, &templateTSConfig)
		if err != nil {
			return errors.Wrap(err, "unmarshalling tsconfig template")
		}

		logger.Debug("Found existing tsconfig.json...")

		existingFile, err := os.ReadFile(configPath)
		if err != nil {
			return errors.Wrap(err, "reading existing tsconfig.json")
		}
		existingTSConfig := map[string]interface{}{}
		err = json.Unmarshal(existingFile, &existingTSConfig)
		if err != nil {
			return errors.Wrap(err, "unmarshalling existing tsconfig")
		}

		newTSConfig := map[string]interface{}{}
		if strategy == MergeStrategyPreferExisting {
			mergeMapsRecursively(newTSConfig, templateTSConfig)
			mergeMapsRecursively(newTSConfig, existingTSConfig)
		} else {
			mergeMapsRecursively(newTSConfig, existingTSConfig)
			mergeMapsRecursively(newTSConfig, templateTSConfig)
		}

		if printTSConfigChanges(newTSConfig, existingTSConfig, "") {
			var ok bool
			err = survey.AskOne(
				&survey.Confirm{
					Message: fmt.Sprintf("Would you like to override %s with these changes?", configPath),
					Default: true,
				},
				&ok,
			)
			if err != nil {
				return errors.Wrap(err, "asking user confirmation")
			}
			if !ok {
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
func printTSConfigChanges(superset, subset map[string]interface{}, parentName string) bool {
	var hasChanges bool
	b := new(bytes.Buffer)
	w := new(tabwriter.Writer)
	w.Init(b, 0, 4, 2, ' ', 0)
	for key, newVal := range superset {
		existingVal, ok := subset[key]
		keyName := key
		if parentName != "" {
			keyName = fmt.Sprintf("%s.%s", parentName, key)
		}
		if existingSubMap, isSubMap := existingVal.(map[string]interface{}); isSubMap {
			if printTSConfigChanges(newVal.(map[string]interface{}), existingSubMap, keyName) {
				hasChanges = true
			}
		} else if !ok || !reflect.DeepEqual(newVal, existingVal) {
			existingJSON, _ := json.Marshal(existingVal)
			newJSON, _ := json.Marshal(newVal)
			_, _ = w.Write([]byte(fmt.Sprintf("%s:\t(%s) -> (%s)\n", keyName, string(existingJSON), string(newJSON))))
			hasChanges = true
		}
	}
	w.Flush()
	if b.String() != "" {
		logger.Log("\n" + b.String())
	}
	return hasChanges
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
