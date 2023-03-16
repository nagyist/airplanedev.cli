package definitions

import (
	"fmt"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/pkg/errors"
)

var _ taskKind = &ShellDefinition{}

type ShellDefinition struct {
	Entrypoint string      `json:"entrypoint"`
	EnvVars    api.TaskEnv `json:"envVars,omitempty"`

	absoluteEntrypoint string `json:"-"`
}

func (d *ShellDefinition) copyToTask(task *api.Task, bc build.BuildConfig, opts GetTaskOpts) error {
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

func (d *ShellDefinition) getKindOptions() (build.KindOptions, error) {
	return build.KindOptions{
		"entrypoint": d.Entrypoint,
	}, nil
}

func (d *ShellDefinition) getEntrypoint() (string, error) {
	return d.Entrypoint, nil
}

func (d *ShellDefinition) getEnv() (api.TaskEnv, error) {
	return d.EnvVars, nil
}

func (d *ShellDefinition) setEnv(e api.TaskEnv) error {
	d.EnvVars = e
	return nil
}

func (d *ShellDefinition) getConfigAttachments() []api.ConfigAttachment {
	return []api.ConfigAttachment{}
}

func (d *ShellDefinition) getResourceAttachments() map[string]string {
	return nil
}

func (d *ShellDefinition) getBuildType() (build.BuildType, build.BuildTypeVersion, build.BuildBase) {
	return build.ShellBuildType, build.BuildTypeVersionUnspecified, build.BuildBaseNone
}

func (d *ShellDefinition) SetBuildVersionBase(v build.BuildTypeVersion, b build.BuildBase) {
}
