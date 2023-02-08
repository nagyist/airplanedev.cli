package network

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyDevViewPath(t *testing.T) {
	require := require.New(t)

	port, err := VerifyDevViewPath("/dev/views/1234", nil)
	require.NoError(err)
	require.Equal(1234, port)

	token := "abc123"
	_, err = VerifyDevViewPath("/dev/views/1234/", &token)
	require.Error(err)

	port, err = VerifyDevViewPath("/dev/views/1234/abc123/", &token)
	require.NoError(err)
	require.Equal(1234, port)
}
