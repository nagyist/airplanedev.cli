package definitions

import (
	"github.com/airplanedev/cli/pkg/api"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/alessio/shellescape"
	"github.com/flynn/go-shlex"
)

var _ taskKind = &ImageDefinition{}

type ImageDefinition struct {
	Image      string      `json:"image"`
	Entrypoint string      `json:"entrypoint,omitempty"`
	Command    string      `json:"command"`
	EnvVars    api.TaskEnv `json:"envVars,omitempty"`
}

func (d *ImageDefinition) copyToTask(task *api.Task, bc buildtypes.BuildConfig, opts GetTaskOpts) error {
	if d.Image != "" {
		task.Image = &d.Image
	}
	if args, err := shlex.Split(d.Command); err != nil {
		return err
	} else {
		task.Arguments = args
	}
	if cmd, err := shlex.Split(d.Entrypoint); err != nil {
		return err
	} else {
		task.Command = cmd
	}
	task.Env = d.EnvVars
	return nil
}

func (d *ImageDefinition) update(t api.UpdateTaskRequest, availableResources []api.ResourceMetadata) error {
	if t.Image != nil {
		d.Image = *t.Image
	}
	d.Command = shellescape.QuoteCommand(t.Arguments)
	d.Entrypoint = shellescape.QuoteCommand(t.Command)
	d.EnvVars = t.Env
	return nil
}

func (d *ImageDefinition) setEntrypoint(entrypoint string) error {
	return ErrNoEntrypoint
}

func (d *ImageDefinition) setAbsoluteEntrypoint(entrypoint string) error {
	return ErrNoEntrypoint
}

func (d *ImageDefinition) getAbsoluteEntrypoint() (string, error) {
	return "", ErrNoEntrypoint
}

func (d *ImageDefinition) getKindOptions() (buildtypes.KindOptions, error) {
	return nil, nil
}

func (d *ImageDefinition) getEntrypoint() (string, error) {
	return "", ErrNoEntrypoint
}

func (d *ImageDefinition) getEnv() (api.TaskEnv, error) {
	return d.EnvVars, nil
}
func (d *ImageDefinition) setEnv(e api.TaskEnv) error {
	d.EnvVars = e
	return nil
}

func (d *ImageDefinition) getConfigAttachments() []api.ConfigAttachment {
	return []api.ConfigAttachment{}
}

func (d *ImageDefinition) getResourceAttachments() map[string]string {
	return nil
}

func (d *ImageDefinition) getBuildType() (buildtypes.BuildType, buildtypes.BuildTypeVersion, buildtypes.BuildBase) {
	return buildtypes.NoneBuildType, buildtypes.BuildTypeVersionUnspecified, buildtypes.BuildBaseNone
}

func (d *ImageDefinition) SetBuildVersionBase(v buildtypes.BuildTypeVersion, b buildtypes.BuildBase) {
}
