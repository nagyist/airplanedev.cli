package dev

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/cmd/auth/login"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type Config struct {
	Root    *cli.Config
	Dir     string
	Args    []string
	EnvSlug string
}

func New(c *cli.Config) *cobra.Command {
	var cfg = Config{Root: c}

	cmd := &cobra.Command{
		Use:   "dev [./path/to/directory]",
		Short: "Locally run a view",
		Long:  "Locally runs a view from the view's directory",
		Example: heredoc.Doc(`
			airplane dev
			airplane dev ./path/to/directory
		`),
		PersistentPreRunE: utils.WithParentPersistentPreRunE(func(cmd *cobra.Command, args []string) error {
			// TODO: update the `dev` command to work w/out internet access
			return login.EnsureLoggedIn(cmd.Root().Context(), c)
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				wd, err := os.Getwd()
				if err != nil {
					return errors.Wrap(err, "error determining current working directory")

				}
				cfg.Dir = wd
			} else {
				cfg.Dir = args[0]
			}

			return Run(cmd.Root().Context(), cfg)
		},
		Hidden: true,
	}

	cmd.Flags().StringVar(&cfg.EnvSlug, "env", "", "The slug of the environment to run the view against. Defaults to your team's default environment.")

	return cmd
}

func Run(ctx context.Context, cfg Config) error {
	if !fsx.Exists(cfg.Dir) {
		return errors.Errorf("Unable to open: %s", cfg.Dir)
	}

	fileInfo, err := os.Stat(cfg.Dir)
	if err != nil {
		return errors.Wrapf(err, "describing %s", cfg.Dir)
	}
	if !fileInfo.IsDir() {
		return errors.Errorf("%s is not a directory", cfg.Dir)
	}

	if err = IsView(cfg.Dir); err != nil {
		return err
	}
	return StartView(cfg)
}

// IsView returns whether the directory is the root directory of an Airplane View.
func IsView(dir string) error {
	// TODO check if we are nested inside of a View directory.
	contents, err := os.ReadDir(dir)
	if err != nil {
		return errors.Wrapf(err, "reading %s", dir)
	}

	for _, content := range contents {
		if definitions.IsViewDef(content.Name()) {
			return nil
		}
	}
	return errors.Errorf("%s is not an Airplane view. It is missing a view definition file", dir)
}

const (
	hostEnvKey    = "AIRPLANE_API_HOST"
	tokenEnvKey   = "AIRPLANE_TOKEN"
	apiKeyEnvKey  = "AIRPLANE_API_KEY"
	envSlugEnvKey = "AIRPLANE_ENV_SLUG"
)

// StartView starts a view development server.
func StartView(cfg Config) error {
	host := cfg.Root.Client.Host
	apiKey := cfg.Root.Client.APIKey
	token := cfg.Root.Client.Token
	envSlug := cfg.EnvSlug

	cmd := exec.Command("npm", "run", "dev")
	// TODO - View def might not be in the same location as the view itself. If
	// we decide to support this, use the entrypoint to determine where to run
	// the `dev` command.
	cmd.Dir = cfg.Dir
	cmd.Env = append(os.Environ(), getAdditionalEnvs(host, apiKey, token, envSlug)...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	scanner := bufio.NewScanner(stdout)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for scanner.Scan() {
			m := scanner.Text()
			logger.Log(m)
		}
	}()
	if err = cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}

func getAdditionalEnvs(host, apiKey, token, envSlug string) []string {
	var envs []string

	if _, ok := os.LookupEnv(hostEnvKey); !ok && host != "" {
		if !strings.HasPrefix(host, "http") {
			host = "https://" + host
		}
		envs = append(envs, fmt.Sprintf("%s=%s", hostEnvKey, host))
	}
	if _, ok := os.LookupEnv(envSlugEnvKey); !ok && envSlug != "" {
		envs = append(envs, fmt.Sprintf("%s=%s", envSlugEnvKey, envSlug))
	}
	if token != "" {
		if _, ok := os.LookupEnv(tokenEnvKey); !ok {
			envs = append(envs, fmt.Sprintf("%s=%s", tokenEnvKey, token))
		}
	} else if _, ok := os.LookupEnv(apiKeyEnvKey); !ok && apiKey != "" {
		envs = append(envs, fmt.Sprintf("%s=%s", apiKeyEnvKey, apiKey))
	}
	return envs
}
