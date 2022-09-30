package runtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvalTemplates(t *testing.T) {
	paramValues := map[string]interface{}{
		"limit":        23,
		"string_value": "hello world",
		"map_value":    map[string]string{"a": "b"},
	}
	configValues := map[string]interface{}{
		"API_KEY": 1234,
	}
	queryArgs := map[string]interface{}{
		"number":      "{{params.limit}}",
		"msg":         "{{ params.string_value }}",
		"map":         "{{ params.map_value }}",
		"config_test": "{{configs.API_KEY}}",
	}
	res, err := EvalTemplates(context.Background(), queryArgs, EvalGlobals{
		ParamValues: paramValues,
		Configs:     configValues,
	})
	require.NoError(t, err)
	expect := map[string]interface{}{
		"number":      23,
		"msg":         "hello world",
		"map":         map[string]string{"a": "b"},
		"config_test": 1234,
	}
	require.EqualValues(t, expect, res)
}

func TestEvalTemplatesInvalid(t *testing.T) {
	queryArgs := map[string]interface{}{
		"doesnt_exist":          "{{params.whatever}}",
		"doesnt_exist2":         "{{configs.whatever2}}",
		"doesnt_default_string": "{{doesnt_exist.whatever}}",
		"bad_template":          "{{bad.template}",
		"not_jst":               "{not.jst}}",
	}
	res, err := EvalTemplates(context.Background(), queryArgs, EvalGlobals{})
	require.NoError(t, err)
	expect := map[string]interface{}{
		"doesnt_exist":          nil,
		"doesnt_exist2":         nil,
		"doesnt_default_string": "", // the default if the top level config doesn't exist is a string
		"bad_template":          nil,
		"not_jst":               "{not.jst}}",
	}
	require.EqualValues(t, expect, res)
}

func TestEvalRunOptions(t *testing.T) {
	// SQL example
	sqlOpts := PrepareRunOptions{
		ParamValues: map[string]interface{}{"limit": 1},
		ConfigVars:  map[string]interface{}{"env": "production"},
		KindOptions: map[string]interface{}{
			"field1": "not_a_jst",
			"field2": 123,
			"queryArgs": map[string]interface{}{
				"limit":       "{{params.limit}}",
				"name":        "name_not_jst",
				"env":         "{{configs.env}}",
				"invalid_jst": "{{configs.doesnt_exist}}",
			},
		},
	}
	res, err := EvalRunOptionTemplates(context.Background(), sqlOpts)
	require.NoError(t, err)
	expectSQL := map[string]interface{}{
		"field1": "not_a_jst",
		"field2": 123,
		"queryArgs": map[string]interface{}{
			"limit":       1,
			"name":        "name_not_jst",
			"env":         "production",
			"invalid_jst": nil,
		},
	}
	require.EqualValues(t, expectSQL, res)

	// REST example
	restOpts := PrepareRunOptions{
		ParamValues: map[string]interface{}{"limit": 2},
		ConfigVars:  map[string]interface{}{"env": "production", "API_KEY": "abc123"},
		KindOptions: map[string]interface{}{
			"body": "hello world",
			"headers": map[string]interface{}{
				"header1": "some_header",
				"bearer":  "{{configs.API_KEY}}",
			},
			"urlParams": map[string]interface{}{
				"env":         "{{configs.env}}",
				"invalid_jst": "{{configs.doesnt_exist}}",
				"number":      "{{params.limit}}",
			},
		},
	}
	res, err = EvalRunOptionTemplates(context.Background(), restOpts)
	require.NoError(t, err)
	expectRest := map[string]interface{}{
		"body": "hello world",
		"headers": map[string]interface{}{
			"header1": "some_header",
			"bearer":  "abc123",
		},
		"urlParams": map[string]interface{}{
			"env":         "production",
			"invalid_jst": nil,
			"number":      2,
		},
	}
	require.EqualValues(t, expectRest, res)
}
