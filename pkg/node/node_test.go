package node

import (
	"reflect"
	"testing"

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
