package discover

import (
	"path/filepath"

	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/runtime/javascript"
	"github.com/airplanedev/lib/pkg/utils/fsx"
)

// TaskBuildContext gets the build context for a task.
func TaskBuildContext(taskroot string, taskRuntime runtime.Interface) (buildtypes.BuildContext, error) {
	buildVersion, err := taskRuntime.Version(taskroot)
	if err != nil {
		return buildtypes.BuildContext{}, err
	}

	var c config.AirplaneConfig
	hasAirplaneConfig := fsx.Exists(filepath.Join(taskroot, config.FileName))
	if hasAirplaneConfig {
		c, err = config.NewAirplaneConfigFromFile(taskroot)
		if err != nil {
			return buildtypes.BuildContext{}, err
		}
	}

	envVars := make(map[string]buildtypes.EnvVarValue)
	switch taskRuntime.Kind() {
	case buildtypes.TaskKindNode:
		for k, v := range c.Javascript.EnvVars {
			envVars[k] = buildtypes.EnvVarValue(v)
		}
	case buildtypes.TaskKindPython:
		for k, v := range c.Python.EnvVars {
			envVars[k] = buildtypes.EnvVarValue(v)
		}
	}

	if len(envVars) == 0 {
		envVars = nil
	}

	var base buildtypes.BuildBase
	switch taskRuntime.Kind() {
	case buildtypes.TaskKindNode:
		base = buildtypes.BuildBase(c.Javascript.Base)
	case buildtypes.TaskKindPython:
		base = buildtypes.BuildBase(c.Python.Base)
	}

	return buildtypes.BuildContext{
		Version: buildVersion,
		Base:    base,
		EnvVars: envVars,
	}, nil
}

// ViewBuildContext gets the build context for a view.
func ViewBuildContext(viewroot string) (buildtypes.BuildContext, error) {
	buildVersion, err := javascript.Runtime{}.Version(viewroot)
	if err != nil {
		return buildtypes.BuildContext{}, err
	}

	var c config.AirplaneConfig
	hasAirplaneConfig := fsx.Exists(filepath.Join(viewroot, config.FileName))
	if hasAirplaneConfig {
		c, err = config.NewAirplaneConfigFromFile(viewroot)
		if err != nil {
			return buildtypes.BuildContext{}, err
		}
	}

	envVars := make(map[string]buildtypes.EnvVarValue)
	for k, v := range c.View.EnvVars {
		envVars[k] = buildtypes.EnvVarValue(v)
	}
	if len(envVars) == 0 {
		envVars = nil
	}

	return buildtypes.BuildContext{
		Version: buildVersion,
		Base:    buildtypes.BuildBaseSlim,
		EnvVars: envVars,
	}, nil
}
