package definitions

import (
	"encoding/json"

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
