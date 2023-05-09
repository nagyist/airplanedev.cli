package dev

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/pkg/errors"
	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
)

func configureTunnel(ctx context.Context, client api.APIClient, authInfo api.AuthInfoResponse) (token *string, userSubdomain string, ln net.Listener, err error) {
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
		if strings.Contains(err.Error(), "is already bound to another tunnel session") {
			// This is a heuristic since we don't have direct exposure to the ngrok error object (where
			// we could check the error code). Further, the error code is wrong according to their
			// documentation.
			return nil, "", nil, errors.New("Tunnel already bound, do you have another studio session running?")
		}
		return nil, "", nil, errors.Wrap(err, "failed to start tunnel")
	}

	if err := client.SetDevSecret(ctx, randString); err != nil {
		return nil, "", nil, errors.Wrap(err, "setting dev token")
	}

	return token, userSubdomain, ln, nil
}
