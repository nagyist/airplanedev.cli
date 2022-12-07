package flags

import (
	"context"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/flags/flagsiface"
	"github.com/airplanedev/cli/pkg/logger"
)

const (
	defaultFlagFreshness = time.Minute * 10
)

type APIClient struct {
	Client api.APIClient
}

var _ flagsiface.Flagger = &APIClient{}

func (c *APIClient) Bool(ctx context.Context, l logger.Logger, flag string, opts ...flagsiface.BoolOpts) bool {
	o := flagsiface.BoolOpts{}
	if len(opts) > 0 {
		o = opts[0]
	}

	userConf, _ := conf.ReadDefaultUserConfig()

	var flags map[string]string
	if len(userConf.Flags.Flags) > 0 && userConf.Flags.Updated.After(time.Now().Add(defaultFlagFreshness*-1)) {
		flags = userConf.Flags.Flags
	} else {
		resp, err := c.Client.ListFlags(ctx)
		if err != nil {
			l.Debug("Error listing feature flags %s", err)

			return o.Default
		}
		flags = resp.Flags

		userConf.Flags = conf.FlagsUpdate{
			Flags:   flags,
			Updated: time.Now().UTC(),
		}

		if err := conf.WriteDefaultUserConfig(userConf); err != nil {
			l.Debug("Error writing flags to user config %s", err)
		}
	}

	val, ok := flags[flag]
	if !ok {
		l.Debug("Flag %s does not exist", flag)
		return o.Default
	}
	l.Debug("Flag %s evaluated: %s", flag, val)
	return val == "true"
}
