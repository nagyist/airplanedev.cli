package analytics

import (
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/golang-jwt/jwt/v5"
)

type AnalyticsToken struct {
	UserID string
	TeamID string
}

// ParseTokenForAnalytics parses UNVERIFIED JWT information - this information can be spoofed.
// Should only be used for analytics, nothing sensitive.
func ParseTokenForAnalytics(token string) AnalyticsToken {
	var res AnalyticsToken
	if token == "" {
		return res
	}
	t, _, err := new(jwt.Parser).ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		logger.Debug("error parsing token: %v", err)
		return res
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return res
	}
	res.UserID = claims["userID"].(string)
	res.TeamID = claims["teamID"].(string)
	return res
}
