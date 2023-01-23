package utils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// Zip packages the given directory into a zip file. Modified from https://github.com/golang/go/issues/54898#issuecomment-1239570464
// TODO(@justin): Add tests for this.
func Zip(w io.Writer, dir string, include func(filePath string, info os.FileInfo) (bool, error)) error {
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	f := os.DirFS(dir)

	// AddFS creates a zip archive of a directory by writing files from f to w.
	if err := fs.WalkDir(f, ".", func(path string, d fs.DirEntry, err error) error {
		// Error with path.
		if err != nil {
			return err
		}

		// Skip ignored files and _all_ directories. Subdirectories are auto-generated when add files to the zip
		// archive.
		info, err := d.Info()
		if err != nil {
			return errors.Wrap(err, "getting file info")
		}

		absPath := filepath.Join(dir, path)
		if ok, err := include(absPath, info); err != nil {
			return errors.Wrap(err, "checking if file should be included")
		} else if info.IsDir() {
			// We don't ever add directories to the zip archive since they are implicitly added, but if the directory
			// is explictily skipped, we also don't want to traverse its contents.
			if ok {
				return nil
			}
			return fs.SkipDir
		}

		// Handle formatting path name properly for use in zip file. Paths must
		// use forward slashes, even on Windows.
		// See: https://pkg.go.dev/archive/zip#Writer.Create
		//
		// Directories are created automatically based on the subdirectories provided
		// in each file's path.
		path = filepath.ToSlash(path)

		// Open the path to read from.
		f, err := f.Open(path)
		if err != nil {
			return errors.Wrap(err, "opening file")
		}
		defer f.Close()

		// Create the file in the zip.
		w, err := zipWriter.Create(path)
		if err != nil {
			return errors.Wrap(err, "adding file to zip")
		}

		// Write the source file into the zip at path noted in Create().
		_, err = io.Copy(w, f)
		if err != nil {
			return errors.Wrap(err, "copying file")
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "zipping directory")
	}

	return nil
}

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
