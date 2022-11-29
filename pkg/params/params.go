// This package provides utilities for translating between
// inputs (entered via CLI) and api values (representation to API)
package params

import (
	"strconv"
	"strings"
	"time"

	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/pkg/errors"
)

const (
	YesString = "Yes"
	NoString  = "No"
)

// ValidateInput checks that string from CLI fits into expected API value
// This is best effort - API may still return a 400 even with valid inputs
func ValidateInput(param libapi.Parameter, in string) error {
	// Treat empty value as valid - optional/required is checked separately.
	if in == "" {
		return nil
	}

	switch param.Type {
	case libapi.TypeString:
		return nil

	case libapi.TypeBoolean:
		if _, err := ParseBool(in); err != nil {
			return errors.New("expected yes, no, true, false, 1 or 0")
		}

	case libapi.TypeInteger:
		if _, err := strconv.Atoi(in); err != nil {
			return errors.New("invalid integer")
		}

	case libapi.TypeFloat:
		if _, err := strconv.ParseFloat(in, 64); err != nil {
			return errors.New("invalid number")
		}

	case libapi.TypeUpload:
		if in != "" {
			// TODO(amir): we need to support them with some special
			// character perhaps `@` like curl?
			return errors.New("uploads are not supported from the CLI")
		}

	case libapi.TypeDate:
		if _, err := time.Parse("2006-01-02", in); err != nil {
			return errors.New("expected to be formatted as '2016-01-02'")
		}
	case libapi.TypeDatetime:
		if _, err := time.Parse(time.RFC3339, in); err != nil {
			return errors.Errorf("expected to be formatted as '%s'", time.RFC3339)
		}
		return nil
	}
	return nil
}

// ParseInput converts a string entered from CLI into the API value
// Handles deafult values when in is empty
func ParseInput(param libapi.Parameter, in string) (interface{}, error) {
	if in == "" {
		return param.Default, nil
	}
	switch param.Type {
	case libapi.TypeString, libapi.TypeDate, libapi.TypeDatetime:
		return in, nil

	case libapi.TypeBoolean:
		return ParseBool(in)

	case libapi.TypeInteger:
		v, err := strconv.Atoi(in)
		if err != nil {
			return nil, errors.Wrap(err, "atoi")
		}
		return v, nil

	case libapi.TypeFloat:
		v, err := strconv.ParseFloat(in, 64)
		if err != nil {
			return nil, errors.Wrap(err, "parsefloat")
		}
		return v, nil

	case libapi.TypeUpload:
		// TODO: ideally we read the file input here for API
		if in != "" {
			return nil, errors.New("uploads are not supported from the CLI")
		}
		return nil, nil

	case libapi.TypeConfigVar:
		return map[string]interface{}{
			"__airplaneType": "configvar",
			"name":           in,
		}, nil

	default:
		return in, nil
	}
}

// Light wrapper around strconv.ParseBool with support for yes and no
func ParseBool(v string) (bool, error) {
	switch vl := strings.ToLower(v); vl {
	case "yes", "y":
		return true, nil
	case "no", "n":
		return false, nil
	default:
		return strconv.ParseBool(vl)
	}
}

// Converts value from API to an input string (e.g. for a default CLI value)
// For example, bool `true` becomes `"Yes"` while strings, datetimes remain unchanged
func APIValueToInput(param libapi.Parameter, value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}

	switch param.Type {
	// For now, just use the original formatting on dates / datetimes
	case libapi.TypeString, libapi.TypeDate, libapi.TypeDatetime:
		v, ok := value.(string)
		if !ok {
			return "", errors.Errorf("could not cast %v to string", value)
		}
		return v, nil
	case libapi.TypeBoolean:
		v, ok := value.(bool)
		if !ok {
			return "", errors.Errorf("could not cast %v to bool", value)
		}
		if v {
			return YesString, nil
		} else {
			return NoString, nil
		}
	case libapi.TypeUpload:
		v, ok := value.(string)
		if !ok {
			return "", errors.Errorf("could not cast %v to string", value)
		}
		if v != "" {
			return "", errors.New("uploads not supported")
		}
		return "", nil
	case libapi.TypeInteger:
		// This is float64 from JSON inputs
		switch v := value.(type) {
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64), nil
		default:
			return "", errors.Errorf("could not cast %v to int or float64", value)
		}
	case libapi.TypeFloat:
		v, ok := value.(float64)
		if !ok {
			return "", errors.Errorf("could not cast %v to float64", value)
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return "", nil
	}
}
