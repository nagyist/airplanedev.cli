package build

import (
	"github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/deploy/bundlediscover"
	"github.com/airplanedev/cli/pkg/deploy/discover"
	"github.com/airplanedev/cli/pkg/utils/logger"
)

func BundleDiscoverer(client api.APIClient, l logger.Logger, envSlug string) *bundlediscover.Discoverer {
	return &bundlediscover.Discoverer{
		TaskDiscoverers: []discover.TaskDiscoverer{
			&discover.ScriptDiscoverer{
				Client:  client,
				Logger:  l,
				EnvSlug: envSlug,
			},
			&discover.DefnDiscoverer{
				Client: client,
				Logger: l,
			},
			&discover.CodeTaskDiscoverer{
				Client: client,
				Logger: l,
			},
		},
		ViewDiscoverers: []discover.ViewDiscoverer{
			&discover.ViewDefnDiscoverer{Client: client, Logger: l},
			&discover.CodeViewDiscoverer{Client: client, Logger: l},
		},
		Client:  client,
		Logger:  l,
		EnvSlug: envSlug,
	}
}
