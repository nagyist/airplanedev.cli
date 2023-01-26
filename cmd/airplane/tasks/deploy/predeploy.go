package deploy

import (
	"context"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/configs"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	libhttp "github.com/airplanedev/lib/pkg/api/http"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/pkg/errors"
)

// ensureConfigVarsExist checks for config references in env and asks users to create any missing ones
func ensureConfigVarsExist(ctx context.Context, client api.APIClient, l logger.LoggerWithLoader, def definitions.DefinitionInterface, envSlug string) error {
	// Check if configs exist for env vars populated from configs
	env, err := def.GetEnv()
	if err != nil {
		return err
	}
	for _, v := range env {
		if v.Config != nil {
			if err := ensureConfigVarExists(ctx, client, l, ensureConfigVarExistsParams{
				ConfigName: *v.Config,
				EnvSlug:    envSlug,
			}); err != nil {
				return err
			}
		}
	}

	// Check if configs exist for config attachments
	configAttachments, err := def.GetConfigAttachments()
	if err != nil {
		return err
	}
	for _, ca := range configAttachments {
		if err != nil {
			return err
		}
		if err := ensureConfigVarExists(ctx, client, l, ensureConfigVarExistsParams{
			ConfigName: ca.NameTag,
			EnvSlug:    envSlug,
		}); err != nil {
			return err
		}
	}
	return nil
}

type ensureConfigVarExistsParams struct {
	ConfigName string
	EnvSlug    string
}

func ensureConfigVarExists(ctx context.Context, client api.APIClient, l logger.LoggerWithLoader, params ensureConfigVarExistsParams) error {
	cn, err := configs.ParseName(params.ConfigName)
	if err != nil {
		return err
	}
	_, err = client.GetConfig(ctx, api.GetConfigRequest{
		Name:    cn.Name,
		Tag:     cn.Tag,
		EnvSlug: params.EnvSlug,
	})
	var errsc libhttp.ErrStatusCode
	if errors.As(err, &errsc) && errsc.StatusCode == 404 {
		if !utils.CanPrompt() {
			return errors.Errorf("config %s does not exist", params.ConfigName)
		}
		l.Log("Your task definition references config %s, which does not exist", logger.Bold(params.ConfigName))
		wasActive := l.StopLoader()
		if wasActive {
			defer l.StartLoader()
		}
		confirmed, errc := utils.Confirm("Create it now?")
		if errc != nil {
			return errc
		}
		if !confirmed {
			return errors.Errorf("config %s does not exist", params.ConfigName)
		}
		return createConfig(ctx, client, cn, params.EnvSlug)
	}

	return err
}

func createConfig(ctx context.Context, client api.APIClient, cn configs.NameTag, envSlug string) error {
	var secret bool
	if err := survey.AskOne(
		&survey.Confirm{
			Message: "Is this config a secret?",
			Help:    "Secret config values are not shown to users",
			Default: false,
		},
		&secret,
		survey.WithStdio(os.Stdin, os.Stderr, os.Stderr),
	); err != nil {
		return errors.Wrap(err, "prompting value")
	}
	value, err := configs.ReadValueFromPrompt(fmt.Sprintf("Value for %s", configs.JoinName(cn)), secret)
	if err != nil {
		return err
	}
	return configs.SetConfig(ctx, client, configs.SetConfigRequest{
		NameTag: cn,
		Value:   value,
		Secret:  secret,
		EnvSlug: envSlug,
	})
}
