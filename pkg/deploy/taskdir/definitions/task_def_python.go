package definitions

import (
	"path"

	"github.com/airplanedev/lib/pkg/api"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/pkg/errors"
)

var _ taskKind = &PythonDefinition{}

type PythonDefinition struct {
	// Entrypoint is the relative path from the task definition file to the script. It does not
	// apply for inline configured tasks.
	Entrypoint string               `json:"entrypoint"`
	EnvVars    api.TaskEnv          `json:"envVars,omitempty"`
	Base       buildtypes.BuildBase `json:"base,omitempty"`
	Version    string               `json:"-"`

	absoluteEntrypoint string `json:"-"`
}

func (d *PythonDefinition) copyToTask(task *api.Task, bc buildtypes.BuildConfig, opts GetTaskOpts) error {
	task.Env = d.EnvVars
	if opts.Bundle {
		entrypointFunc, _ := bc["entrypointFunc"].(string)
		task.Command = []string{"python"}
		task.Arguments = []string{
			"/airplane/.airplane/shim.py",
			path.Join("/airplane/", bc["entrypoint"].(string)),
			entrypointFunc,
			"{{JSON.stringify(params)}}",
		}
	}
	return nil
}

func (d *PythonDefinition) update(t api.UpdateTaskRequest, availableResources []api.ResourceMetadata) error {
	if v, ok := t.KindOptions["entrypoint"]; ok {
		if sv, ok := v.(string); ok {
			d.Entrypoint = sv
		} else {
			return errors.Errorf("expected string entrypoint, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["version"]; ok {
		if sv, ok := v.(string); ok {
			d.Version = sv
		} else {
			return errors.Errorf("expected string version, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["base"]; ok {
		if sv, ok := v.(buildtypes.BuildBase); ok {
			d.Base = sv
		} else if sv, ok := v.(string); ok {
			d.Base = buildtypes.BuildBase(sv)
		} else {
			return errors.Errorf("expected string base, got %T instead", v)
		}
	}
	d.EnvVars = t.Env
	return nil
}

func (d *PythonDefinition) setEntrypoint(entrypoint string) error {
	d.Entrypoint = entrypoint
	return nil
}

func (d *PythonDefinition) setAbsoluteEntrypoint(entrypoint string) error {
	d.absoluteEntrypoint = entrypoint
	return nil
}

func (d *PythonDefinition) getAbsoluteEntrypoint() (string, error) {
	if d.absoluteEntrypoint == "" {
		return "", ErrNoAbsoluteEntrypoint
	}
	return d.absoluteEntrypoint, nil
}

func (d *PythonDefinition) getKindOptions() (buildtypes.KindOptions, error) {
	ko := buildtypes.KindOptions{}
	if d.Entrypoint != "" {
		ko["entrypoint"] = d.Entrypoint
	}
	if d.Base != "" {
		ko["base"] = d.Base
	}
	if d.Version != "" {
		ko["version"] = d.Version
	}
	return ko, nil
}

func (d *PythonDefinition) getEntrypoint() (string, error) {
	return d.Entrypoint, nil
}

func (d *PythonDefinition) getEnv() (api.TaskEnv, error) {
	return d.EnvVars, nil
}

func (d *PythonDefinition) setEnv(e api.TaskEnv) error {
	d.EnvVars = e
	return nil
}

func (d *PythonDefinition) getConfigAttachments() []api.ConfigAttachment {
	return []api.ConfigAttachment{}
}

func (d *PythonDefinition) getResourceAttachments() map[string]string {
	return nil
}

func (d *PythonDefinition) getBuildType() (buildtypes.BuildType, buildtypes.BuildTypeVersion, buildtypes.BuildBase) {
	return buildtypes.PythonBuildType, buildtypes.BuildTypeVersion(d.Version), d.Base
}

func (d *PythonDefinition) SetBuildVersionBase(v buildtypes.BuildTypeVersion, b buildtypes.BuildBase) {
	if d.Version == "" {
		d.Version = string(v)
	}
	if d.Base == "" {
		d.Base = b
	}
}
