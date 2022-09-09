package node

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeTSConfigsRecursively(t *testing.T) {
	dest := map[string]interface{}{}
	src := map[string]interface{}{
		"foo": "hello",
		"bar": map[string]interface{}{
			"baz": []string{"hello", "there"},
		},
	}

	mergeTSConfigsRecursively(dest, src)
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

	mergeTSConfigsRecursively(dest, src)
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
