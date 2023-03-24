package definitions

import (
	"path"

	"github.com/airplanedev/lib/pkg/api"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
)

var _ taskKind = &NodeDefinition{}

type NodeDefinition struct {
	// Entrypoint is the relative path from the task definition file to the script. It does not
	// apply for inline configured tasks.
	Entrypoint  string               `json:"entrypoint"`
	NodeVersion string               `json:"nodeVersion"`
	EnvVars     api.TaskEnv          `json:"envVars,omitempty"`
	Base        buildtypes.BuildBase `json:"base,omitempty"`

	absoluteEntrypoint string `json:"-"`
}

func (d *NodeDefinition) copyToTask(task *api.Task, bc buildtypes.BuildConfig, opts GetTaskOpts) error {
	task.Env = d.EnvVars
	if opts.Bundle {
		entrypointFunc, _ := bc["entrypointFunc"].(string)
		entrypoint, _ := bc["entrypoint"].(string)
		if task.Runtime == buildtypes.TaskRuntimeWorkflow {
			// command needs to be initialized to an empty array
			// so that workflow commands get set correctly on the update path
			task.Command = []string{}
			task.Arguments = []string{
				"{{JSON.stringify(params)}}",
				entrypoint,
				entrypointFunc,
			}
		} else {
			entrypoint := path.Join("/airplane/.airplane/", entrypoint)
			// Ensure that the entrypoint is a .js file.
			entrypoint = fsx.TrimExtension(entrypoint) + ".js"
			task.Command = []string{"node"}
			task.Arguments = []string{
				"/airplane/.airplane/dist/universal-shim.js",
				entrypoint,
				entrypointFunc,
				"{{JSON.stringify(params)}}",
			}
		}
	}
	return nil
}

func (d *NodeDefinition) update(t api.UpdateTaskRequest, availableResources []api.ResourceMetadata) error {
	if v, ok := t.KindOptions["entrypoint"]; ok {
		if sv, ok := v.(string); ok {
			d.Entrypoint = sv
		} else {
			return errors.Errorf("expected string entrypoint, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["nodeVersion"]; ok {
		if sv, ok := v.(string); ok {
			d.NodeVersion = sv
		} else {
			return errors.Errorf("expected string nodeVersion, got %T instead", v)
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

func (d *NodeDefinition) setEntrypoint(entrypoint string) error {
	d.Entrypoint = entrypoint
	return nil
}

func (d *NodeDefinition) setAbsoluteEntrypoint(entrypoint string) error {
	d.absoluteEntrypoint = entrypoint
	return nil
}

func (d *NodeDefinition) getAbsoluteEntrypoint() (string, error) {
	if d.absoluteEntrypoint == "" {
		return "", ErrNoAbsoluteEntrypoint
	}
	return d.absoluteEntrypoint, nil
}

func (d *NodeDefinition) getKindOptions() (buildtypes.KindOptions, error) {
	ko := buildtypes.KindOptions{}
	if d.Entrypoint != "" {
		ko["entrypoint"] = d.Entrypoint
	}
	if d.NodeVersion != "" {
		ko["nodeVersion"] = d.NodeVersion
	}
	if d.Base != "" {
		ko["base"] = d.Base
	}
	return ko, nil
}

func (d *NodeDefinition) getEntrypoint() (string, error) {
	return d.Entrypoint, nil
}

func (d *NodeDefinition) getEnv() (api.TaskEnv, error) {
	return d.EnvVars, nil
}

func (d *NodeDefinition) setEnv(e api.TaskEnv) error {
	d.EnvVars = e
	return nil
}

func (d *NodeDefinition) getConfigAttachments() []api.ConfigAttachment {
	return []api.ConfigAttachment{}
}

func (d *NodeDefinition) getResourceAttachments() map[string]string {
	return nil
}

func (d *NodeDefinition) getBuildType() (buildtypes.BuildType, buildtypes.BuildTypeVersion, buildtypes.BuildBase) {
	return buildtypes.NodeBuildType, buildtypes.BuildTypeVersion(d.NodeVersion), d.Base
}

func (d *NodeDefinition) SetBuildVersionBase(v buildtypes.BuildTypeVersion, b buildtypes.BuildBase) {
	if d.NodeVersion == "" {
		d.NodeVersion = string(v)
	}
	if d.Base == "" {
		d.Base = b
	}
}
