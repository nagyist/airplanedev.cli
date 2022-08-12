package resources

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

type ResourceFactory func(map[string]interface{}) (Resource, error)

var ResourceFactories = map[ResourceKind]ResourceFactory{}

func RegisterResourceFactory(kind ResourceKind, factory ResourceFactory) {
	if _, ok := ResourceFactories[kind]; ok {
		panic(fmt.Sprintf("resource factory already registered for kind %s", kind))
	}
	ResourceFactories[kind] = factory
}

func RegisterBaseResourceFactory(kind ResourceKind, makeResource func() Resource) {
	if _, ok := ResourceFactories[kind]; ok {
		panic(fmt.Sprintf("resource factory already registered for kind %s", kind))
	}
	ResourceFactories[kind] = func(serialized map[string]interface{}) (Resource, error) {
		resource := makeResource()
		if err := BaseFactory(serialized, &resource); err != nil {
			return nil, err
		}
		return resource, nil
	}
}

func GetResource(kind ResourceKind, serialized map[string]interface{}) (Resource, error) {
	factory, ok := ResourceFactories[kind]
	if !ok {
		return nil, errors.Errorf("no factory for kind %s", kind)
	}

	return factory(serialized)
}

func BaseFactory(serialized map[string]interface{}, result interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "mapstructure",
		Result:  result,
	})
	if err != nil {
		return errors.Wrap(err, "error creating decoder")
	}
	if err := decoder.Decode(serialized); err != nil {
		return errors.Wrap(err, "error decoding resource")
	}
	return nil
}
