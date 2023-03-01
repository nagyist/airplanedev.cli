package build

import (
	"context"

	"github.com/airplanedev/cli/pkg/api"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
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
	TaskEnv libapi.TaskEnv
	Shim    bool
}

// Response represents a build response.
type Response struct {
	ImageURL string
	// Optional, only if applicable
	BuildID string
}
