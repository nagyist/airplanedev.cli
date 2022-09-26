package viewdir

import (
	"context"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
)

// TODO(zhan): this probably should be in airplanedev/lib.
type ViewDirectoryInterface interface {
	Root() string
	EntrypointPath() string
	CacheDir() string
}

type ViewDirectory struct {
	root           string
	entrypointPath string
}

func (this *ViewDirectory) Root() string {
	return this.root
}

func (this *ViewDirectory) EntrypointPath() string {
	return this.entrypointPath
}

func missingViewHandler(ctx context.Context, defn definitions.ViewDefinition) (*api.View, error) {
	// TODO(zhan): generate view?
	return &api.View{
		ID:   "temp",
		Slug: defn.Slug,
	}, nil
}

func FindRoot(fileOrDir string) (string, error) {
	var err error
	fileOrDir, err = filepath.Abs(fileOrDir)
	if err != nil {
		return "", errors.Wrap(err, "getting absolute path")
	}
	fileInfo, err := os.Stat(fileOrDir)
	if err != nil {
		return "", errors.Wrapf(err, "describing %s", fileOrDir)
	}
	if !fileInfo.IsDir() {
		fileOrDir = filepath.Dir(fileOrDir)
	}

	if p, ok := fsx.Find(fileOrDir, "package.json"); ok {
		return p, nil
	}
	return filepath.Dir(fileOrDir), nil
}

func NewViewDirectory(ctx context.Context, cfg *cli.Config, root string, searchPath string, envSlug string) (ViewDirectory, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ViewDirectory{}, errors.Wrap(err, "getting absolute root filepath")
	}

	d := &discover.Discoverer{
		ViewDiscoverers: []discover.ViewDiscoverer{
			&discover.ViewDefnDiscoverer{
				Client:             cfg.Client,
				Logger:             logger.NewStdErrLogger(logger.StdErrLoggerOpts{}),
				MissingViewHandler: missingViewHandler,
			},
			&discover.CodeViewDiscoverer{
				Client:             cfg.Client,
				Logger:             logger.NewStdErrLogger(logger.StdErrLoggerOpts{}),
				MissingViewHandler: missingViewHandler,
			},
		},
		EnvSlug: envSlug,
		Client:  cfg.Client,
	}

	// If pointing towards a view definition file, we just use that file as the view to run.
	if definitions.IsViewDef(searchPath) {
		vc, err := d.ViewDiscoverers[0].GetViewConfig(ctx, searchPath)
		if err != nil {
			return ViewDirectory{}, errors.Wrap(err, "reading view config")
		}

		return ViewDirectory{
			root:           absRoot,
			entrypointPath: vc.Def.Entrypoint,
		}, nil
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

	return ViewDirectory{
		root:           absRoot,
		entrypointPath: vc.Def.Entrypoint,
	}, nil
}
