package dev

import (
	"context"

	libapi "github.com/airplanedev/cli/pkg/api"
	api "github.com/airplanedev/cli/pkg/api/cliapi"
	libhttp "github.com/airplanedev/cli/pkg/api/http"
	devenv "github.com/airplanedev/cli/pkg/dev/env"
	"github.com/airplanedev/cli/pkg/resources"
	"github.com/pkg/errors"
)

const (
	StrictModeOn  = true
	StrictModeOff = false
)

func baseEvaluateTemplateRequest(cfg LocalRunConfig, configVars map[string]string) libapi.EvaluateTemplateRequest {
	res := libapi.EvaluateTemplateRequest{
		RunID:       cfg.ID,
		Env:         devenv.NewLocalEnv(),
		Configs:     configVars,
		ParamValues: cfg.ParamValues,
		TaskID:      cfg.ID,
		TaskSlug:    cfg.Slug,
	}
	if cfg.ParentRunID != nil {
		res.ParentRunID = *cfg.ParentRunID
	}
	return res
}

func interpolate(ctx context.Context, remoteClient api.APIClient, baseRequest libapi.EvaluateTemplateRequest, useStrictMode bool, value any) (any, error) {
	resp, err := remoteClient.EvaluateTemplate(ctx, libapi.EvaluateTemplateRequest{
		Value:             value,
		RunID:             baseRequest.RunID,
		Env:               baseRequest.Env,
		Resources:         baseRequest.Resources,
		Configs:           baseRequest.Configs,
		ParamValues:       baseRequest.ParamValues,
		DisableStrictMode: !useStrictMode,
	})
	if err != nil {
		var errsc libhttp.ErrStatusCode
		if errors.As(err, &errsc) {
			return nil, errors.New(errsc.Msg)
		}
		return nil, err
	}

	return resp.Value, nil
}

func getResourceKind(rawRes map[string]resources.Resource, slug string) (resources.ResourceKind, error) {
	ogRes, ok := rawRes[slug]
	if !ok {
		return "", errors.New("resource not found")
	}
	return ogRes.GetKind(), nil

}

func interpolateResource(ctx context.Context, remoteClient api.APIClient, baseRequest libapi.EvaluateTemplateRequest, rawRes map[string]resources.Resource) (map[string]resources.Resource, error) {
	interpolatedRes, err := interpolate(ctx, remoteClient, baseRequest, StrictModeOff, rawRes)
	if err != nil {
		return nil, err
	}
	interpolatedAliasMap, ok := interpolatedRes.(map[string]interface{})
	if !ok {
		return nil, err
	}
	updatedRes := map[string]resources.Resource{}
	for slug, r := range interpolatedAliasMap {
		mapRes, ok := r.(map[string]interface{})
		if !ok {
			return nil, errors.New("expected resource to be a map")
		}
		resourceKind, err := getResourceKind(rawRes, slug)
		if err != nil {
			return nil, err
		}
		res, err := resources.GetResource(resourceKind, mapRes)
		if err != nil {
			return nil, err
		}
		if err := res.Calculate(); err != nil {
			return nil, errors.Wrap(err, "calculating resource")
		}
		updatedRes[slug] = res
	}
	return updatedRes, nil
}
