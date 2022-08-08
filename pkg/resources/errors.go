package resources

import "fmt"

type ErrResourceNotFound struct {
	Slug string
}

var _ error = &ErrResourceNotFound{}

func (e ErrResourceNotFound) Error() string {
	return fmt.Sprintf("ErrResourceNotFound: Resource %s not found", e.Slug)
}

func NewErrResourceNotFound(slug string) ErrResourceNotFound {
	return ErrResourceNotFound{slug}
}

type ErrUnsupportedResourceVersion struct {
	Version string
}

var _ error = &ErrUnsupportedResourceVersion{}

func (e ErrUnsupportedResourceVersion) Error() string {
	return fmt.Sprintf("Unsupported AIRPLANE_RESOURCES_VERSION: %q", e.Version)
}

func NewErrUnsupportedResourceVersion(version string) ErrUnsupportedResourceVersion {
	return ErrUnsupportedResourceVersion{version}
}

type ErrMissingResourceField struct {
	Field string
}

var _ error = &ErrMissingResourceField{}

func (e ErrMissingResourceField) Error() string {
	return fmt.Sprintf("Missing resource field: %s", e.Field)
}

func NewErrMissingResourceField(field string) ErrMissingResourceField {
	return ErrMissingResourceField{field}
}
