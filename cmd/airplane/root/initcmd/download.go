package initcmd

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/build/clibuild"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// initializeCodeWorkspace bootstraps the given workspace directory with all the entity code that belongs to a user's
// team.
func initializeCodeWorkspace(ctx context.Context, cfg config) error {
	resp, err := cfg.client.GenerateSignedURLs(ctx, cfg.envSlug)
	if err != nil {
		return err
	}
	urls := resp.SignedURLs

	if err := os.Mkdir(cfg.workspace, 0755); err != nil {
		if !os.IsExist(err) {
			return errors.Wrap(err, "creating workspace directory")
		}
	}

	var g errgroup.Group
	for _, url := range urls {
		url := url // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			return downloadAndExtractBundle(cfg.workspace, url)
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{WithLoader: true})
	defer l.StopLoader()
	d := build.BundleDiscoverer(cfg.client, l, "")
	bundles, err := d.Discover(ctx, cfg.workspace)
	if err != nil {
		return err
	}

	for _, bundle := range bundles {
		if err := dev.InstallBundleDependencies(bundle); err != nil {
			return err
		}
	}
	return nil
}

// downloadAndExtractBundle downloads a bundle from the given URL and extracts it to a subdirectory in the given
// workspace directory.
func downloadAndExtractBundle(workspace, url string) error {
	tarball, err := os.CreateTemp("", "archive-*.tar.gz")
	if err != nil {
		return errors.Wrap(err, "creating temp tarball file")
	}
	defer os.RemoveAll(tarball.Name())

	// Download from the signed URL.
	//nolint: noctx
	resp, err := http.Get(url)
	if err != nil {
		return errors.Wrap(err, "getting bundle data")
	}
	defer resp.Body.Close()

	_, err = io.Copy(tarball, resp.Body)
	if err != nil {
		return errors.Wrap(err, "copying content into tarball")
	}

	// TODO: Swap this out when we start naming bundles on the API-side.
	bundleName := utils.GenerateID("bundle_")
	bundleDir := filepath.Join(workspace, bundleName)
	// Create the bundle directory where we'll extract the tarball.
	if err := os.Mkdir(bundleDir, 0755); err != nil {
		return err
	}

	// Seek back to the beginning of the tar file.
	if _, err := tarball.Seek(0, 0); err != nil {
		return err
	}

	// Extract the tarball into the bundle directory.
	if err = utils.Untar(filepath.Join(workspace, bundleName), tarball); err != nil {
		return errors.Wrap(err, "extracting tarball")
	}

	return nil
}
