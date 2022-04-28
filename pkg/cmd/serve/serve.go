package serve

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/httpd"
	"github.com/spf13/cobra"
)

// Config is the httpd config.
type config struct {
	root        *cli.Config
	host        string
	port        int
	cmd         string
	parallelism int
	args        []string
}

const (
	defaultPort = 6000
)

// New returns a new execute cobra command.
func New(c *cli.Config) *cobra.Command {
	var cfg = config{root: c}

	cmd := &cobra.Command{
		Use:   "serve cmd... [--port] [--host] [--parallelism]",
		Short: "Start the Airplane runtime.",
		Long:  "Start an http server that implements the Airplane runtime.",
		Example: heredoc.Doc(`
			airplane serve ./my_script.sh
			airplane serve --port 5000 --host localhost -- python helloworld.py
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				cfg.cmd = args[0]
				cfg.args = args[1:]
			} else {
				return errors.New("expected 1 argument: airplane serve cmd...")
			}

			return run(cmd.Root().Context(), cfg)
		},
	}
	cmd.Flags().IntVar(&cfg.port, "port", defaultPort, "port to run listen on")
	// Unless localhost is specified, MacOS with firewall on will ask for approval every time server starts
	cmd.Flags().StringVar(&cfg.host, "host", "", "host to listen on")
	cmd.Flags().IntVar(&cfg.parallelism, "parallelism", 1, "how many concurrent processes can be started")
	// Hide this command until it's ready.
	cmd.Hidden = true
	return cmd
}

// Run runs the execute command.
func run(ctx context.Context, cfg config) error {
	// Keep track of the maximum number of parallel processes.
	slots := make(chan interface{}, cfg.parallelism)
	for i := 0; i < cfg.parallelism; i++ {
		slots <- true
	}
	serverDoneC := make(chan interface{})
	return httpd.ServeWithGracefulShutdown(
		ctx,
		&http.Server{
			Addr:    fmt.Sprintf("%s:%d", cfg.host, cfg.port),
			Handler: httpd.Route(cfg.cmd, cfg.args, slots, serverDoneC),
		},
		cfg.parallelism,
		slots,
		serverDoneC,
	)
}
