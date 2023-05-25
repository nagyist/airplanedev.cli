package apidev

import (
	"context"
	"net/http"
	"regexp"
	"sort"

	libhttp "github.com/airplanedev/cli/pkg/cli/apiclient/http"
	"github.com/airplanedev/cli/pkg/cli/server/state"
)

type GetEnvVarResponse struct {
	EnvVar LocalEnvVar `json:"envVar"`
}

func GetEnvVarHandler(ctx context.Context, state *state.State, r *http.Request) (GetEnvVarResponse, error) {
	name := r.URL.Query().Get("name")
	if name == "" {
		return GetEnvVarResponse{}, libhttp.NewErrBadRequest("env var name cannot be empty")
	}

	if val, ok := state.DevConfig.EnvVars[name]; ok {
		return GetEnvVarResponse{
			EnvVar: LocalEnvVar{
				Name:  name,
				Value: val,
			},
		}, nil
	}

	return GetEnvVarResponse{}, libhttp.NewErrNotFound("env var with name %q not found", name)
}

func UpsertEnvVarHandler(ctx context.Context, state *state.State, r *http.Request, req LocalEnvVar) (struct{}, error) {
	if req.Name == "" {
		return struct{}{}, libhttp.NewErrBadRequest("name cannot be empty")
	}

	// Verify that env var name is valid
	match, _ := regexp.MatchString("^[A-Za-z_]+[A-Za-z0-9_]*$", req.Name)
	if !match {
		return struct{}{}, libhttp.NewErrBadRequest("invalid env var name %q, must consist only of alphanumeric characters and underscores and may not begin with a number", req.Name)
	}

	return struct{}{}, state.DevConfig.SetEnvVar(req.Name, req.Value)
}

type DeleteEnvVarRequest struct {
	Name string `json:"name"`
}

func DeleteEnvVarHandler(ctx context.Context, state *state.State, r *http.Request, req DeleteEnvVarRequest) (struct{}, error) {
	return struct{}{}, state.DevConfig.DeleteEnvVar(req.Name)
}

type LocalEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ListEnvVarsResponse struct {
	EnvVars []LocalEnvVar `json:"envVars"`
}

func ListEnvVarsHandler(ctx context.Context, state *state.State, r *http.Request) (ListEnvVarsResponse, error) {
	envVars := make([]LocalEnvVar, 0, len(state.DevConfig.EnvVars))
	for name, val := range state.DevConfig.EnvVars {
		envVars = append(envVars, LocalEnvVar{
			Name:  name,
			Value: val,
		})
	}

	sort.Slice(envVars, func(i, j int) bool {
		return envVars[i].Name < envVars[j].Name
	})

	return ListEnvVarsResponse{
		EnvVars: envVars,
	}, nil
}
