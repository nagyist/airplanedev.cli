package login

import (
	"context"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/token"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	root  *cli.Config
	token string
}

// New returns a new login command.
func New(c *cli.Config) *cobra.Command {
	var cfg = config{
		root: c,
	}

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Airplane",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Root().Context(), cfg)
		},
	}

	cmd.Flags().StringVar(&cfg.token, "token", "", "pass the cli token directly instead of via prompt")
	if err := cmd.Flags().MarkHidden("token"); err != nil {
		logger.Debug("error: %s", err)
	}

	return cmd
}

// Run runs the login command.
func run(ctx context.Context, cfg config) error {
	if err := login(ctx, cfg); err != nil {
		return err
	}

	logger.Log("You're all set!\n\nTo see what tasks you can run, try:\n    airplane tasks list")
	analytics.Track(cfg.root, "User Logged In", nil)
	return nil
}

var (
	ErrLoggedOut = errors.New("you are not logged in. To login, run:\n    airplane login")
)

// validateToken returns a boolean indicating whether or not the current
// client token is valid.
func validateToken(ctx context.Context, c *cli.Config) (bool, error) {
	if c.Client.Token == "" {
		return false, nil
	}

	_, err := c.Client.AuthInfo(ctx)
	if e, ok := err.(api.Error); ok && e.Code == 401 {
		logger.Debug("Found an expired token. Re-authenticating.")
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func validateAPIKey(ctx context.Context) bool {
	return conf.GetAPIKey() != "" && conf.GetTeamID() != ""
}

func EnsureLoggedIn(ctx context.Context, c *cli.Config) error {
	if ok, err := validateToken(ctx, c); err != nil {
		return err
	} else if ok {
		return nil
	}

	if ok := validateAPIKey(ctx); ok {
		return nil
	}

	if !utils.CanPrompt() {
		return ErrLoggedOut
	}

	if ok, err := utils.Confirm("You are not logged in. Do you want to login now?"); err != nil {
		return err
	} else if !ok {
		return ErrLoggedOut
	}

	logger.Log("\n  Logging in...\n")

	if err := login(ctx, config{
		root: c,
	}); err != nil {
		return err
	}

	return nil
}

func login(ctx context.Context, cfg config) error {
	writeToken := func(token string) error {
		cfg.root.Client.Token = token
		userConf, err := conf.ReadDefaultUserConfig()
		if err != nil && !errors.Is(err, conf.ErrMissing) {
			return err
		}
		if userConf.Tokens == nil {
			userConf.Tokens = map[string]string{}
		}
		userConf.Tokens[cfg.root.Client.Host] = token
		if err := conf.WriteDefaultUserConfig(userConf); err != nil {
			return err
		}
		return nil
	}

	if cfg.token != "" {
		// If token was provided on cmdline, exit early.
		return writeToken(cfg.token)
	}

	logger.Log("Enter your %s from %s\nor hit ENTER to log in with your browser.", logger.Bold("token"), logger.Blue(cfg.root.Client.TokenURL()))
	var tkn string
	if err := survey.AskOne(
		&survey.Password{
			Message: "Token:",
		},
		&tkn,
		survey.WithStdio(os.Stdin, os.Stderr, os.Stderr),
	); err != nil {
		return errors.Wrap(err, "prompting for token")
	}

	// If the user entered a token, finish logging in.
	if tkn != "" {
		return writeToken(tkn)
	}

	// Otherwise, open the login URL and use an HTTP server to listen for the response.
	srv, err := token.NewServer(ctx, cfg.root.Client.LoginSuccessURL())
	if err != nil {
		return err
	}
	defer srv.Close()

	url := cfg.root.Client.LoginURL(srv.URL())
	if ok := utils.Open(url); !ok {
		logger.Log("Visit %s to finish logging in", logger.Blue("%s", url))
	} else {
		logger.Log("Opening %s in your browser...", logger.Blue("%s", url))
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case tkn := <-srv.Token():
		if err := writeToken(tkn); err != nil {
			logger.Error("failed to persist token: %+v", err)
		}
	}

	return nil
}
