package build

import (
	"testing"

	"github.com/airplanedev/lib/pkg/build/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalPackages(t *testing.T) {
	testCases := []struct {
		desc             string
		packageJSON      string
		externalPackages []string
		usesWorkspaces   bool
	}{
		{
			desc:           "no packages",
			packageJSON:    fixtures.Path(t, "node_externals/empty/package.json"),
			usesWorkspaces: false,
		},
		{
			desc:             "marks external dependencies, dev dependencies and optional dependencies",
			packageJSON:      fixtures.Path(t, "node_externals/dependencies/package.json"),
			externalPackages: []string{"react", "@types/react", "react-table"},
			usesWorkspaces:   false,
		},
		{
			desc:             "does not mark esm modules as external",
			packageJSON:      fixtures.Path(t, "node_externals/esm/package.json"),
			externalPackages: []string{"react"},
			usesWorkspaces:   false,
		},
		{
			desc:             "marks external all packages in yarn workspace",
			packageJSON:      fixtures.Path(t, "node_externals/yarnworkspace/package.json"),
			externalPackages: []string{"react", "@types/react", "react-table"},
			usesWorkspaces:   true,
		},
		{
			desc:             "marks external all packages in yarn workspace with a package.json not included in the workspace",
			packageJSON:      fixtures.Path(t, "node_externals/yarnworkspacewithpackagenotinworkspace/package.json"),
			externalPackages: []string{"react", "@types/react", "react-table"},
			usesWorkspaces:   true,
		},
		{
			desc:             "does not mark local yarn workspace import as external",
			packageJSON:      fixtures.Path(t, "node_externals/yarnworkspace_importlocal/package.json"),
			externalPackages: []string{"react", "@types/react", "react-table"},
			usesWorkspaces:   true,
		},
		{
			desc:             "marks local yarn workspace import as external if mismatched version",
			packageJSON:      fixtures.Path(t, "node_externals/yarnworkspace_importlocalmismatched/package.json"),
			externalPackages: []string{"react", "@types/react", "react-table", "lib"},
			usesWorkspaces:   true,
		},
		{
			desc:             "marks external all packages in yarn workspace with yarn 2",
			packageJSON:      fixtures.Path(t, "node_externals/yarn2workspace_importlocal/package.json"),
			externalPackages: []string{"react", "@types/react", "react-table"},
			usesWorkspaces:   true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			packageJSONs, usesWorkspaces, err := GetPackageJSONs(tC.packageJSON)
			require.NoError(err)
			assert.Equal(tC.usesWorkspaces, usesWorkspaces)

			externalPackages, err := ExternalPackages(packageJSONs, usesWorkspaces)
			require.NoError(err)

			assert.ElementsMatch(tC.externalPackages, externalPackages)
		})
	}
}

func TestHasInstallHooks(t *testing.T) {
	result, err := hasInstallHooks(
		fixtures.Path(t, "node_externals/postinstall/package.json"),
	)
	require.NoError(t, err)
	require.True(t, result)

	result, err = hasInstallHooks(
		fixtures.Path(t, "node_externals/preinstall/package.json"),
	)
	require.NoError(t, err)
	require.True(t, result)

	result, err = hasInstallHooks(
		fixtures.Path(t, "node_externals/esm/package.json"),
	)
	require.NoError(t, err)
	require.False(t, result)

	result, err = hasInstallHooks(
		fixtures.Path(t, "node_externals/non_existent_path/package.json"),
	)
	require.NoError(t, err)
	require.False(t, result)
}

func TestGetPackageCopyCmds(t *testing.T) {
	result, err := GetPackageCopyCmds(
		"/home/base",
		[]string{
			"/home/base/test1/package.json",
			"/home/base/package.json",
			"/home/base/test1/package-lock.json",
			"/home/base/_test2/test3/package.json",
		},
	)
	require.NoError(t, err)

	require.Equal(
		t,
		[]string{
			"COPY package*.json yarn.* /airplane/",
			"COPY test1/package*.json test1/yarn.* /airplane/test1/",
			"COPY _test2/test3/package*.json _test2/test3/yarn.* /airplane/_test2/test3/",
		},
		result,
	)
}

func TestGetYarnLockPackageVersion(t *testing.T) {
	version, err := getYarnLockPackageVersion(
		fixtures.Path(t, "node_externals/yarnworkspace"),
		"csstype",
	)
	require.NoError(t, err)
	require.Equal(t, "3.1.0", version)

	version, err = getYarnLockPackageVersion(
		fixtures.Path(t, "node_externals/yarn_nopkgversion"),
		"tslib",
	)
	require.NoError(t, err)
	require.Equal(t, "2.4.1", version)

	_, err = getYarnLockPackageVersion(
		fixtures.Path(t, "node_externals/yarnworkspace"),
		"non-existent-package",
	)
	require.Error(t, err)

	_, err = getYarnLockPackageVersion(
		fixtures.Path(t, "node_externals/non-existent-path"),
		"csstype",
	)
	require.Error(t, err)
}

func TestGetNPMLockPackageVersion(t *testing.T) {
	version, err := getNPMLockPackageVersion(
		fixtures.Path(t, "node_externals/packagelock"),
		"react",
	)
	require.NoError(t, err)
	require.Equal(t, "18.2.0", version)

	_, err = getNPMLockPackageVersion(
		fixtures.Path(t, "node_externals/packagelock"),
		"non-existent-package",
	)
	require.Error(t, err)

	_, err = getNPMLockPackageVersion(
		fixtures.Path(t, "node_externals/non-existent-path"),
		"react",
	)
	require.Error(t, err)
}

func TestGetLockPackageVersion(t *testing.T) {
	version := getLockPackageVersion(
		fixtures.Path(t, "node_externals/yarnworkspace"),
		"csstype",
		"fallback",
	)
	require.Equal(t, "3.1.0", version)

	version = getLockPackageVersion(
		fixtures.Path(t, "node_externals/packagelock"),
		"react",
		"fallback",
	)
	require.Equal(t, "18.2.0", version)

	version = getLockPackageVersion(
		fixtures.Path(t, "node_externals/non-existent-path"),
		"react",
		"fallback",
	)
	require.Equal(t, "fallback", version)
}
