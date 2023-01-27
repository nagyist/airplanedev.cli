package apidev

import (
	"context"
	"net/http"
	"regexp"
	"sort"

	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/pkg/errors"
)

type GetEnvVarResponse struct {
	EnvVar LocalEnvVar `json:"envVar"`
}

func GetEnvVarHandler(ctx context.Context, state *state.State, r *http.Request) (GetEnvVarResponse, error) {
	name := r.URL.Query().Get("name")
	if name == "" {
		return GetEnvVarResponse{}, errors.New("name cannot be empty")
	}

	if val, ok := state.DevConfig.EnvVars[name]; ok {
		return GetEnvVarResponse{
			EnvVar: LocalEnvVar{
				Name:  name,
				Value: val,
			},
		}, nil
	}

	return GetEnvVarResponse{}, errors.Errorf("env var with name %s not found", name)
}

func UpsertEnvVarHandler(ctx context.Context, state *state.State, r *http.Request, req LocalEnvVar) (struct{}, error) {
	if req.Name == "" {
		return struct{}{}, errors.New("name cannot be empty")
	}

	// Verify that env var name is valid
	match, _ := regexp.MatchString("^[A-Za-z_]+[A-Za-z0-9_]*$", req.Name)
	if !match {
		return struct{}{}, errors.Errorf("invalid env var name %s, must consist only of alphanumeric characters and underscores", req.Name)
	}

	if err := state.DevConfig.SetEnvVar(req.Name, req.Value); err != nil {
		return struct{}{}, errors.Wrap(err, "setting env var")
	}

	return struct{}{}, nil
}

type DeleteEnvVarRequest struct {
	Name string `json:"name"`
}

func DeleteEnvVarHandler(ctx context.Context, state *state.State, r *http.Request, req DeleteEnvVarRequest) (struct{}, error) {
	if err := state.DevConfig.DeleteEnvVar(req.Name); err != nil {
		return struct{}{}, errors.Wrap(err, "deleting env var")
	}

	return struct{}{}, nil
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
