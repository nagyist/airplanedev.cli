package builtins

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshalRequest(t *testing.T) {
	paramValues := map[string]interface{}{"hello": 123}

	request, err := MarshalRequest("airplane:rest_request", paramValues)
	require.NoError(t, err)
	require.Equal(t, request, `{"namespace":"rest","name":"request","request":{"hello":123}}`)
}
