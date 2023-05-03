package cli

import (
	api "github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/devconf"
	"github.com/airplanedev/cli/pkg/flags/flagsiface"
	"github.com/airplanedev/cli/pkg/prompts"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/golang-jwt/jwt/v5"
)

// Config represents command configuration.
//
// The config is passed down to all commands from
// the root command.
type Config struct {
	// Client represents the API client to use.
	//
	// It is initialized in the root command and passed
	// down to all commands.
	Client api.APIClient

	// Flagger supports querying feature flags.
	Flagger flagsiface.Flagger

	// DebugMode indicates if the CLI should produce additional
	// debug output to guide end-users through issues.
	DebugMode bool

	// WithTelemetry indicates if the CLI should send usage analytics and errors, even if it's been
	// previously disabled.
	WithTelemetry bool

	// Version indicates if the CLI version should be printed.
	Version bool

	// Dev indicates that we are in dev mode.
	Dev bool

	// The API host to use.
	Host string

	// Prompter represents the prompter to use to get user input.
	Prompter prompts.Prompter
}

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

// Must should be used for Cobra initialize commands that can return an error
// to enforce that they do not produce errors.
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// DevCLI stores information for subcommands under airplane dev.
type DevCLI struct {
	*Config
	DevConfig *devconf.DevConfig
	Filepath  string
}
