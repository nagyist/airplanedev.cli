package dev

import (
	"context"

	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
)

func configureSandbox(ctx context.Context, client api.APIClient, namespace string, key string) (*string, error) {
	// Get a sandbox token, creating a sandbox if necessary. The sandbox should have already been created at this point,
	// so internally this should just short-circuit and return a token.
	res, err := client.CreateSandbox(ctx, api.CreateSandboxRequest{
		Namespace: &namespace,
		Key:       &key,
	})
	if err != nil {
		return nil, err
	}

	return &res.Token, nil
}