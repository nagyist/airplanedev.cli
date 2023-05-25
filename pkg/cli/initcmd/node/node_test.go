package node

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/airplanedev/cli/pkg/cli/prompts"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/stretchr/testify/require"
)

func TestMergeMapsRecursively(t *testing.T) {
	dest := map[string]interface{}{}
	src := map[string]interface{}{
		"foo": "hello",
		"bar": map[string]interface{}{
			"baz": []string{"hello", "there"},
		},
	}

	mergeMapsRecursively(dest, src)
	require.True(t, reflect.DeepEqual(dest, src))

	dest = map[string]interface{}{
		"overridden":     []string{"override", "me"},
		"not_overridden": []string{"do", "not", "override", "me"},
		"overridden_sub": map[string]interface{}{
			"overridden_sub_overridden":     "ok",
			"overridden_sub_not_overridden": "ok",
		},
	}
	src = map[string]interface{}{
		"overridden": "ok",
		"overridden_sub": map[string]interface{}{
			"overridden_sub_overridden":       "not ok",
			"overridden_sub_additional_field": "not ok",
		},
		"additional_field": "ok",
	}

	mergeMapsRecursively(dest, src)
	result := map[string]interface{}{
		"overridden":     "ok",
		"not_overridden": []string{"do", "not", "override", "me"},
		"overridden_sub": map[string]interface{}{
			"overridden_sub_overridden":       "not ok",
			"overridden_sub_not_overridden":   "ok",
			"overridden_sub_additional_field": "not ok",
		},
		"additional_field": "ok",
	}

	require.True(t, reflect.DeepEqual(dest, result))
}

func TestParseTSConfig(t *testing.T) {
	tsConfig, err := parseTSConfig([]byte(`
		{
			"compilerOptions": {
				"experimentalDecorators": true,
				"skipLibCheck": true, // This is a trailing comma
			},
			"compileOnSave": true,
			"files": ["node_modules/jest-expect-message/types/index.d.ts"],
		}`,
	))
	require.NoError(t, err)
	require.Equal(
		t,
		map[string]interface{}{
			"compilerOptions": map[string]interface{}{
				"experimentalDecorators": true,
				"skipLibCheck":           true,
			},
			"compileOnSave": true,
			"files": []interface{}{
				"node_modules/jest-expect-message/types/index.d.ts",
			},
		},
		tsConfig,
	)

	_, err = parseTSConfig([]byte(`bad config`))
	require.Error(t, err)
}

func TestMergeTSConfig(t *testing.T) {
	require := require.New(t)
	tempDir := t.TempDir()

	cwd, err := os.Getwd()
	require.NoError(err)
	defer func() {
		// Change back to the original directory when the current test case is done.
		err = os.Chdir(cwd)
		require.NoError(err)
	}()

	err = os.Chdir(tempDir)
	require.NoError(err)

	err = os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(`{
  "compilerOptions": {
    "lib": ["ESNext"],
	"target": "ESNext"
  },
}`,
	), 0644)

	requiredTSConfig := []byte(`{
  "compilerOptions": {
    "lib": ["DOM", "DOM.Iterable"],
	"jsx": "react-jsx"
  },
}`)

	// Trigger two merges. The second one should be a no-op.
	for i := 0; i < 2; i++ {
		created, err := mergeTSConfig(
			tempDir,
			nil,
			requiredTSConfig,
			prompts.NewMock(true),
			&logger.MockLogger{},
			false,
		)
		require.NoError(err)
		require.False(created)

		newTsConfigContents, err := os.ReadFile(filepath.Join(tempDir, "tsconfig.json"))
		require.NoError(err)
		require.Equal(
			`{
  "compilerOptions": {
    "jsx": "react-jsx",
    "lib": [
      "ESNext",
      "DOM",
      "DOM.Iterable"
    ],
    "target": "ESNext"
  }
}`, string(newTsConfigContents))
	}
}
