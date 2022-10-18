package env

import (
	libapi "github.com/airplanedev/lib/pkg/api"
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
func NewLocalEnv() libapi.Env {
	return libapi.Env{
		ID:   LocalEnvID,
		Slug: LocalEnvID,
		Name: "Local",
	}
}
