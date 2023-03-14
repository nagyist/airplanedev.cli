package definitions

import (
	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
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

func (d *ImageDefinition) copyToTask(task *api.Task, bc build.BuildConfig, opts GetTaskOpts) error {
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

func (d *ImageDefinition) hydrateFromTask(t api.Task, availableResources []api.ResourceMetadata) error {
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

func (d *ImageDefinition) getKindOptions() (build.KindOptions, error) {
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

func (d *ImageDefinition) getBuildType() (build.BuildType, build.BuildTypeVersion, build.BuildBase) {
	return build.NoneBuildType, build.BuildTypeVersionUnspecified, build.BuildBaseNone
}

func (d *ImageDefinition) SetBuildVersionBase(v build.BuildTypeVersion, b build.BuildBase) {
}
