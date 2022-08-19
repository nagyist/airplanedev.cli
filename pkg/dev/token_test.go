package dev

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToken(t *testing.T) {
	require := require.New(t)

	claims := AirplaneTokenClaims{
		RunID: "run123",
	}
	token, err := GenerateInsecureAirplaneToken(claims)
	require.NoError(err)
	actualClaims, err := ParseInsecureAirplaneToken(token)
	require.NoError(err)

	require.Equal(claims, actualClaims)
}
