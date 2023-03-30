package parameters

import (
	"reflect"
	"strings"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/pkg/errors"
)

func SanitizeParamValues(values map[string]interface{}, params api.Parameters) (map[string]interface{}, error) {
	schema := map[string]*api.Parameter{}
	for i, param := range params {
		schema[param.Slug] = &params[i]
	}
	sanitized, err := sanitizeMapValues(values, schema)
	if err != nil {
		return nil, err
	}
	return sanitized, nil
}

func sanitizeMapValues(values map[string]interface{}, schema map[string]*api.Parameter) (map[string]interface{}, error) {
	if values == nil {
		return nil, nil
	}
	sanitized := map[string]interface{}{}
	for k, v := range values {
		if isSecretValue(k, schema[k]) {
			sanitized[k] = getZeroValue(v)
		} else if reflect.TypeOf(v) != nil && reflect.TypeOf(v).Kind() == reflect.Map {
			mapV, ok := v.(map[string]interface{})
			if !ok {
				return nil, errors.New("unable to cast value to map")
			}
			sanitizedV, err := sanitizeMapValues(mapV, nil)
			if err != nil {
				return nil, err
			}
			sanitized[k] = sanitizedV
		} else {
			sanitized[k] = v
		}
	}
	return sanitized, nil
}

var secretWords = []string{
	"token",
	"authorization",
	"secret",
	"password",
	"apikey",
	"api_key",
	"api key",
	"api-key",
}

func isSecretValue(key string, param *api.Parameter) bool {
	toCheck := []string{strings.ToLower(key)}
	if param != nil {
		toCheck = append(toCheck, strings.ToLower(param.Name))
	}
	for _, secret := range secretWords {
		for _, s := range toCheck {
			if strings.Contains(s, secret) {
				return true
			}
		}
	}
	return false
}

func getZeroValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Bool {
		return false
	} else if val.CanInt() {
		return 0
	} else if val.Kind() == reflect.String {
		return "****"
	} else if val.Kind() == reflect.Array || val.Kind() == reflect.Slice {
		return []interface{}{}
	} else if val.Kind() == reflect.Map {
		return map[string]interface{}{}
	}
	return nil
}
