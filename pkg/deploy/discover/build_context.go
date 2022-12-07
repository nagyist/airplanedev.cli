package discover

import (
	"path/filepath"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/airplanedev/lib/pkg/runtime"
	"github.com/airplanedev/lib/pkg/utils/fsx"
)

// taskBuildContext gets the build context for a task.
func taskBuildContext(taskroot string, taskRuntime runtime.Interface) (build.BuildContext, error) {
	buildVersion, err := taskRuntime.Version(taskroot)
	if err != nil {
		return build.BuildContext{}, err
	}

	var c config.AirplaneConfig
	hasAirplaneConfig := fsx.Exists(filepath.Join(taskroot, config.FileName))
	if hasAirplaneConfig {
		c, err = config.NewAirplaneConfigFromFile(taskroot)
		if err != nil {
			return build.BuildContext{}, err
		}
	}
	envVars := make(map[string]build.EnvVarValue)
	switch taskRuntime.Kind() {
	case build.TaskKindNode:
		for k, v := range c.Javascript.EnvVars {
			envVars[k] = build.EnvVarValue(v)
		}
	case build.TaskKindPython:
		for k, v := range c.Python.EnvVars {
			envVars[k] = build.EnvVarValue(v)
		}
	}

	if len(envVars) == 0 {
		envVars = nil
	}

	var base build.BuildBase
	switch taskRuntime.Kind() {
	case build.TaskKindNode:
		base = build.BuildBase(c.Javascript.Base)
	case build.TaskKindPython:
		base = build.BuildBase(c.Python.Base)
	}

	return build.BuildContext{
		Version: buildVersion,
		Base:    base,
		EnvVars: envVars,
	}, nil
}
