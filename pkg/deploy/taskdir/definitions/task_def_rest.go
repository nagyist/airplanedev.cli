package definitions

import (
	"github.com/airplanedev/lib/pkg/api"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/pkg/errors"
)

var _ taskKind = &RESTDefinition{}

type RESTDefinition struct {
	Resource      string                 `json:"resource"`
	Method        string                 `json:"method"`
	Path          string                 `json:"path"`
	URLParams     map[string]interface{} `json:"urlParams,omitempty"`
	Headers       map[string]interface{} `json:"headers,omitempty"`
	BodyType      string                 `json:"bodyType,omitempty"`
	Body          interface{}            `json:"body,omitempty"`
	FormData      map[string]interface{} `json:"formData,omitempty"`
	RetryFailures interface{}            `json:"retryFailures,omitempty"`
	Configs       []string               `json:"configs,omitempty"`
}

func (d *RESTDefinition) copyToTask(task *api.Task, bc buildtypes.BuildConfig, opts GetTaskOpts) error {
	// Check slugs first.
	if resource := getResourceBySlug(opts.AvailableResources, d.Resource); resource != nil {
		task.Resources["rest"] = resource.ID
	} else if resource := getResourceByName(opts.AvailableResources, d.Resource); resource != nil {
		task.Resources["rest"] = resource.ID
	} else if !opts.IgnoreInvalid {
		return api.ResourceMissingError{Slug: d.Resource}
	}
	return nil
}

func (d *RESTDefinition) update(t api.UpdateTaskRequest, availableResources []api.ResourceMetadata) error {
	if resID, ok := t.Resources["rest"]; ok {
		if resource := getResourceByID(availableResources, resID); resource != nil {
			d.Resource = resource.Slug
		}
	}
	if v, ok := t.KindOptions["method"]; ok {
		if sv, ok := v.(string); ok {
			d.Method = sv
		} else {
			return errors.Errorf("expected string method, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["path"]; ok {
		if sv, ok := v.(string); ok {
			d.Path = sv
		} else {
			return errors.Errorf("expected string path, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["urlParams"]; ok {
		if mv, ok := v.(map[string]interface{}); ok {
			d.URLParams = mv
		} else {
			return errors.Errorf("expected map urlParams, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["headers"]; ok {
		if mv, ok := v.(map[string]interface{}); ok {
			d.Headers = mv
		} else {
			return errors.Errorf("expected map headers, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["bodyType"]; ok {
		if sv, ok := v.(string); ok {
			d.BodyType = sv
		} else {
			return errors.Errorf("expected string bodyType, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["body"]; ok {
		d.Body = v
	}
	if v, ok := t.KindOptions["formData"]; ok {
		if mv, ok := v.(map[string]interface{}); ok {
			d.FormData = mv
		} else {
			return errors.Errorf("expected map formData, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["retryFailures"]; ok {
		d.RetryFailures = v
	}

	return nil
}

func (d *RESTDefinition) setEntrypoint(entrypoint string) error {
	return ErrNoEntrypoint
}

func (d *RESTDefinition) setAbsoluteEntrypoint(entrypoint string) error {
	return ErrNoEntrypoint
}

func (d *RESTDefinition) getAbsoluteEntrypoint() (string, error) {
	return "", ErrNoEntrypoint
}

func (d *RESTDefinition) getKindOptions() (buildtypes.KindOptions, error) {
	if d.URLParams == nil {
		d.URLParams = map[string]interface{}{}
	}
	if d.Headers == nil {
		d.Headers = map[string]interface{}{}
	}
	if d.FormData == nil {
		d.FormData = map[string]interface{}{}
	}
	return buildtypes.KindOptions{
		"method":        d.Method,
		"path":          d.Path,
		"urlParams":     d.URLParams,
		"headers":       d.Headers,
		"bodyType":      d.BodyType,
		"body":          d.Body,
		"formData":      d.FormData,
		"retryFailures": d.RetryFailures,
	}, nil
}

func (d *RESTDefinition) getEntrypoint() (string, error) {
	return "", ErrNoEntrypoint
}

func (d *RESTDefinition) getEnv() (api.TaskEnv, error) {
	return nil, nil
}
func (d *RESTDefinition) setEnv(e api.TaskEnv) error {
	return nil
}

func (d *RESTDefinition) getConfigAttachments() []api.ConfigAttachment {
	configAttachments := make([]api.ConfigAttachment, len(d.Configs))
	for i, configName := range d.Configs {
		configAttachments[i] = api.ConfigAttachment{NameTag: configName}
	}

	return configAttachments
}

// Rewrites Resource to be a slug if it's a name.
func (d *RESTDefinition) normalize(availableResources []api.ResourceMetadata) error {
	// Check slugs first.
	if resource := getResourceBySlug(availableResources, d.Resource); resource != nil {
		return nil
	} else if resource := getResourceByName(availableResources, d.Resource); resource != nil {
		d.Resource = resource.Slug
	} else {
		return api.ResourceMissingError{Slug: d.Resource}
	}
	return nil
}

func (d *RESTDefinition) getResourceAttachments() map[string]string {
	return map[string]string{"rest": d.Resource}
}

func (d *RESTDefinition) getBuildType() (buildtypes.BuildType, buildtypes.BuildTypeVersion, buildtypes.BuildBase) {
	return buildtypes.NoneBuildType, buildtypes.BuildTypeVersionUnspecified, buildtypes.BuildBaseNone
}
func (d *RESTDefinition) SetBuildVersionBase(v buildtypes.BuildTypeVersion, b buildtypes.BuildBase) {
}
