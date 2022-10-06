package env

import (
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/lib/pkg/resources"
)

const LocalEnvID = "local"

// ResourceWithEnv store information about a resource and whether it's remote.
type ResourceWithEnv struct {
	// The configuration for the resource.
	Resource resources.Resource
	// Whether the resource's configuration is remote or local.
	Remote bool
}

// NewLocalEnv returns a new environment struct with special, local fields.
func NewLocalEnv() api.Env {
	return api.Env{
		ID:   LocalEnvID,
		Slug: LocalEnvID,
		Name: LocalEnvID,
	}
}
