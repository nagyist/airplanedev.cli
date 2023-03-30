package builtins

import (
	"fmt"
	"strings"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/pkg/errors"
)

const builtinsSlugPrefix = "airplane"

// Specification of a function via namespace and name. Each function should have a unique namespace + name combination.
type FunctionSpecification struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type FunctionKey string

func (fs FunctionSpecification) Key() FunctionKey {
	return FunctionKey(fs.String())
}

func (fs FunctionSpecification) String() string {
	return fmt.Sprintf("%s.%s", fs.Namespace, fs.Name)
}

func IsBuiltinTaskSlug(slug string) bool {
	_, err := GetBuiltinFunctionSpecification(slug)
	return err == nil
}

// Returns an unvalidated function specification if it matches the format of
// airplane:<namespace>_<name>.
func GetBuiltinFunctionSpecification(slug string) (FunctionSpecification, error) {
	// Validate function specification and schema
	slugParts := strings.Split(slug, ":")
	if len(slugParts) != 2 || slugParts[0] != builtinsSlugPrefix {
		return FunctionSpecification{}, fmt.Errorf("unknown builtin task slug: %s", slug)
	}
	namespaceParts := strings.Split(slugParts[1], "_")
	if len(namespaceParts) != 2 { // This may not be true in the future
		return FunctionSpecification{}, fmt.Errorf("unknown builtin task slug: %s", slug)
	}
	return FunctionSpecification{
		Namespace: namespaceParts[0],
		Name:      namespaceParts[1],
	}, nil
}

// Returns a FunctionSpecification from a set of KindOptions. Expects a `functionSpecification` key
// at the top level to contain a function specification serialized as a map[string]interface{}.
func GetFunctionSpecificationFromKindOptions(kindOptions buildtypes.KindOptions) (FunctionSpecification, error) {
	var out FunctionSpecification
	fs, ok := kindOptions["functionSpecification"]
	if !ok {
		return out, errors.New("Missing function specification from builtin KindOptions")
	}
	fsMap, ok := fs.(map[string]interface{})
	if !ok {
		return out, errors.Errorf("expected map function specification, got %T instead", fs)
	}

	if v, ok := fsMap["namespace"]; ok {
		if sv, ok := v.(string); ok {
			out.Namespace = sv
		} else {
			return out, errors.Errorf("expected string namespace, got %T instead", v)
		}
	} else {
		return out, errors.New("missing namespace from function specification")
	}

	if v, ok := fsMap["name"]; ok {
		if sv, ok := v.(string); ok {
			out.Name = sv
		} else {
			return out, errors.Errorf("expected string name, got %T instead", v)
		}
	} else {
		return out, errors.New("missing namespace from function specification")
	}

	return out, nil
}
