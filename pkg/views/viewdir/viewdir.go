package viewdir

import (
	"context"
	"os"
	"path/filepath"

	libapi "github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	"github.com/airplanedev/cli/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/pkg/errors"
)

type ViewDirectoryInterface interface {
	Root() string
	EntrypointPath() string
	Slug() string
}

type ViewDirectory struct {
	root           string
	entrypointPath string
	slug           string
}

func (this *ViewDirectory) Root() string {
	return this.root
}

func (this *ViewDirectory) EntrypointPath() string {
	return this.entrypointPath
}
func (this *ViewDirectory) Slug() string {
	return this.slug
}

func missingViewHandler(ctx context.Context, defn definitions.ViewDefinition) (*libapi.View, error) {
	// TODO(zhan): generate view?
	return &libapi.View{
		ID:   "temp",
		Slug: defn.Slug,
	}, nil
}

func NewViewDirectory(ctx context.Context, client api.APIClient, searchPath string, envSlug string) (ViewDirectory, error) {
	d := &discover.Discoverer{
		ViewDiscoverers: []discover.ViewDiscoverer{
			&discover.ViewDefnDiscoverer{
				Client:             client,
				Logger:             logger.NewStdErrLogger(logger.StdErrLoggerOpts{}),
				MissingViewHandler: missingViewHandler,
			},
			&discover.CodeViewDiscoverer{
				Client:             client,
				Logger:             logger.NewStdErrLogger(logger.StdErrLoggerOpts{}),
				MissingViewHandler: missingViewHandler,
			},
		},
		EnvSlug: envSlug,
		Client:  client,
	}

	// If pointing towards a view definition file, we just use that file as the view to run.
	if definitions.IsViewDef(searchPath) {
		vc, err := d.ViewDiscoverers[0].GetViewConfig(ctx, searchPath)
		if err != nil {
			return ViewDirectory{}, errors.Wrap(err, "reading view config")
		}

		vd, err := NewViewDirectoryFromViewConfig(*vc)
		if err != nil {
			return ViewDirectory{}, err
		}

		return vd, nil
	}

	// If pointing towards a non-view-definition file, we use the directory around
	// that as our search path.
	fileInfo, err := os.Stat(searchPath)
	if err != nil {
		return ViewDirectory{}, errors.Wrapf(err, "describing %s", searchPath)
	}
	if !fileInfo.IsDir() {
		searchPath = filepath.Dir(searchPath)
	}

	// We try to find a single view in our search path. If there isn't exactly
	// one view, we error out.
	_, viewConfigs, err := d.Discover(ctx, searchPath)
	if err != nil {
		return ViewDirectory{}, errors.Wrap(err, "discovering view configs")
	}
	if len(viewConfigs) > 1 {
		return ViewDirectory{}, errors.New("currently can only have at most one view!")
	} else if len(viewConfigs) == 0 {
		return ViewDirectory{}, errors.New("no views found!")
	}
	vc := viewConfigs[0]

	vd, err := NewViewDirectoryFromViewConfig(vc)
	if err != nil {
		return ViewDirectory{}, err
	}

	return vd, nil
}

// NewViewDirectoryFromViewConfig constructs a new ViewDirectory from a ViewConfig.
func NewViewDirectoryFromViewConfig(vc discover.ViewConfig) (ViewDirectory, error) {
	absRoot, err := filepath.Abs(vc.Root)
	if err != nil {
		return ViewDirectory{}, errors.Wrap(err, "getting absolute root filepath")
	}

	return ViewDirectory{
		root:           absRoot,
		entrypointPath: vc.Def.Entrypoint,
		slug:           vc.Def.Slug,
	}, nil
}
