// Package resource exists because we need the resource structs for (un)marshalling the dev config file. These structs
// should eventually be part of their own repo
package resource

import (
	"github.com/pkg/errors"
)

type ResourceKind string

type Resource interface {
	Kind() ResourceKind
}

type BaseResource struct {
	Kind ResourceKind `json:"kind" yaml:"kind"`
	ID   string       `json:"id" yaml:"id"`
	Slug string       `json:"slug" yaml:"slug"`
	Name string       `json:"name" yaml:"name"`
}

// GenerateAliasToResourceMap generates a mapping from alias to resource - resourceAttachments is a mapping from alias
// to slug, and slugToResource is a mapping from slug to resource, and so we just link the two.
func GenerateAliasToResourceMap(
	resourceAttachments map[string]string,
	slugToResource map[string]Resource,
) (map[string]Resource, error) {
	aliasToResourceMap := map[string]Resource{}
	// We only need to generate entries in the map for resources that are explicitly attached to a task.
	for alias, slug := range resourceAttachments {
		resource, ok := slugToResource[slug]
		if !ok {
			// TODO: Augment error message with airplane subcommand to add resource
			return nil, errors.Errorf("Cannot find resource with slug %s in dev config file", slug)
		}
		aliasToResourceMap[alias] = resource
	}

	return aliasToResourceMap, nil
}
