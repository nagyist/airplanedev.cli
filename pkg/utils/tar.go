package utils

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// Untar extracts a tarball (in the form of a Reader) to the specified folder. Based on https://medium.com/@skdomino/taring-untaring-files-in-go-6b07cf56bc07.
func Untar(dst string, r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		// Detect zip slip vulnerability: https://security.snyk.io/research/zip-slip-vulnerability
		if strings.Contains(header.Name, "..") {
			return errors.New("tarball may not contain directory traversal elements (`..`)")
		}

		target := filepath.Join(dst, header.Name)
		// Check header block for content type
		switch header.Typeflag {
		case tar.TypeDir: // Directory
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}
		case tar.TypeReg: // File
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// Manually close here after each file operation, as defering would cause each file close  to wait until all
			// operations have completed.
			f.Close()
		}
	}
}
