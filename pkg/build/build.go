package build

import (
	"context"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/build"
)

type BuildCreator interface {
	CreateBuild(ctx context.Context, req Request) (*build.Response, error)
}

// Request represents a build request.
type Request struct {
	Client  api.APIClient
	Root    string
	Def     definitions.DefinitionInterface
	TaskID  string
	TaskEnv api.TaskEnv
	Shim    bool
	GitMeta api.BuildGitMeta
}

// Response represents a build response.
type Response struct {
	ImageURL string
	// Optional, only if applicable
	BuildID string
}
