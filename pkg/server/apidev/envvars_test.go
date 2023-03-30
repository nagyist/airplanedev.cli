package apidev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	libhttp "github.com/airplanedev/cli/pkg/api/http"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/server"
	"github.com/airplanedev/cli/pkg/server/apidev"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/server/test_utils"
	"github.com/stretchr/testify/require"
)

func TestEnvVarsCRUD(t *testing.T) {
	require := require.New(t)

	dir, err := os.MkdirTemp("", "cli_test")
	require.NoError(err)
	path := filepath.Join(dir, "airplane.dev.yaml")

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			DevConfig: &conf.DevConfig{
				EnvVars: map[string]string{
					"ENV_VAR_0": "0",
					"ENV_VAR_1": "1",
				},
				Path: path,
			},
		}, server.Options{}),
	)

	// Test listing
	var listResp apidev.ListEnvVarsResponse
	body := h.GET("/dev/envVars/list").
		Expect().
		Status(http.StatusOK).Body()
	err = json.Unmarshal([]byte(body.Raw()), &listResp)
	require.NoError(err)
	expected := []apidev.LocalEnvVar{
		{
			Name:  "ENV_VAR_0",
			Value: "0",
		},
		{
			Name:  "ENV_VAR_1",
			Value: "1",
		},
	}
	require.Equal(expected, listResp.EnvVars)

	// Test getting
	var getResp apidev.GetEnvVarResponse
	body = h.GET("/dev/envVars/get").
		WithQuery("name", "ENV_VAR_0").
		Expect().
		Status(http.StatusOK).Body()
	err = json.Unmarshal([]byte(body.Raw()), &getResp)
	require.NoError(err)
	require.Equal(apidev.LocalEnvVar{
		Name:  "ENV_VAR_0",
		Value: "0",
	}, getResp.EnvVar)

	// Test update
	//nolint: staticcheck
	body = h.PUT("/dev/envVars/upsert").
		WithJSON(apidev.LocalEnvVar{Name: "ENV_VAR_0", Value: "2"}).
		Expect().
		Status(http.StatusOK).Body()

	var getResp2 apidev.GetEnvVarResponse
	body = h.GET("/dev/envVars/get").
		WithQuery("name", "ENV_VAR_0").
		Expect().
		Status(http.StatusOK).Body()
	err = json.Unmarshal([]byte(body.Raw()), &getResp2)
	require.NoError(err)
	require.Equal(apidev.LocalEnvVar{
		Name:  "ENV_VAR_0",
		Value: "2",
	}, getResp2.EnvVar)

	// Test deleting
	//nolint: staticcheck
	body = h.DELETE("/dev/envVars/delete").
		WithJSON(apidev.DeleteEnvVarRequest{Name: "ENV_VAR_0"}).
		Expect().
		Status(http.StatusOK).Body()

	var listResp2 apidev.ListEnvVarsResponse
	body = h.GET("/dev/envVars/list").
		Expect().
		Status(http.StatusOK).Body()
	err = json.Unmarshal([]byte(body.Raw()), &listResp2)
	require.NoError(err)
	require.Equal([]apidev.LocalEnvVar{
		{
			Name:  "ENV_VAR_1",
			Value: "1",
		},
	}, listResp2.EnvVars)
}

func TestEnvVarsInvalidName(t *testing.T) {
	require := require.New(t)

	dir, err := os.MkdirTemp("", "cli_test")
	require.NoError(err)
	path := filepath.Join(dir, "airplane.dev.yaml")

	h := test_utils.GetHttpExpect(
		context.Background(),
		t,
		server.NewRouter(&state.State{
			DevConfig: &conf.DevConfig{
				EnvVars: map[string]string{},
				Path:    path,
			},
		}, server.Options{}),
	)

	// Test bad update
	//nolint: staticcheck
	body := h.PUT("/dev/envVars/upsert").
		WithJSON(apidev.LocalEnvVar{Name: "ENV_VAR=asdf", Value: "0"}).
		Expect().
		Status(http.StatusBadRequest).Body()

	var errResp libhttp.ErrorResponse
	err = json.Unmarshal([]byte(body.Raw()), &errResp)
	require.NoError(err)
	require.Contains(errResp.Error, "must consist only of alphanumeric characters and underscores")
}
