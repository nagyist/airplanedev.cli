package definitions

import (
	"encoding/json"

	"github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/goccy/go-yaml"
)

type DefaultTrueDefinition struct {
	value *bool
}

var _ yaml.IsZeroer = &DefaultTrueDefinition{}
var _ json.Unmarshaler = &DefaultTrueDefinition{}
var _ json.Marshaler = &DefaultTrueDefinition{}

func NewDefaultTrueDefinition(value bool) DefaultTrueDefinition {
	return DefaultTrueDefinition{&value}
}

func (d DefaultTrueDefinition) Value() bool {
	if d.value == nil {
		return true
	} else {
		return *d.value
	}
}

func (d *DefaultTrueDefinition) UnmarshalJSON(b []byte) error {
	var v bool
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	d.value = &v
	return nil
}

func (d DefaultTrueDefinition) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Value())
}

func (d DefaultTrueDefinition) IsZero() bool {
	return d.Value()
}

type DefaultOneDefinition struct {
	value *int
}

var _ yaml.IsZeroer = &DefaultOneDefinition{}
var _ json.Unmarshaler = &DefaultOneDefinition{}
var _ json.Marshaler = &DefaultOneDefinition{}

func NewDefaultOneDefinition(value int) DefaultOneDefinition {
	return DefaultOneDefinition{&value}
}

func (d DefaultOneDefinition) Value() int {
	if d.value == nil {
		return 1
	}
	return *d.value
}

func (d *DefaultOneDefinition) UnmarshalJSON(b []byte) error {
	var v int
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	d.value = &v
	return nil
}

func (d DefaultOneDefinition) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Value())
}

func (d DefaultOneDefinition) IsZero() bool {
	return d.Value() == 1
}

type DefaultTaskViewersDefinition struct {
	value *api.DefaultRunPermissions
}

var _ yaml.IsZeroer = &DefaultTaskViewersDefinition{}
var _ json.Unmarshaler = &DefaultTaskViewersDefinition{}
var _ json.Marshaler = &DefaultTaskViewersDefinition{}

func NewDefaultTaskViewersDefinition(value api.DefaultRunPermissions) DefaultTaskViewersDefinition {
	return DefaultTaskViewersDefinition{&value}
}

func (d DefaultTaskViewersDefinition) Value() api.DefaultRunPermissions {
	if d.value == nil || *d.value == "" {
		return api.DefaultRunPermissionTaskViewers
	}
	return *d.value
}

func (d *DefaultTaskViewersDefinition) UnmarshalJSON(b []byte) error {
	var v api.DefaultRunPermissions
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	d.value = &v
	return nil
}

func (d DefaultTaskViewersDefinition) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Value())
}
func (d DefaultTaskViewersDefinition) MarshalYAML() (interface{}, error) {
	return d.Value(), nil
}

func (d DefaultTaskViewersDefinition) IsZero() bool {
	return d.Value() == api.DefaultRunPermissionTaskViewers
}
