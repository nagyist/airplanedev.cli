package env

import (
	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/airplanedev/cli/pkg/cli/resources"
)

const StudioEnvID = "studio"

// ResourceWithEnv store information about a resource and whether it's remote.
type ResourceWithEnv struct {
	// The configuration for the resource.
	Resource resources.Resource
	// Whether the resource's configuration is remote or local.
	Remote bool
}

type ConfigWithEnv struct {
	api.Config
	Remote bool       `json:"remote"`
	Env    libapi.Env `json:"env"`
}

// NewLocalEnv returns a new environment struct with special, local fields.
func NewLocalEnv() libapi.Env {
	return NewStudioEnv("Local")
}

// NewStudioEnv returns a new environment struct with studio-specific fields.
func NewStudioEnv(name string) libapi.Env {
	return libapi.Env{
		ID:   StudioEnvID,
		Slug: StudioEnvID,
		Name: name,
	}
}
