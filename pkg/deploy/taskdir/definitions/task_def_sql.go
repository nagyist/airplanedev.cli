package definitions

import (
	"os"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

var _ taskKind = &SQLDefinition{}

type SQLDefinition struct {
	Resource        string                 `json:"resource"`
	Entrypoint      string                 `json:"entrypoint"`
	QueryArgs       map[string]interface{} `json:"queryArgs,omitempty"`
	TransactionMode SQLTransactionMode     `json:"transactionMode,omitempty"`
	Configs         []string               `json:"configs,omitempty"`

	// Contents of Entrypoint, cached
	entrypointContents string `json:"-"`
	absoluteEntrypoint string `json:"-"`
}

type SQLTransactionMode string

var _ yaml.IsZeroer = SQLTransactionMode("")

func (tm SQLTransactionMode) IsZero() bool {
	return tm == "auto" || tm == ""
}

func (tm SQLTransactionMode) Value() string {
	if tm == "" {
		return "auto"
	}
	return string(tm)
}

func (d *SQLDefinition) GetQuery() (string, error) {
	if d.entrypointContents == "" {
		if d.absoluteEntrypoint == "" {
			return "", ErrNoAbsoluteEntrypoint
		}
		queryBytes, err := os.ReadFile(d.absoluteEntrypoint)
		if err != nil {
			return "", errors.Wrapf(err, "reading SQL entrypoint %s", d.Entrypoint)
		}
		d.entrypointContents = string(queryBytes)
	}
	return d.entrypointContents, nil
}

func (d *SQLDefinition) copyToTask(task *api.Task, bc build.BuildConfig, opts GetTaskOpts) error {
	// Check slugs first.
	if resource := getResourceBySlug(opts.AvailableResources, d.Resource); resource != nil {
		task.Resources["db"] = resource.ID
	} else if resource := getResourceByName(opts.AvailableResources, d.Resource); resource != nil {
		task.Resources["db"] = resource.ID
	} else if !opts.IgnoreInvalid {
		return api.ResourceMissingError{Slug: d.Resource}
	}
	return nil
}

func (d *SQLDefinition) update(t api.UpdateTaskRequest, availableResources []api.ResourceMetadata) error {
	if resID, ok := t.Resources["db"]; ok {
		if resource := getResourceByID(availableResources, resID); resource != nil {
			d.Resource = resource.Slug
		}
	}
	if v, ok := t.KindOptions["entrypoint"]; ok {
		if sv, ok := v.(string); ok {
			d.Entrypoint = sv
		} else {
			return errors.Errorf("expected string entrypoint, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["query"]; ok {
		if sv, ok := v.(string); ok {
			d.entrypointContents = sv
		} else {
			return errors.Errorf("expected string query, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["queryArgs"]; ok {
		if mv, ok := v.(map[string]interface{}); ok {
			d.QueryArgs = mv
		} else {
			return errors.Errorf("expected map queryArgs, got %T instead", v)
		}
	}
	if v, ok := t.KindOptions["transactionMode"]; ok {
		if sv, ok := v.(string); ok {
			d.TransactionMode = SQLTransactionMode(sv)
		} else {
			return errors.Errorf("expected string transactionMode, got %T instead", v)
		}
	}

	return nil
}

func (d *SQLDefinition) setEntrypoint(entrypoint string) error {
	d.Entrypoint = entrypoint
	return nil
}

func (d *SQLDefinition) setAbsoluteEntrypoint(entrypoint string) error {
	d.absoluteEntrypoint = entrypoint
	return nil
}

func (d *SQLDefinition) getAbsoluteEntrypoint() (string, error) {
	if d.absoluteEntrypoint == "" {
		return "", ErrNoAbsoluteEntrypoint
	}
	return d.absoluteEntrypoint, nil
}

func (d *SQLDefinition) getKindOptions() (build.KindOptions, error) {
	query, err := d.GetQuery()
	if err != nil {
		return nil, err
	}
	if d.QueryArgs == nil {
		d.QueryArgs = map[string]interface{}{}
	}
	return build.KindOptions{
		"entrypoint":      d.Entrypoint,
		"query":           query,
		"queryArgs":       d.QueryArgs,
		"transactionMode": d.TransactionMode.Value(),
	}, nil
}

func (d *SQLDefinition) getEntrypoint() (string, error) {
	return d.Entrypoint, nil
}

func (d *SQLDefinition) getEnv() (api.TaskEnv, error) {
	return nil, nil
}

func (d *SQLDefinition) setEnv(e api.TaskEnv) error {
	return nil
}

func (d *SQLDefinition) getConfigAttachments() []api.ConfigAttachment {
	configAttachments := make([]api.ConfigAttachment, len(d.Configs))
	for i, configName := range d.Configs {
		configAttachments[i] = api.ConfigAttachment{NameTag: configName}
	}

	return configAttachments
}

// Rewrites Resource to be a slug if it's a name.
func (d *SQLDefinition) normalize(availableResources []api.ResourceMetadata) error {
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

func (d *SQLDefinition) getResourceAttachments() map[string]string {
	return map[string]string{"db": d.Resource}
}

func (d *SQLDefinition) getBuildType() (build.BuildType, build.BuildTypeVersion, build.BuildBase) {
	return build.NoneBuildType, build.BuildTypeVersionUnspecified, build.BuildBaseNone
}
func (d *SQLDefinition) SetBuildVersionBase(v build.BuildTypeVersion, b build.BuildBase) {
}
