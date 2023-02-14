package build

import (
	_ "embed"
	"encoding/json"

	"github.com/pkg/errors"
)

// If you change the versions in this file, make sure to:
//
//  1. Update the digest to match. The tags are just a convenience to note
//     which version the digest correlates to without consulting DockerHub. The
//     digest should be for the multi-platform version of the image- run
//     'docker buildx imagetools inspect [uri]' to get the digest. Note that this is
//     different from the (single platform) digest that is shown in the Dockerhub UI
//     or that you see when pulling the image locally.
//  2. Run the copy_cache_images.py script to update the images in our public cache.
//  3. Alpine-based images will not work with shim-based builders, but it's
//     a straightforward change if we end up wanting it (different echo
//     semantics than debian-based images). These base images are cached on
//     our agents, so for the most part, we don't need to worry about the
//     size of the base image.
//
//go:embed versions.json
var versionsJSON []byte

// Versions contains a mapping table of (builder, version) to
// (node, tag, digest) image tuples. The digests are always for
// images built for the linux/amd64 architecture.
//
// This lookup table is used to construct Dockerfiles that always
// pull from the most-up-date version of the underlying base image
// based on what we have cached in the Airplane registry for customers.
type Versions map[string]map[string]Version

type Version struct {
	Image  string `json:"image"`
	Tag    string `json:"tag"`
	Digest string `json:"digest"`
}

func (v Version) String() string {
	if v.Image == "" || v.Digest == "" {
		return ""
	}

	return v.Image + "@" + v.Digest
}

func GetVersions() (Versions, error) {
	var versions Versions
	if err := json.Unmarshal(versionsJSON, &versions); err != nil {
		return Versions{}, errors.Wrap(err, "unmarshalling versions.json")
	}

	return versions, nil
}

func GetVersion(builder Name, version string, slim bool) (Version, error) {
	versions, err := GetVersions()
	if err != nil {
		return Version{}, err
	}

	builderVersions, ok := versions[string(builder)]
	if !ok {
		return Version{}, errors.Errorf("unknown builder: %s", builder)
	}

	var versionKey string
	if slim {
		versionKey = version + "-slim"
	} else {
		versionKey = version
	}

	return builderVersions[versionKey], nil
}
