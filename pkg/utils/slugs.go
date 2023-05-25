package utils

import (
	"fmt"
	"strings"

	"github.com/gosimple/slug"
	"github.com/pkg/errors"
)

func init() {
	slug.MaxLength = 50
}

// Make returns a slug generated from the provided string.
func MakeSlug(s string) string {
	// We prefer underscores over dashes since they are easier
	// to use in Go templates.
	return strings.ReplaceAll(slug.Make(s), "-", "_")
}

// IsSlug returns True if the provided text does not contain whitespace
// characters, punctuation, uppercase letters, and non-ASCII characters.
// It can contain `_`, but not at the beginning or end of the text.
// It should be of length <= to MaxLength.
// All output from MakeSlug(text) will pass this test.
func IsSlug(text string) bool {
	// The slug library will accept text with `-`'s, so we need to add our own check.
	return slug.IsSlug(text) && !strings.Contains(text, "-")
}

type GetUniqueSlugRequest struct {
	Slug        string
	SlugExists  func(string) (bool, error)
	MaxAttempts int
}

func GetUniqueSlug(req GetUniqueSlugRequest) (string, error) {
	if req.MaxAttempts == 0 {
		req.MaxAttempts = 10
	}

	// Make sure provided slug is actually a slug.
	baseSlug := MakeSlug(req.Slug)
	slug := baseSlug
	for attempts := 0; attempts < req.MaxAttempts; attempts += 1 {
		if exists, err := req.SlugExists(slug); err != nil {
			return "", err
		} else if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s_%s", baseSlug, RandomString(3, CharsetAlphaLowercase))
	}
	return "", errors.New("Too many attempts at getting a unique slug")
}
