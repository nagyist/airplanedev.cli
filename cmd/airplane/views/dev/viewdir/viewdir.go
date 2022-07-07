package viewdir

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/mitchellh/hashstructure/v2"
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

func (this *ViewDirectory) CacheDir() string {
	hash, err := hashstructure.Hash(this, hashstructure.FormatV2, nil)
	if err != nil {
		logger.Log("error with hashing viewdir, using default hash value: %d", hash)
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("airplane-view-%d", hash))
}

func NewViewDirectory(root string) ViewDirectory {
	return ViewDirectory{
		root:           root,
		entrypointPath: "App.tsx",
	}
}
