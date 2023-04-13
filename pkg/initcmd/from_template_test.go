package initcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindTemplate(t *testing.T) {
	testCases := []struct {
		desc             string
		templates        []Template
		gitPath          string
		expectedTemplate Template
	}{
		{
			desc:    "from full URL",
			gitPath: "https://github.com/airplanedev/templates/myTemplate",
			templates: []Template{
				{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
			},
			expectedTemplate: Template{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
		},
		{
			desc:    "from short URL",
			gitPath: "github.com/airplanedev/templates/myTemplate",
			templates: []Template{
				{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
			},
			expectedTemplate: Template{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
		},
		{
			desc:    "from template name",
			gitPath: "myTemplate",
			templates: []Template{
				{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
			},
			expectedTemplate: Template{GitHubPath: "github.com/airplanedev/templates/myTemplate"},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			temp, err := FindTemplate(tC.templates, tC.gitPath)
			require.NoError(t, err)
			require.Equal(t, tC.expectedTemplate, temp)
		})
	}
}
