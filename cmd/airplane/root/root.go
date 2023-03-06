package root

import (
	"context"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/agents"
	"github.com/airplanedev/cli/cmd/airplane/apikeys"
	"github.com/airplanedev/cli/cmd/airplane/auth"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/cmd/airplane/auth/logout"
	"github.com/airplanedev/cli/cmd/airplane/configs"
	"github.com/airplanedev/cli/cmd/airplane/demo"
	"github.com/airplanedev/cli/cmd/airplane/root/deploy"
	"github.com/airplanedev/cli/cmd/airplane/root/initcmd"
	"github.com/airplanedev/cli/cmd/airplane/runs"
	"github.com/airplanedev/cli/cmd/airplane/tasks"
	"github.com/airplanedev/cli/cmd/airplane/tasks/dev"
	"github.com/airplanedev/cli/cmd/airplane/tasks/execute"
	"github.com/airplanedev/cli/cmd/airplane/version"
	"github.com/airplanedev/cli/cmd/airplane/views"
	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/flags"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/print"
	"github.com/airplanedev/cli/pkg/version/latest"
	"github.com/airplanedev/lib/pkg/utils/tracing"
	"github.com/airplanedev/trap"
	isatty "github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// New returns a new root cobra command.
func New() *cobra.Command {
	var output string
	client := api.NewClient(api.ClientOpts{})
	var cfg = &cli.Config{
		Client: &client,
	}

	cmd := &cobra.Command{
		Use:   "airplane <command>",
		Short: "Airplane CLI",
		Example: heredoc.Doc(`
			airplane init
			airplane dev ./path/to/tasks_views_workflows
			airplane deploy ./path/to/script
		`),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			c, err := conf.ReadDefaultUserConfig()
			if err == nil {
				cfg.Client.Token = c.Tokens[cfg.Client.Host]
			}
			cfg.Client.APIKey = conf.GetAPIKey()
			cfg.Client.TeamID = conf.GetTeamID()
			cfg.Client.Source = conf.GetSource()
			if err := analytics.Init(cfg); err != nil {
				logger.Debug("error in analytics.Init: %v", err)
			}
			cfg.Flagger = &flags.APIClient{Client: cfg.Client}

			switch output {
			case "json":
				print.DefaultFormatter = print.NewJSONFormatter()
			case "yaml":
				print.DefaultFormatter = print.YAML{}
			case "table":
				print.DefaultFormatter = print.Table{}
			default:
				return errors.New("--output must be (json|yaml|table)")
			}

			logger.EnableDebug = cfg.DebugMode
			trap.Printf = logger.Log

			// Log the version every time the CLI is run with `--debug`. This aligns
			// customer debugging output with a specific release of the CLI.
			logger.Debug(version.Version())
			latest.CheckLatest(cmd.Context(), &c)

			if cfg.WithTracing {
				authInfo, err := cfg.Client.AuthInfo(cmd.Context())
				if err != nil {
					logger.Warning("Error getting auth info for trace: %+v", err)
				}

				logger.Debug("Setting up traces with collector %s", cfg.TracingCollectorAddr)
				_, _, err = tracing.InitializeTracerProvider(
					cmd.Context(),
					tracing.TracerOpts{
						CollectorAddr: cfg.TracingCollectorAddr,
						TeamID:        authInfo.Team.ID,
						UserID:        authInfo.User.ID,
						UserEmail:     authInfo.User.Email,
						Version:       version.Version(),
					},
				)
				if err != nil {
					return errors.Wrap(err, "initializing tracing provider")
				}
			}

			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if cfg.WithTracing {
				tracerProvider, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)

				if ok && tracerProvider != nil {
					if err := tracerProvider.Shutdown(context.Background()); err != nil {
						logger.Warning("Error shutting down tracer provider: %+v", err)
					}
				}
			}
			analytics.Close()
		},
	}

	// Silence usage and errors.
	//
	// Allows us to control how the output looks like.
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	// Set usage, help functions.
	cmd.SetUsageFunc(usage)
	cmd.SetHelpFunc(help)
	cmd.SetVersionTemplate(version.Version() + "\n")

	// Persistent flags, set globally to all commands.
	cmd.PersistentFlags().StringVarP(&cfg.Client.Host, "host", "", api.Host, "Airplane API Host.")
	defaultFormat := "table"
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		defaultFormat = "json"
	}
	cmd.PersistentFlags().StringVarP(&output, "output", "o", defaultFormat, "The format to use for output (json|yaml|table).")
	cmd.PersistentFlags().BoolVar(&cfg.DebugMode, "debug", false, "Whether to produce debugging output.")
	cmd.PersistentFlags().BoolVar(&cfg.Dev, "dev", false, "Dev mode: warning, not guaranteed to work and subject to change.")
	if err := cmd.PersistentFlags().MarkHidden("dev"); err != nil {
		logger.Debug("error: %s", err)
	}
	cmd.PersistentFlags().StringVar(&cfg.TracingCollectorAddr, "tracing-addr", "otel-collector.airplane.dev:443", "Address of tracing collector. Only used if tracing is also enabled.")
	cmd.PersistentFlags().BoolVar(&cfg.WithTelemetry, "with-telemetry", false, "Whether to send debug telemetry to Airplane.")
	cmd.PersistentFlags().BoolVar(&cfg.WithTracing, "with-tracing", false, "Whether to send performance tracing data to Airplane.")
	cmd.PersistentFlags().BoolVarP(&cfg.Version, "version", "v", false, "Print the CLI version.")
	// Root commands:
	cmd.AddCommand(initcmd.New(cfg))
	cmd.AddCommand(deploy.New(cfg))

	// Aliases for popular namespaced commands:
	cmd.AddCommand(dev.New(cfg))
	cmd.AddCommand(execute.New(cfg))
	cmd.AddCommand(login.New(cfg))
	cmd.AddCommand(logout.New(cfg))

	// Sub-commands:
	cmd.AddCommand(agents.New(cfg))
	cmd.AddCommand(apikeys.New(cfg))
	cmd.AddCommand(auth.New(cfg))
	cmd.AddCommand(configs.New(cfg))
	cmd.AddCommand(demo.New(cfg))
	cmd.AddCommand(tasks.New(cfg))
	cmd.AddCommand(views.New(cfg))
	cmd.AddCommand(runs.New(cfg))
	cmd.AddCommand(version.New(cfg))

	return cmd
}
