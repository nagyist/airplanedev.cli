package dev

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
)

type AirplaneTokenClaims struct {
	RunID string
}

// generateLocalDevAirplaneToken creates a JWT token that can be supplied to local
// runs as AIRPLANE_TOKEN. The token is not intended for secure usage.
func GenerateInsecureAirplaneToken(claims AirplaneTokenClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"runID": claims.RunID,
	})
	s, err := token.SignedString([]byte("airplane"))
	return s, errors.Wrap(err, "generating local dev token")
}

// ParseInsecureAirplaneToken extracts claims from an airplane runtime token. This
// should only be used for local development where we do not need to validate the
// integrity of this token.
func ParseInsecureAirplaneToken(token string) (AirplaneTokenClaims, error) {
	p := jwt.Parser{}
	t, _, err := p.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return AirplaneTokenClaims{}, errors.Wrap(err, "parsing airplane token")
	}
	claims, _ := t.Claims.(jwt.MapClaims)
	runID, _ := claims["runID"].(string)
	return AirplaneTokenClaims{
		RunID: runID,
	}, nil
}
