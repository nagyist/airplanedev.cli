package build

import (
	"context"

	libapi "github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/build"
	"github.com/airplanedev/cli/pkg/definitions"
)

type BuildCreator interface {
	CreateBuild(ctx context.Context, req Request) (*build.Response, error)
}

// Request represents a build request.
type Request struct {
	Client  api.APIClient
	Root    string
	Def     definitions.Definition
	TaskID  string
	TaskEnv libapi.EnvVars
	Shim    bool
}

// Response represents a build response.
type Response struct {
	ImageURL string
	// Optional, only if applicable
	BuildID string
}
