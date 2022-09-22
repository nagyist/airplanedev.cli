package resource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/server/state"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/pkg/errors"
)

// GenerateAliasToResourceMap generates a mapping from alias to resource - resourceAttachments is a mapping from alias
// to slug, and slugToResource is a mapping from slug to resource, and so we just link the two.
func GenerateAliasToResourceMap(
	ctx context.Context,
	state *state.State,
	resourceAttachments map[string]string,
	slugToResource map[string]env.ResourceWithEnv,
) (map[string]resources.Resource, error) {
	aliasToResourceMap := map[string]resources.Resource{}
	// We only need to generate entries in the map for resources that are explicitly attached to a task.
	for alias, slug := range resourceAttachments {
		var resource resources.Resource
		resourceWithEnv, ok := slugToResource[slug]
		if !ok {
			return nil, errors.Errorf("Cannot find resource with slug %s in dev config file or remotely", slug)
		} else if resourceWithEnv.Remote {
			remoteResourceWithCredentials, err := state.CliConfig.Client.GetResource(ctx, api.GetResourceRequest{
				Slug:                 slug,
				EnvSlug:              state.EnvSlug,
				IncludeSensitiveData: true,
			})
			if err != nil {
				return nil, errors.Wrap(err, "getting resource with credentials")
			}
			resource = remoteResourceWithCredentials.ExportResource
		} else {
			resource = resourceWithEnv.Resource
		}

		aliasToResourceMap[alias] = resource
	}

	return aliasToResourceMap, nil
}

// MergeRemoteResources merges the resources defined in the dev config file with remote resources from the env passed
// in the local dev server on startup.
func MergeRemoteResources(ctx context.Context, state *state.State) (map[string]env.ResourceWithEnv, error) {
	mergedResources := make(map[string]env.ResourceWithEnv)
	if state == nil {
		return mergedResources, nil
	}

	for slug, res := range state.DevConfig.Resources {
		mergedResources[slug] = res
	}

	if state.EnvID != env.LocalEnvID {
		remoteResources, err := ListRemoteResources(ctx, state)
		if err != nil {
			return nil, errors.Wrap(err, "listing remote resources")
		}

		for _, res := range remoteResources {
			if _, ok := mergedResources[res.Slug]; !ok {
				mergedResources[res.Slug] = env.ResourceWithEnv{
					Resource: res.ExportResource,
					Remote:   true,
				}
			}
		}
	}

	return mergedResources, nil
}

func ListRemoteResources(ctx context.Context, state *state.State) ([]libapi.Resource, error) {
	resp, err := state.CliConfig.Client.ListResources(ctx, state.EnvSlug)
	if err != nil {
		return nil, err
	}
	return resp.Resources, nil
}

// Creates a map of the resource alias to resource ID
func GenerateResourceAliasToID(resourceAliases map[string]resources.Resource) map[string]string {
	resourceAliasToID := map[string]string{}
	for alias, resource := range resourceAliases {
		resourceAliasToID[alias] = resource.ID()
	}
	return resourceAliasToID
}

func KindConfigToMap(r kind_configs.InternalResource) (map[string]interface{}, error) {
	b, err := json.Marshal(r.KindConfig)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling KindConfig")
	}
	kindConfig := map[string]interface{}{}
	if err := json.Unmarshal(b, &kindConfig); err != nil {
		return nil, errors.Wrap(err, "unmarshaling KindConfig")
	}
	return kindConfig, nil
}

func ResourceSlugFromID(id string) (string, error) {
	if len(id) <= 4 {
		return "", errors.New("id must be of the form res-{slug}")
	}

	return id[4:], nil
}

func ResourceIDFromSlug(slug string) string {
	return fmt.Sprintf("res-%s", slug)
}
