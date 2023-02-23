package resources

import (
	"context"
	"fmt"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	libapi "github.com/airplanedev/lib/pkg/api"
	libhttp "github.com/airplanedev/lib/pkg/api/http"
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/pkg/errors"
)

const SlackID = "res00000000zteamslack"
const slackSlug = "team_slack"

// This is not guaranteed to be the slug of the demo db, but should be in all cases where demo db creation doesn't
// fail during team creation.
const DemoDBSlug = "demo_db"

// Always add remote Demo DB for convenience, even when no fallback environment is set.
var defaultRemoteResourceSlugs = []string{DemoDBSlug}
var defaultRemoteVirtualResourceSlugs = []string{slackSlug}

// GenerateAliasToResourceMap generates a mapping from alias to resource - resourceAttachments is a mapping from alias
// to slug, and slugToResource is a mapping from slug to resource, and so we just link the two.
func GenerateAliasToResourceMap(
	ctx context.Context,
	resourceAttachments map[string]string,
	slugToResource map[string]env.ResourceWithEnv,
	fallbackEnvSlug *string,
	remoteClient api.APIClient,
) (map[string]resources.Resource, error) {
	aliasToResourceMap := map[string]resources.Resource{}
	// We only need to generate entries in the map for resources that are explicitly attached to a task.
	for alias, ref := range resourceAttachments {
		var resource resources.Resource
		resourceWithEnv, ok := LookupResource(slugToResource, ref)
		if !ok {
			msg := fmt.Sprintf("cannot find resource %q. Is it defined in your dev config file", ref)
			if fallbackEnvSlug != nil {
				msg += fmt.Sprintf(" or in your %s environment", *fallbackEnvSlug)
			}
			msg += "? You can add it using the resource sidebar to the left."
			return nil, libhttp.NewErrNotFound(msg)
		}

		// This should get remote resource credentials even if no fallback env is provided in the case of default
		// resources
		if resourceWithEnv.Remote && remoteClient != nil {
			remoteResourceWithCredentials, err := remoteClient.GetResource(ctx, api.GetResourceRequest{
				ID:                   resourceWithEnv.Resource.GetID(),
				EnvSlug:              pointers.ToString(fallbackEnvSlug),
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
func MergeRemoteResources(ctx context.Context, state *state.State, envSlug *string) (map[string]env.ResourceWithEnv, error) {
	mergedResources := make(map[string]env.ResourceWithEnv)
	if state == nil {
		return mergedResources, nil
	}

	for slug, res := range state.DevConfig.Resources {
		mergedResources[slug] = res
	}

	remoteResources, err := ListRemoteResources(ctx, state, envSlug)
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
func ListRemoteResources(ctx context.Context, state *state.State, envSlug *string) ([]libapi.Resource, error) {
	if state.RemoteClient == nil {
		return nil, libhttp.NewErrBadRequest("no remote client, dev server is likely not ready yet")
	}

	if envSlug != nil {
		resp, err := state.RemoteClient.ListResources(ctx, *envSlug)
		if err != nil {
			return nil, err
		}
		return resp.Resources, nil
	}

	// Pull default remote resources. These get pulled from the default environment, not the given
	// environment.
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

// LookupResource looks up a resource by slug or name.
func LookupResource(resourcesBySlug map[string]env.ResourceWithEnv, ref string) (env.ResourceWithEnv, bool) {
	if res, ok := resourcesBySlug[ref]; ok {
		return res, true
	}

	for _, res := range resourcesBySlug {
		if res.Resource.GetName() == ref {
			return res, true
		}
	}

	return env.ResourceWithEnv{}, false
}

// GenerateResourceAliasToID creates a map of the resource alias to resource ID
func GenerateResourceAliasToID(aliasToResource map[string]resources.Resource) map[string]string {
	resourceAliasToID := map[string]string{}
	for alias, resource := range aliasToResource {
		resourceAliasToID[alias] = resource.GetID()
	}
	return resourceAliasToID
}
