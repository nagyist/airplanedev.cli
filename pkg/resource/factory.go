package resource

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

type ResourceFactory func(map[string]interface{}) (Resource, error)

var resourceFactories = map[ResourceKind]ResourceFactory{}

func RegisterResourceFactory(kind ResourceKind, factory ResourceFactory) {
	if _, ok := resourceFactories[kind]; ok {
		panic(fmt.Sprintf("resource factory already registered for kind %s", kind))
	}
	resourceFactories[kind] = factory
}

func RegisterBaseResourceFactory(kind ResourceKind, makeResource func() Resource) {
	if _, ok := resourceFactories[kind]; ok {
		panic(fmt.Sprintf("resource factory already registered for kind %s", kind))
	}
	resourceFactories[kind] = func(serialized map[string]interface{}) (Resource, error) {
		resource := makeResource()
		if err := BaseFactory(serialized, &resource); err != nil {
			return nil, err
		}
		return resource, nil
	}
}

func GetResource(kind ResourceKind, serialized map[string]interface{}) (Resource, error) {
	factory, ok := resourceFactories[kind]
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
