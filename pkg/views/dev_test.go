package views

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAdditionalEnvs(t *testing.T) {
	testCases := []struct {
		desc    string
		envSlug string
		host    string
		apiKey  string
		token   string
		osEnvs  map[string]string
		envs    []string
	}{
		{
			desc:    "all vars passed in",
			envSlug: "env",
			host:    "host",
			apiKey:  "apiKey",
			token:   "token",
			envs: []string{
				"AIRPLANE_API_HOST=https://host",
				"AIRPLANE_ENV_SLUG=env",
				"AIRPLANE_TOKEN=token",
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
			assert := assert.New(t)

			for k, v := range tC.osEnvs {
				os.Setenv(k, v)
			}
			e := getAdditionalEnvs(tC.host, tC.apiKey, tC.token, tC.envSlug)
			assert.Equal(tC.envs, e)
		})
	}
}
