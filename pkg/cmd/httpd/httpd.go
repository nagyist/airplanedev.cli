package httpd

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/alessio/shellescape"
	"github.com/spf13/cobra"
)

// Config is the httpd config.
type config struct {
	root *cli.Config
	host string
	port int
	cmd  string
	args []string
}

const (
	defaultPort = 6000
)

// New returns a new execute cobra command.
func New(c *cli.Config) *cobra.Command {
	var cfg = config{root: c}

	cmd := &cobra.Command{
		Use:   "httpdexec cmd [--port] [--host] [cmd_args...]",
		Short: "Start the Airplane runtime.",
		Long:  "Start an http server that implements the Airplane runtime.",
		Example: heredoc.Doc(`
			airplane httpdexec ./helloworld.py [-- <parameters...>]
			airplane httpdexec ./helloworld.py --port 5000 --host localhost [-- <parameters...>]
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				for i, arg := range args {
					args[i] = shellescape.Quote(arg)
				}
				cfg.cmd = args[0]
				cfg.args = args[1:]
			} else {
				return errors.New("expected 1 argument: airplane httpdexec [cmd]")
			}

			return run(cmd.Root().Context(), cfg)
		},
	}
	cmd.Flags().IntVar(&cfg.port, "port", defaultPort, "port to run listen on")
	// Unless localhost is specified, MacOS with firewall on will ask for approval every time server starts
	cmd.Flags().StringVar(&cfg.host, "httpdhost", "", "host to listen on")
	// Hide this command until it's ready.
	cmd.Hidden = true
	return cmd
}

// Run runs the execute command.
func run(ctx context.Context, cfg config) error {
	return ServeWithGracefulShutdown(
		ctx,
		&http.Server{
			Addr:    fmt.Sprintf("%s:%d", cfg.host, cfg.port),
			Handler: Route(cfg.cmd, cfg.args, map[string]*CmdExecutor{}),
		},
	)
}
