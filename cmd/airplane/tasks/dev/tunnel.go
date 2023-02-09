package dev

import (
	"context"
	"fmt"
	"net"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/pkg/errors"
	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
)

func configureTunnel(ctx context.Context, client *api.Client, authInfo api.AuthInfoResponse) (token *string, userSubdomain string, ln net.Listener, err error) {
	// Obtain user-specific ngrok auth token.
	tokenResp, err := client.GetTunnelToken(ctx)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, "unable to acquire tunnel token")
	}

	randString := utils.RandomString(20, utils.CharsetAlphaNumeric)
	token = &randString
	userSubdomain = fmt.Sprintf("%s.t.airplane.sh", authInfo.User.ID)
	if ln, err = ngrok.Listen(ctx,
		config.HTTPEndpoint(
			config.WithDomain(userSubdomain),
		),
		ngrok.WithAuthtoken(tokenResp.Token),
		ngrok.WithRegion("us"),
	); err != nil {
		return nil, "", nil, errors.Wrap(err, "failed to start tunnel")
	}

	if err := client.SetDevSecret(ctx, randString); err != nil {
		return nil, "", nil, errors.Wrap(err, "setting dev token")
	}

	return token, userSubdomain, ln, nil
}
