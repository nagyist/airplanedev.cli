package definitions

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"text/template"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	api "github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

// ViewDefinition specifies a view's configuration.
//
// A definition is commonly serialized as JavaScript, co-located with the View's code. However, it can
// also be serialized as YAML.
//
// Optional fields should have `omitempty` set. If omitempty doesn't work on the field's type, add an
// `IsZero` method. See the `DefaultXYZDefinition` structs as an example. Add test cases to
// TestDefinitionMarshal to confirm this behavior. This behavior is relied upon when updating
// view definitions via `pkg/cli/views.Update` (you should add test cases in the corresponding
// `pkg/definitions/updaters` when adding new fields).
type ViewDefinition struct {
	Slug        string      `json:"slug"`
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
	Entrypoint  string      `json:"entrypoint"`
	EnvVars     api.EnvVars `json:"envVars,omitempty"`
	// DefnFilePath is the absolute path to this View definition, if one exists.
	DefnFilePath string               `json:"-"`
	Base         buildtypes.BuildBase `json:"base,omitempty"`
}

//go:embed view_schema.json
var viewSchemaStr string

func GetViewSchema() string {
	return viewSchemaStr
}

// Update updates a definition by applying the UpdateViewRequest using patch semantics.
func (d *ViewDefinition) Update(req api.UpdateViewRequest) error {
	d.Slug = req.Slug
	d.Name = req.Name
	d.Description = req.Description

	if req.EnvVars != nil {
		d.EnvVars = req.EnvVars
	}

	return nil
}

func (d *ViewDefinition) Unmarshal(format DefFormat, buf []byte) error {
	var err error
	switch format {
	case DefFormatYAML:
		buf, err = yaml.YAMLToJSON(buf)
		if err != nil {
			return err
		}
	case DefFormatJSON:
		// nothing
	default:
		return errors.Errorf("unknown format: %s", format)
	}

	schemaLoader := gojsonschema.NewStringLoader(viewSchemaStr)
	docLoader := gojsonschema.NewBytesLoader(buf)

	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return errors.Wrap(err, "validating schema")
	}

	if !result.Valid() {
		return errors.WithStack(ErrSchemaValidation{Errors: result.Errors()})
	}

	if err = json.Unmarshal(buf, &d); err != nil {
		return err
	}
	return nil
}

func (d ViewDefinition) Marshal(format DefFormat) ([]byte, error) {
	switch format {
	case DefFormatYAML:
		// Use the JSON marshaler so we use MarshalJSON methods.
		buf, err := yaml.MarshalWithOptions(d,
			yaml.UseJSONMarshaler(),
			yaml.UseLiteralStyleIfMultiline(true))
		if err != nil {
			return nil, err
		}
		return buf, nil

	case DefFormatJSON:
		// Use the YAML marshaler so we can take advantage of the yaml.IsZeroer check on omitempty.
		// But make it use the JSON marshaler so we use MarshalJSON methods.
		buf, err := yaml.MarshalWithOptions(d,
			yaml.UseJSONMarshaler(),
			yaml.JSON())
		if err != nil {
			return nil, err
		}
		// `yaml.Marshal` doesn't allow configuring JSON indentation, so do it after the fact.
		var out bytes.Buffer
		if err := json.Indent(&out, buf, "", "\t"); err != nil {
			return nil, err
		}
		return out.Bytes(), nil

	default:
		return nil, errors.Errorf("unknown format: %s", format)
	}
}

func (d *ViewDefinition) GenerateCommentedFile() ([]byte, error) {
	tmpl, err := template.New("definition").Parse(viewDefinitionTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "parsing definition template")
	}
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, map[string]interface{}{
		"slug":       d.Slug,
		"name":       d.Name,
		"entrypoint": d.Entrypoint,
	}); err != nil {
		return nil, errors.Wrap(err, "executing definition template")
	}
	return buf.Bytes(), nil
}
