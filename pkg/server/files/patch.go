package files

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/pkg/errors"
)

func BatchPatch(s *state.State, patches []string) error {
	for _, patch := range patches {
		if err := Patch(s, patch); err != nil {
			return err
		}
	}
	return nil
}

// Patch applies a unified diff patch. The patch should contain a preamble that describe the file we're patching, and so
// we can infer exactly the changes that the patch will cause.
func Patch(s *state.State, patch string) error {
	patchReader := strings.NewReader(patch)

	// diffs is a slice of *gitdiff.File describing the diffs included in the patch. This will error if the patch is
	// malformed.
	diffs, _, err := gitdiff.Parse(patchReader)
	if err != nil {
		return errors.Wrap(err, "parsing patch")
	}

	// A patch may contain multiple diffs. We apply each diffs in the patch.
	for _, diff := range diffs {
		// diff.OldName is the path of the file we're patching, which should be relative to the dev server root. This
		// also ensures that we're not patching files outside the dev server root.
		// TODO: Handle the case when old name and new name differ (i.e. we're renaming files).
		path := filepath.Join(s.Dir, diff.OldName)

		f, err := os.OpenFile(path, os.O_RDWR, 0755)
		if err != nil {
			return errors.Wrap(err, "opening file")
		}

		// apply the changes in the patch to a source diff and store it in a buffer.
		var output bytes.Buffer
		err = gitdiff.Apply(&output, f, diff)
		if err != nil {
			return errors.Wrap(err, "applying patch")
		}

		// write the new contents to the file
		if _, err := f.Write(output.Bytes()); err != nil {
			return errors.Wrap(err, "writing new contents")
		}

		if err := f.Close(); err != nil {
			return errors.Wrap(err, "closing file")
		}
	}

	return nil
}
