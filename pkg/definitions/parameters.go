package definitions

import (
	"reflect"

	api "github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/pkg/errors"
)

// Converts a definition file parameter into the corresponding format used by the API.
//
// Can be inverted by convertParameterAPIToDef.
func convertParameterDefToAPI(param ParameterDefinition) (api.Parameter, error) {
	out := api.Parameter{
		Name: param.Name,
		Slug: param.Slug,
		Desc: param.Description,
	}

	switch param.Type {
	case "shorttext":
		out.Type = api.TypeString
	case "longtext":
		out.Type = api.TypeString
		out.Component = api.ComponentTextarea
	case "sql":
		out.Type = api.TypeString
		out.Component = api.ComponentEditorSQL
	case "boolean", "upload", "integer", "float", "date", "datetime", "configvar":
		out.Type = api.Type(param.Type)
	default:
		return api.Parameter{}, errors.Errorf("unknown parameter type: %q", param.Type)
	}

	if param.Default != nil {
		if param.Type == "configvar" {
			switch reflect.ValueOf(param.Default).Kind() {
			case reflect.Map:
				m, ok := param.Default.(map[string]interface{})
				if !ok {
					return api.Parameter{}, errors.Errorf("expected map but got %T", param.Default)
				}
				if configName, ok := m["config"]; !ok {
					return api.Parameter{}, errors.Errorf("missing config property from configvar type: %v", param.Default)
				} else {
					out.Default = map[string]interface{}{
						"__airplaneType": "configvar",
						"name":           configName,
					}
				}
			case reflect.String:
				out.Default = map[string]interface{}{
					"__airplaneType": "configvar",
					"name":           param.Default,
				}
			default:
				return api.Parameter{}, errors.Errorf("unsupported type for default value: %T", param.Default)
			}
		} else {
			switch reflect.ValueOf(param.Default).Kind() {
			case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Float32, reflect.Float64:
				out.Default = param.Default
			default:
				return api.Parameter{}, errors.Errorf("unsupported type for default value: %T", param.Default)
			}
		}
	}

	if !param.Required.Value() {
		out.Constraints.Optional = true
	}

	out.Constraints.Regex = param.Regex

	if len(param.Options) > 0 {
		out.Constraints.Options = make([]api.ConstraintOption, len(param.Options))
		for i, opt := range param.Options {
			out.Constraints.Options[i].Label = opt.Label
			if opt.Config != nil {
				out.Constraints.Options[i].Value = map[string]interface{}{
					"__airplaneType": "configvar",
					"name":           *opt.Config,
				}
			} else if param.Type == "configvar" {
				out.Constraints.Options[i].Value = map[string]interface{}{
					"__airplaneType": "configvar",
					"name":           opt.Value,
				}
			} else {
				out.Constraints.Options[i].Value = opt.Value
			}
		}
	}

	return out, nil
}

// Converts a list of parameters from the format used by our API into the format used by definition files.
//
// Can be inverted by convertParametersAPIToDef.
func convertParametersDefToAPI(params []ParameterDefinition) ([]api.Parameter, error) {
	out := []api.Parameter{}
	for _, param := range params {
		converted, err := convertParameterDefToAPI(param)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

// Converts a parameter from the format used by our API into the format used by definition files.
//
// Can be inverted by convertParameterDefToAPI.
func convertParameterAPIToDef(param api.Parameter) (ParameterDefinition, error) {
	out := ParameterDefinition{
		Name:        param.Name,
		Slug:        param.Slug,
		Description: param.Desc,
	}

	switch param.Type {
	case "string":
		switch param.Component {
		case api.ComponentTextarea:
			out.Type = "longtext"
		case api.ComponentEditorSQL:
			out.Type = "sql"
		case api.ComponentNone:
			out.Type = "shorttext"
		default:
			return ParameterDefinition{}, errors.Errorf("unexpected component for type=string: %q", param.Component)
		}
	case "boolean", "upload", "integer", "float", "date", "datetime", "configvar":
		out.Type = string(param.Type)
	default:
		return ParameterDefinition{}, errors.Errorf("unknown parameter type: %q", param.Type)
	}

	if param.Default != nil {
		if param.Type == "configvar" {
			switch k := reflect.ValueOf(param.Default).Kind(); k {
			case reflect.Map:
				configName, err := extractConfigVarName(param.Default)
				if err != nil {
					return ParameterDefinition{}, errors.Wrap(err, "invalid default configvar")
				}
				out.Default = configName
			default:
				return ParameterDefinition{}, errors.Errorf("unsupported type for default value: %T", param.Default)
			}
		} else {
			switch k := reflect.ValueOf(param.Default).Kind(); k {
			case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Float32, reflect.Float64:
				out.Default = param.Default
			default:
				return ParameterDefinition{}, errors.Errorf("unsupported type for default value: %T", param.Default)
			}
		}
	}

	out.Required.value = pointers.Bool(!param.Constraints.Optional)

	out.Regex = param.Constraints.Regex

	if len(param.Constraints.Options) > 0 {
		out.Options = make([]OptionDefinition, len(param.Constraints.Options))
		for i, opt := range param.Constraints.Options {
			if param.Type == "configvar" {
				switch k := reflect.ValueOf(opt.Value).Kind(); k {
				case reflect.Map:
					configName, err := extractConfigVarName(opt.Value)
					if err != nil {
						return ParameterDefinition{}, errors.Wrap(err, "invalid configvar option")
					}
					out.Options[i] = OptionDefinition{
						Label: opt.Label,
						Value: configName,
					}
				default:
					return ParameterDefinition{}, errors.Errorf("unhandled option type: %s", k)
				}
			} else {
				switch k := reflect.ValueOf(opt.Value).Kind(); k {
				case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
					reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
					reflect.Float32, reflect.Float64:
					out.Options[i] = OptionDefinition{
						Label: opt.Label,
						Value: opt.Value,
					}
				default:
					return ParameterDefinition{}, errors.Errorf("unhandled option type: %s", k)
				}
			}
		}
	}

	return out, nil
}

// Converts a list of parameters from the format used by our API into the format used by definition files.
//
// Can be inverted by convertParametersDefToAPI.
func convertParametersAPIToDef(params []api.Parameter) ([]ParameterDefinition, error) {
	out := []ParameterDefinition{}
	for _, param := range params {
		converted, err := convertParameterAPIToDef(param)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func extractConfigVarName(v interface{}) (string, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return "", errors.Errorf("expected map but got %T", v)
	}
	if airplaneType, ok := m["__airplaneType"]; !ok || airplaneType != "configvar" {
		return "", errors.Errorf("expected __airplaneType=configvar but got %v", airplaneType)
	}
	if configName, ok := m["name"]; !ok {
		return "", errors.Errorf("missing name property from configvar type: %v", v)
	} else if configNameStr, ok := configName.(string); !ok {
		return "", errors.Errorf("expected name to be string but got %T", configName)
	} else {
		return configNameStr, nil
	}
}
