package env

import (
	"github.com/airplanedev/cli/pkg/api"
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

type ConfigWithEnv struct {
	api.Config
	Remote bool       `json:"remote"`
	Env    libapi.Env `json:"env"`
}

// NewLocalEnv returns a new environment struct with special, local fields.
func NewLocalEnv() libapi.Env {
	return libapi.Env{
		ID:   LocalEnvID,
		Slug: LocalEnvID,
		Name: "Local",
	}
}
