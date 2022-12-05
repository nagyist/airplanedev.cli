package configs

import (
	"context"
	"strings"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
)

var ErrInvalidConfigName = errors.New("invalid config name")

type NameTag struct {
	Name string
	Tag  string
}

func ParseName(nameTag string) (NameTag, error) {
	var res NameTag
	parts := strings.Split(nameTag, ":")
	if len(parts) > 2 {
		return res, ErrInvalidConfigName
	}
	res.Name = parts[0]
	if len(parts) >= 2 {
		res.Tag = parts[1]
	}
	return res, nil
}

func JoinName(nameTag NameTag) string {
	var tagStr string
	if nameTag.Tag != "" {
		tagStr = ":" + nameTag.Tag
	}
	return nameTag.Name + tagStr
}

// MergeRemoteConfigs merges the configs defined in the dev config file with remote configs from the fallback env passed
// to the local dev server upon startup.
func MergeRemoteConfigs(ctx context.Context, state *state.State) (map[string]env.ConfigWithEnv, error) {
	mergedConfigs := make(map[string]env.ConfigWithEnv)
	if state == nil {
		return mergedConfigs, nil
	}

	if state.UseFallbackEnv {
		maps.Copy(mergedConfigs, state.DevConfig.ConfigVars)

		configs, err := ListRemoteConfigs(ctx, state)
		if err != nil {
			return nil, err
		}

		for _, cfg := range configs {
			if _, ok := mergedConfigs[cfg.Name]; !ok {
				mergedConfigs[cfg.Name] = env.ConfigWithEnv{
					Config: cfg,
					Remote: true,
					Env:    state.RemoteEnv,
				}
			}
		}
		return mergedConfigs, nil
	} else {
		return state.DevConfig.ConfigVars, nil
	}
}

func ListRemoteConfigs(ctx context.Context, state *state.State) ([]api.Config, error) {
	resp, err := state.RemoteClient.ListConfigs(ctx, api.ListConfigsRequest{
		EnvSlug: state.RemoteEnv.Slug,
	})
	if err != nil {
		return nil, errors.Wrap(err, "listing remote configs")
	}

	return resp.Configs, nil
}
