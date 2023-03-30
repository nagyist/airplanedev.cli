package kind_configs

import (
	"encoding/json"

	"github.com/airplanedev/cli/pkg/resources"
)

type InternalResource struct {
	ID             string                 `json:"id" db:"id"`
	Slug           string                 `json:"slug" db:"slug"`
	Name           string                 `json:"name" db:"name"`
	Kind           resources.ResourceKind `json:"kind" db:"kind"`
	ExportResource resources.Resource     `json:"resource"`
}

func (r *InternalResource) UnmarshalJSON(buf []byte) error {
	var raw struct {
		ID             string                 `json:"id"`
		Slug           string                 `json:"slug"`
		Name           string                 `json:"name"`
		Kind           resources.ResourceKind `json:"kind"`
		ExportResource map[string]interface{} `json:"resource"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}

	var export resources.Resource
	var err error
	if raw.ExportResource != nil {
		export, err = resources.GetResource(resources.ResourceKind(raw.Kind), raw.ExportResource)
		if err != nil {
			return err
		}
	}

	r.ID = raw.ID
	r.Slug = raw.Slug
	r.Name = raw.Name
	r.Kind = raw.Kind
	r.ExportResource = export

	return nil
}

func (r InternalResource) ToExternalResource() (resources.Resource, error) {
	return r.ExportResource, nil
}

const KindUnknown resources.ResourceKind = ""
