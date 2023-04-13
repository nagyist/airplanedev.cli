package views

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/airplanedev/cli/pkg/build/node"
	"github.com/stretchr/testify/require"
)

func TestGetAdditionalEnvs(t *testing.T) {
	testCases := []struct {
		desc        string
		envSlug     string
		host        string
		apiKey      string
		token       string
		tunnelToken string
		osEnvs      map[string]string
		envs        []string
	}{
		{
			desc:        "all vars passed in",
			envSlug:     "env",
			host:        "host",
			apiKey:      "apiKey",
			token:       "token",
			tunnelToken: "tunnel_token",
			envs: []string{
				"AIRPLANE_API_HOST=https://host",
				"AIRPLANE_ENV_SLUG=env",
				"AIRPLANE_TOKEN=token",
				"AIRPLANE_TUNNEL_TOKEN=tunnel_token",
			},
		},
		{
			desc:    "api key",
			envSlug: "env",
			host:    "host",
			apiKey:  "apiKey",
			envs: []string{
				"AIRPLANE_API_HOST=https://host",
				"AIRPLANE_ENV_SLUG=env",
				"AIRPLANE_API_KEY=apiKey",
			},
		},
		{
			desc: "host already has https",
			host: "https://host",
			envs: []string{
				"AIRPLANE_API_HOST=https://host",
			},
		},
		{
			desc:    "env vars already exist",
			envSlug: "env",
			host:    "host",
			apiKey:  "apiKey",
			token:   "token",
			osEnvs: map[string]string{
				"AIRPLANE_ENV_SLUG": "env2",
				"AIRPLANE_TOKEN":    "token2",
				"AIRPLANE_API_HOST": "host2",
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			r := require.New(t)

			for k, v := range tC.osEnvs {
				os.Setenv(k, v)
			}
			var tunnelTokenPtr *string
			if tC.tunnelToken != "" {
				tunnelTokenPtr = &tC.tunnelToken
			}

			e := getAdditionalEnvs(tC.host, tC.apiKey, tC.token, tC.envSlug, tunnelTokenPtr)
			r.Equal(tC.envs, e)
		})
	}
}

func TestAddDepsToPackageJSON(t *testing.T) {
	r := require.New(t)
	var buildToolsPackageJSON node.PackageJSON
	err := json.Unmarshal([]byte(node.BuildToolsPackageJSON), &buildToolsPackageJSON)
	r.NoError(err)

	testCases := []struct {
		desc                   string
		existingPackageJSON    node.PackageJSON
		expectedDevPackageJSON node.PackageJSON
	}{
		{
			desc:                "empty",
			existingPackageJSON: node.PackageJSON{},
			expectedDevPackageJSON: node.PackageJSON{
				Dependencies: map[string]string{
					"react":           buildToolsPackageJSON.Dependencies["react"],
					"react-dom":       buildToolsPackageJSON.Dependencies["react-dom"],
					"@airplane/views": buildToolsPackageJSON.Dependencies["@airplane/views"],
					"object-hash":     buildToolsPackageJSON.Dependencies["object-hash"],
				},
				DevDependencies: map[string]string{
					"@vitejs/plugin-react": buildToolsPackageJSON.Dependencies["@vitejs/plugin-react"],
					"vite":                 buildToolsPackageJSON.Dependencies["vite"],
					"vite-plugin-replace":  buildToolsPackageJSON.Dependencies["vite-plugin-replace"],
				},
			},
		},
		{
			desc: "existing dep don't override",
			existingPackageJSON: node.PackageJSON{
				Dependencies: map[string]string{
					"react": "1.0.0",
				},
			},
			expectedDevPackageJSON: node.PackageJSON{
				Dependencies: map[string]string{
					"react-dom":       buildToolsPackageJSON.Dependencies["react-dom"],
					"@airplane/views": buildToolsPackageJSON.Dependencies["@airplane/views"],
					"object-hash":     buildToolsPackageJSON.Dependencies["object-hash"],
				},
				DevDependencies: map[string]string{
					"@vitejs/plugin-react": buildToolsPackageJSON.Dependencies["@vitejs/plugin-react"],
					"vite":                 buildToolsPackageJSON.Dependencies["vite"],
					"vite-plugin-replace":  buildToolsPackageJSON.Dependencies["vite-plugin-replace"],
				},
			},
		},
		{
			desc: "existing dev dep always override",
			existingPackageJSON: node.PackageJSON{
				DevDependencies: map[string]string{
					"vite": "1.0.0",
				},
			},
			expectedDevPackageJSON: node.PackageJSON{
				Dependencies: map[string]string{
					"react":           buildToolsPackageJSON.Dependencies["react"],
					"react-dom":       buildToolsPackageJSON.Dependencies["react-dom"],
					"@airplane/views": buildToolsPackageJSON.Dependencies["@airplane/views"],
					"object-hash":     buildToolsPackageJSON.Dependencies["object-hash"],
				},
				DevDependencies: map[string]string{
					"@vitejs/plugin-react": buildToolsPackageJSON.Dependencies["@vitejs/plugin-react"],
					"vite":                 buildToolsPackageJSON.Dependencies["vite"],
					"vite-plugin-replace":  buildToolsPackageJSON.Dependencies["vite-plugin-replace"],
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			subR := require.New(t)

			devPackageJSON := node.PackageJSON{
				Dependencies:    map[string]string{},
				DevDependencies: map[string]string{},
			}
			err := addDevDepsToPackageJSON(tC.existingPackageJSON, devPackageJSON)
			subR.NoError(err)
			subR.Equal(tC.expectedDevPackageJSON, devPackageJSON)
		})
	}
}
