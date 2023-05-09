package python

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPackagesToAdd(t *testing.T) {
	testCases := []struct {
		desc                 string
		requirements         string
		dependencies         []PythonDependency
		expectedDependencies []PythonDependency
	}{
		{
			desc:                 "empty requirements.txt",
			dependencies:         []PythonDependency{{Name: "airplanesdk", Version: ">3.0.0"}},
			expectedDependencies: []PythonDependency{{Name: "airplanesdk", Version: ">3.0.0"}},
		},
		{
			desc:         "requirements.txt already has airplanesdk",
			requirements: "airplanesdk==3.0.0",
			dependencies: []PythonDependency{{Name: "airplanesdk", Version: ">3.0.0"}},
		},
		{
			desc:                 "requirements.txt has other deps",
			requirements:         "someotherpackage==3.0.0",
			dependencies:         []PythonDependency{{Name: "airplanesdk", Version: ">3.0.0"}},
			expectedDependencies: []PythonDependency{{Name: "airplanesdk", Version: ">3.0.0"}},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)
			r := strings.NewReader(tC.requirements)

			d, err := getPackagesToAdd(r, tC.dependencies)
			require.NoError(err)
			assert.ElementsMatch(tC.expectedDependencies, d)
		})
	}
}
