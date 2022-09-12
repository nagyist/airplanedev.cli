package resources

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

type BaseResource struct {
	Kind ResourceKind `json:"kind"`
	ID   string       `json:"id"`
	Slug string       `json:"slug"`
	Name string       `json:"name"`
}

func (r *BaseResource) Update(update BaseResource) {
	if update.Kind != "" {
		r.Kind = update.Kind
	}
	if update.ID != "" {
		r.ID = update.ID
	}
	if update.Slug != "" {
		r.Slug = update.Slug
	}
	if update.Name != "" {
		r.Name = update.Name
	}
}

type ResourceKind string

type EnvFactory func(ref string) (Resource, error)

func GetAirplaneEnv(ref string, name string) string {
	return GetAirplaneEnvFromFunc(ref, name, os.LookupEnv)
}

func GetAirplaneEnvFromFunc(ref string, name string, f EnvLookupFunc) string {
	key := fmt.Sprintf("AIRPLANE_%s_%s", strings.ToUpper(ref), strings.ToUpper(name))
	val, _ := f(key)
	return val
}

func AirplaneResourceFromFunc(ref string, f EnvLookupFunc, res Resource) error {
	version, ok := f("AIRPLANE_RESOURCES_VERSION")
	if !ok {
		version = "1"
	}

	switch version {
	case "1":
		serializedResources, ok := f("AIRPLANE_RESOURCES")
		if !ok {
			return NewErrResourceNotFound(ref)
		}

		resources := map[string]interface{}{}
		err := json.Unmarshal([]byte(serializedResources), &resources)
		if err != nil {
			return errors.Wrap(err, "error unmarshalling AIRPLANE_RESOURCES")
		}

		r, ok := resources[ref]
		if !ok {
			return NewErrResourceNotFound(ref)
		}

		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			TagName: "json",
			Result:  res,
		})
		if err != nil {
			return errors.Wrap(err, "error creating decoder")
		}
		if err := decoder.Decode(r); err != nil {
			return errors.Wrap(err, "error decoding resource")
		}
		return nil
	default:
		return NewErrUnsupportedResourceVersion(version)
	}
}

func GetAirplaneResourceFromFunc(ref string, f EnvLookupFunc) (Resource, error) {
	version, _ := f("AIRPLANE_RESOURCES_VERSION")
	switch version {
	case "2":
		serializedResources, ok := f("AIRPLANE_RESOURCES")
		if !ok {
			return nil, NewErrResourceNotFound(ref)
		}

		resources := map[string]map[string]interface{}{}
		err := json.Unmarshal([]byte(serializedResources), &resources)
		if err != nil {
			return nil, errors.Wrap(err, "error unmarshalling AIRPLANE_RESOURCES")
		}

		r, ok := resources[ref]
		if !ok {
			return nil, NewErrResourceNotFound(ref)
		}

		if kind, ok := r["kind"]; ok {
			if kindStr, ok := kind.(string); ok {
				return GetResource(ResourceKind(kindStr), r)
			} else {
				return nil, errors.Errorf("expected kind type string, got %T", r["kind"])
			}
		} else {
			return nil, errors.New("missing kind property in resource")
		}
	default:
		return nil, NewErrUnsupportedResourceVersion(version)
	}
}

func EnvFactoryFromFunc(rf func(string, EnvLookupFunc) (Resource, error), f EnvLookupFunc) EnvFactory {
	return func(ref string) (Resource, error) {
		return rf(ref, f)
	}
}

type Resource interface {
	ScrubSensitiveData()
	Update(r Resource) error
	Validate() error
	Kind() ResourceKind
	String() string
	ID() string
	UpdateBaseResource(r BaseResource) error
}

type EnvLookupFunc func(string) (string, bool)
