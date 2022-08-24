package dev

import (
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestMarshalRequest(t *testing.T) {
	paramValues := api.Values{"hello": 123}

	request, err := marshalBuiltinRequest("airplane:rest_request", paramValues)
	require.NoError(t, err)
	require.Equal(t, request, `{"namespace":"rest","name":"request","request":{"hello":123}}`)
}
