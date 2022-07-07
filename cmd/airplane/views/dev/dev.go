package dev

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/cmd/airplane/views/dev/viewdir"
	"github.com/airplanedev/cli/pkg/cli"
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
func StartView(cfg Config) (rerr error) {
	v := viewdir.NewViewDirectory(cfg.Dir)
	root := v.Root()
	tmpdir := v.CacheDir()
	if _, err := os.Stat(tmpdir); os.IsNotExist(err) {
		if err := os.Mkdir(tmpdir, 0755); err != nil {
			return errors.Wrap(err, "creating temporary dir")
		}
		logger.Log("created temporary dir %s", tmpdir)
	} else {
		logger.Log("temporary dir %s exists", tmpdir)
	}

	if err := createWrapperTemplates(tmpdir); err != nil {
		return err
	}

	// Read existing package.json file, copy it over with vite stuff.
	packageJSONFile, err := os.Open(filepath.Join(root, "package.json"))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return errors.Wrap(err, "opening package.json")
		}
	}
	var packageJSON interface{}
	if packageJSONFile != nil {
		if err := json.NewDecoder(packageJSONFile).Decode(&packageJSON); err != nil {
			return errors.Wrap(err, "decoding package.json")
		}
	} else {
		packageJSON = map[string]interface{}{}
	}

	// Add relevant fields to package.json.
	packageJSONMap, ok := packageJSON.(map[string]interface{})
	if !ok {
		return errors.New("expected package.json to be an object")
	}
	if err := patchPackageJSON(&packageJSONMap, cfg); err != nil {
		return errors.Wrap(err, "patching package.json")
	}

	// Write package.json back out.
	newPackageJSONFile, err := os.Create(filepath.Join(tmpdir, "package.json"))
	if err != nil {
		return errors.Wrap(err, "creating new package.json")
	}
	if err := json.NewEncoder(newPackageJSONFile).Encode(packageJSON); err != nil {
		return errors.Wrap(err, "writing new package.json")
	}
	if err := newPackageJSONFile.Close(); err != nil {
		return errors.Wrap(err, "closing new package.json file")
	}

	// Create vite config.
	if err := createViteConfig(tmpdir); err != nil {
		return errors.Wrap(err, "creating vite config")
	}

	// Create symlink to original directory.
	symlinkPath := filepath.Join(tmpdir, "src")
	if stat, err := os.Lstat(symlinkPath); err == nil {
		if stat.Mode().Type() == fs.ModeSymlink {
			if err := os.Remove(symlinkPath); err != nil {
				return errors.Wrap(err, "deleting old symlink")
			}
		} else {
			return errors.New("non-symlink found at src/ location")
		}
	}
	if err := os.Symlink(root, symlinkPath); err != nil {
		return errors.Wrap(err, "creating symlink")
	}

	// TODO(zhan): try yarn first instead of npm.
	// Run npm install.
	if err := runNPMInstall(tmpdir); err != nil {
		return errors.Wrap(err, "running npm install")
	}

	// Run vite.
	if err := runVite(cfg, tmpdir); err != nil {
		return errors.Wrap(err, "running vite")
	}

	return nil
}

func createWrapperTemplates(tmpdir string) error {
	indexHtmlPath := filepath.Join(tmpdir, "index.html")
	// TODO(zhan): extract these into embeds.
	// TODO(zhan): put the view slug instead of Airplane as the title.
	if err := os.WriteFile(indexHtmlPath, []byte(heredoc.Doc(`
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<meta charset="UTF-8" />
			<link rel="icon" type="image/svg+xml" href="/src/favicon.svg" />
			<meta name="viewport" content="width=device-width, initial-scale=1.0" />
			<title>Airplane</title>
		</head>
		<body>
			<div id="root"></div>
			<script type="module" src="/main.tsx"></script>
		</body>
	</html>
	`)), 0644); err != nil {
		return errors.Wrap(err, "writing index.html")
	}
	mainTsxPath := filepath.Join(tmpdir, "main.tsx")
	// TODO(zhan): change App here to whatever the entrypoint is.
	if err := os.WriteFile(mainTsxPath, []byte(heredoc.Doc(`
	import { Container, Stack, ThemeProvider, ViewProvider, setEnvVars } from "@airplane/views";
	import React from "react";
	import ReactDOM from "react-dom/client";
	import App from "./src/App";

	setEnvVars(
	  import.meta.env.AIRPLANE_API_HOST || "https://api.airplane.dev",
		import.meta.env.AIRPLANE_TOKEN,
		import.meta.env.AIRPLANE_API_KEY,
		import.meta.env.AIRPLANE_ENV_SLUG,
	);
	ReactDOM.createRoot(document.getElementById("root")!).render(
		<React.StrictMode>
			<ThemeProvider>
				<ViewProvider>
					<Container size="xl" py={96}>
						<App />
					</Container>
				</ViewProvider>
			</ThemeProvider>
		</React.StrictMode>
	);
	`)), 0644); err != nil {
		return errors.Wrap(err, "writing main.tsx")
	}
	return nil
}

func createViteConfig(tmpdir string) error {
	viteConfigPath := filepath.Join(tmpdir, "vite.config.ts")
	if err := os.WriteFile(viteConfigPath, []byte(heredoc.Doc(`
		import react from "@vitejs/plugin-react";
		import { defineConfig } from "vite";

		export default defineConfig({
			plugins: [react()],
			envPrefix: "AIRPLANE_",
			resolve: {
				preserveSymlinks: true,
			},
			base: "",
			build: {
				assetsDir: "",
			},
		});
	`)), 0644); err != nil {
		return errors.Wrap(err, "writing vite.config.ts")
	}
	return nil
}

func addMissingDeps(defaultDeps map[string]string, deps *map[string]interface{}) {
	for k, v := range defaultDeps {
		if _, ok := (*deps)[k]; !ok {
			(*deps)[k] = v
		}
	}
}

func patchPackageJSON(packageJSON *map[string]interface{}, cfg Config) error {
	// TODO(zhan): fill out correct values.
	defaultDeps := map[string]string{
		"react":           "18.0.0",
		"react-dom":       "18.0.0",
		"@airplane/views": "*",
	}
	defaultDevDeps := map[string]string{
		"@vitejs/plugin-react": "^1.3.0",
		"vite":                 "^2.9.9",
	}

	deps, ok := (*packageJSON)["dependencies"].(map[string]interface{})
	if !ok {
		(*packageJSON)["dependencies"] = map[string]interface{}{}
		deps = (*packageJSON)["dependencies"].(map[string]interface{})
	}
	addMissingDeps(defaultDeps, &deps)

	devDeps, ok := (*packageJSON)["devDependencies"].(map[string]interface{})
	if !ok {
		(*packageJSON)["devDependencies"] = map[string]interface{}{}
		devDeps = (*packageJSON)["devDependencies"].(map[string]interface{})
	}
	addMissingDeps(defaultDevDeps, &devDeps)

	return nil
}

func runNPMInstall(tmpdir string) error {
	cmd := exec.Command("npm", "install")
	cmd.Dir = tmpdir

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

func runVite(cfg Config, tmpdir string) error {
	host := cfg.Root.Client.Host
	apiKey := cfg.Root.Client.APIKey
	token := cfg.Root.Client.Token
	envSlug := cfg.EnvSlug

	cmd := exec.Command("node_modules/.bin/vite", "dev")
	// TODO - View def might not be in the same location as the view itself. If
	// we decide to support this, use the entrypoint to determine where to run
	// the `dev` command.
	cmd.Dir = tmpdir
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
