package resource

import (
	"context"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/server/state"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

const SlackID = "res00000000zteamslack"
const slackSlug = "team_slack"

// This is not guaranteed to be the slug of the demo db, but should be in all cases where demo db creation doesn't
// fail during team creation.
const DemoDBSlug = "demo_db"
const DemoDBName = "[Demo DB]"

var defaultRemoteResourceSlugs = []string{DemoDBSlug}
var defaultRemoteVirtualResourceSlugs = []string{slackSlug}

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
			var envSlug string
			// We load in some default remote resources (e.g. Slack) - in those cases, the remote flag will be true,
			// but the envID/slug will still be "local", which is not a valid remote environment. In these cases, we
			// keep the env slug empty, which will default to the user's team's default environment.
			if state.HasFallbackEnv() {
				envSlug = state.Env.Slug
			}
			remoteResourceWithCredentials, err := state.RemoteClient.GetResource(ctx, api.GetResourceRequest{
				Slug:                 slug,
				EnvSlug:              envSlug,
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

	// Add default virtual resources, which aren't returned by the request to list remote resources. Our web app doesn't
	// currently handle Slack resources, and so we just use the Slack resource at task run time.
	for _, slug := range defaultRemoteVirtualResourceSlugs {
		mergeDefaultRemoteResource(ctx, state, mergedResources, slug)
	}

	return mergedResources, nil
}

func mergeDefaultRemoteResource(
	ctx context.Context,
	state *state.State,
	mergedResources map[string]env.ResourceWithEnv,
	slug string,
) {
	if _, ok := mergedResources[slug]; ok {
		return
	}

	remoteResource, err := state.RemoteClient.GetResource(ctx, api.GetResourceRequest{
		Slug: slug,
	})

	if err == nil {
		mergedResources[slug] = env.ResourceWithEnv{
			Resource: remoteResource.ExportResource,
			Remote:   true,
		}
	} else {
		// If remote resource isn't found, don't error.
		logger.Debug("Error getting resource: %v", err)
	}
}

// ListRemoteResources returns any remote resources that the user can develop against. If no fallback environment is
// set, we still return a set of default remote resources for convenience.
func ListRemoteResources(ctx context.Context, state *state.State) ([]libapi.Resource, error) {
	if state.HasFallbackEnv() {
		resp, err := state.RemoteClient.ListResources(ctx, state.Env.Slug)
		if err != nil {
			return nil, err
		}
		return resp.Resources, nil
	}

	// Always add remote Demo DB for convenience, even when no fallback environment is set.
	resources := make([]libapi.Resource, 0)
	for _, slug := range defaultRemoteResourceSlugs {
		remoteResource, err := state.RemoteClient.GetResource(ctx, api.GetResourceRequest{
			Slug: slug,
		})

		if err == nil {
			resources = append(resources, remoteResource.Resource)
		} else {
			// If default remote resource isn't found, don't error.
			logger.Debug("Error getting resource: %v", err)
		}
	}

	return resources, nil
}

// Creates a map of the resource alias to resource ID
func GenerateResourceAliasToID(resourceAliases map[string]resources.Resource) map[string]string {
	resourceAliasToID := map[string]string{}
	for alias, resource := range resourceAliases {
		resourceAliasToID[alias] = resource.ID()
	}
	return resourceAliasToID
}

func SlugFromID(id string) (string, error) {
	if len(id) <= 4 {
		return "", errors.New("id must be of the form res-{slug}")
	}

	return id[4:], nil
}
