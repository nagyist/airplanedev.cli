package cryptox

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"

	"github.com/pkg/errors"
)

// ComputeHashFromFiles computes a sha256 hash by concatenating the contents of the given paths and returning the checksum as a
// hex string.
func ComputeHashFromFiles(paths ...string) (string, error) {
	var b []byte

	for _, p := range paths {
		contents, err := os.ReadFile(p)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", errors.Wrapf(err, "reading %s", p)
		}

		b = append(b, contents...)
	}

	sha := sha256.Sum256(b)
	return hex.EncodeToString(sha[:]), nil
}
