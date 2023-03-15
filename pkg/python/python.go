package python

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
)

const defaultMaxParentDirs = 2

type PythonDependency struct {
	Name    string
	Version string
}
type RequirementsTxtOptions = struct {
	Dependencies []PythonDependency
}

// CreateRequirementsTxt ensures there is a requirements.txt in the project with the provided dependencies installed.
// If requirements.txt exists in cwd, use it.
// If requirements.txt exists in parent directory, ask user if they want to use that or create a new one.
// If requirements.txt doesn't exist, create a new one.
// Returns the path to the directory where the requirements.txt is created/found.
func CreateRequirementsTxt(directory string, options RequirementsTxtOptions, p prompts.Prompter) (string, error) {
	// Check if there's a requirements.txt in the current or parent directory of entrypoint
	requirementsTxtDirPath, ok := fsx.Find(directory, "requirements.txt")

	var selectedRequirementsTxtDir string
	if ok {
		if requirementsTxtDirPath == directory {
			selectedRequirementsTxtDir = requirementsTxtDirPath
		} else {
			opts := []string{
				"Yes (recommended)",
				"No, create a nested project",
			}
			useExisting := opts[0]
			var surveyResp string
			formattedPath, err := formatFilepath(directory, filepath.Join(requirementsTxtDirPath, "requirements.txt"), defaultMaxParentDirs)
			if err != nil {
				return "", err
			}
			if err := p.Input(
				fmt.Sprintf("Found an existing project with requirements.txt at %s. Use this project?", formattedPath),
				&surveyResp,
				prompts.WithSelectOptions(opts),
				prompts.WithDefault(useExisting),
			); err != nil {
				return "", err
			}
			if surveyResp == useExisting {
				selectedRequirementsTxtDir = requirementsTxtDirPath
			} else {
				selectedRequirementsTxtDir = directory
				if err := createRequirementsTxtFile(selectedRequirementsTxtDir); err != nil {
					return "", err
				}
			}
		}
	} else {
		selectedRequirementsTxtDir = directory
		if err := createRequirementsTxtFile(selectedRequirementsTxtDir); err != nil {
			return "", err
		}
	}

	return selectedRequirementsTxtDir, addAllPackages(selectedRequirementsTxtDir, options.Dependencies)
}

func addAllPackages(requirementsTxtDirPath string, dependencies []PythonDependency) error {
	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{WithLoader: true})
	defer l.StopLoader()

	requirementsTxtPath := filepath.Join(requirementsTxtDirPath, "requirements.txt")
	file, err := os.OpenFile(requirementsTxtPath, os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to open requirements.txt")
	}
	defer file.Close()

	dependenciesToAdd, err := getPackagesToAdd(file, dependencies)
	if err != nil {
		return err
	}

	var toWrite []string
	var deps []string
	for _, d := range dependenciesToAdd {
		toWrite = append(toWrite, fmt.Sprintf("%s%s", d.Name, d.Version))
		deps = append(deps, d.Name)
	}
	if len(toWrite) > 0 {
		if _, err := file.WriteString(strings.Join(toWrite, "\n")); err != nil {
			return errors.Wrap(err, "failed to write to requirements.txt")
		}
		cwd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, "getting working directory")
		}
		relRequirementsTxtPath, _ := formatFilepath(cwd, requirementsTxtPath, 4)
		if relRequirementsTxtPath == "" {
			relRequirementsTxtPath = requirementsTxtPath
		}
		l.Step(fmt.Sprintf("Added %s to %s. Run `pip install -r %s` to install dependencies locally.", strings.Join(deps, ", "), relRequirementsTxtPath, relRequirementsTxtPath))
	}

	return nil
}

func getPackagesToAdd(requirementsTxtReader io.Reader, dependencies []PythonDependency) ([]PythonDependency, error) {
	alreadyAddedDependencies := make(map[string]interface{})
	scanner := bufio.NewScanner(requirementsTxtReader)
	for scanner.Scan() {
		if len(alreadyAddedDependencies) == len(dependencies) {
			break
		}

		for _, d := range dependencies {
			if strings.HasPrefix(scanner.Text(), d.Name) {
				alreadyAddedDependencies[d.Name] = struct{}{}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to read requirements.txt")
	}

	var dependenciesToAdd []PythonDependency
	for _, d := range dependencies {
		if _, ok := alreadyAddedDependencies[d.Name]; !ok {
			dependenciesToAdd = append(dependenciesToAdd, d)
		}
	}

	return dependenciesToAdd, nil
}

func createRequirementsTxtFile(directory string) error {
	if _, err := os.Create(filepath.Join(directory, "requirements.txt")); err != nil {
		return errors.Wrap(err, "writing requirements.txt")
	}
	logger.Step("Created requirements.txt")
	return nil
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
	}
	return relpath, nil
}
