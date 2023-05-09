package definitions

import (
	"fmt"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/pkg/errors"
)

var _ taskKind = &ShellDefinition{}

type ShellDefinition struct {
	Entrypoint string      `json:"entrypoint"`
	EnvVars    api.EnvVars `json:"envVars,omitempty"`

	absoluteEntrypoint string `json:"-"`
}

func (d *ShellDefinition) copyToTask(task *api.Task, bc buildtypes.BuildConfig, opts GetTaskOpts) error {
	task.Env = d.EnvVars
	if opts.Bundle {
		task.Command = []string{"bash"}
		task.Arguments = []string{
			".airplane/shim.sh",
			fmt.Sprintf("./%s", bc["entrypoint"].(string)),
		}
		// Pass slug={{slug}} as an array to the shell task
		for _, param := range task.Parameters {
			task.Arguments = append(task.Arguments, fmt.Sprintf("%s={{params.%s}}", param.Slug, param.Slug))
		}
		task.InterpolationMode = "jst"
	}

	return nil
}

func (d *ShellDefinition) update(t api.UpdateTaskRequest, availableResources []api.ResourceMetadata) error {
	if v, ok := t.KindOptions["entrypoint"]; ok {
		if sv, ok := v.(string); ok {
			d.Entrypoint = sv
		} else {
			return errors.Errorf("expected string entrypoint, got %T instead", v)
		}
	}
	d.EnvVars = t.Env
	return nil
}

func (d *ShellDefinition) setEntrypoint(entrypoint string) error {
	d.Entrypoint = entrypoint
	return nil
}

func (d *ShellDefinition) setAbsoluteEntrypoint(entrypoint string) error {
	d.absoluteEntrypoint = entrypoint
	return nil
}

func (d *ShellDefinition) getAbsoluteEntrypoint() (string, error) {
	if d.absoluteEntrypoint == "" {
		return "", ErrNoAbsoluteEntrypoint
	}
	return d.absoluteEntrypoint, nil
}

func (d *ShellDefinition) getKindOptions() (buildtypes.KindOptions, error) {
	return buildtypes.KindOptions{
		"entrypoint": d.Entrypoint,
	}, nil
}

func (d *ShellDefinition) getEntrypoint() (string, error) {
	return d.Entrypoint, nil
}

func (d *ShellDefinition) getEnv() (api.EnvVars, error) {
	return d.EnvVars, nil
}

func (d *ShellDefinition) setEnv(e api.EnvVars) error {
	d.EnvVars = e
	return nil
}

func (d *ShellDefinition) getConfigAttachments() []api.ConfigAttachment {
	return []api.ConfigAttachment{}
}

func (d *ShellDefinition) getResourceAttachments() map[string]string {
	return nil
}

func (d *ShellDefinition) getBuildType() (buildtypes.BuildType, buildtypes.BuildTypeVersion, buildtypes.BuildBase) {
	return buildtypes.ShellBuildType, buildtypes.BuildTypeVersionUnspecified, buildtypes.BuildBaseNone
}

func (d *ShellDefinition) SetBuildVersionBase(v buildtypes.BuildTypeVersion, b buildtypes.BuildBase) {
}
