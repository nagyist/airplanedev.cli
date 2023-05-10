package configs

import (
	"context"
	"io"
	"os"
	"strings"

	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/airplanedev/cli/pkg/cli/prompts"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/pkg/errors"
)

// ReadValue reads a config value from prompt if allowed, else stdin
func ReadValue(secret bool, p prompts.Prompter) (string, error) {
	if prompts.CanPrompt() {
		return ReadValueFromPrompt("Config value:", secret, p)
	}
	// Read from stdin
	logger.Log("Reading secret from stdin...")
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", errors.Wrap(err, "reading from stdin")
	}
	return strings.TrimSpace(string(data)), nil
}

// ReadValueFromPrompt prompts user for config value
func ReadValueFromPrompt(message string, secret bool, p prompts.Prompter) (string, error) {
	var value string
	promptOpts := make([]prompts.Opt, 0)
	if secret {
		promptOpts = append(promptOpts, prompts.WithSecret())
	}

	if err := p.Input(message, &value, promptOpts...); err != nil {
		return "", errors.Wrap(err, "prompting value")
	}

	return strings.TrimSpace(value), nil
}

type SetConfigRequest struct {
	NameTag NameTag
	Value   string
	Secret  bool
	EnvSlug string
}

// SetConfig writes config value to API and prints progress to user
func SetConfig(ctx context.Context, client api.APIClient, req SetConfigRequest) error {
	// Avoid printing back secrets
	var valueStr string
	if req.Secret {
		valueStr = "<secret value>"
	} else {
		valueStr = req.Value
	}
	logger.Log("  Setting %s to %s...", logger.Blue(JoinName(req.NameTag)), logger.Green(valueStr))
	apiReq := api.SetConfigRequest{
		Name:     req.NameTag.Name,
		Tag:      req.NameTag.Tag,
		Value:    req.Value,
		IsSecret: req.Secret,
		EnvSlug:  req.EnvSlug,
	}
	if err := client.SetConfig(ctx, apiReq); err != nil {
		return errors.Wrap(err, "set config")
	}
	logger.Log("  Done!")
	return nil
}
