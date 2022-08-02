package dev

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/cli/cmd/airplane/auth/login"
	"github.com/airplanedev/cli/cmd/airplane/views/dev/viewdir"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	libbuild "github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type Config struct {
	Root      *cli.Config
	FileOrDir string
	Args      []string
	EnvSlug   string
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
				cfg.FileOrDir = wd
			} else {
				cfg.FileOrDir = args[0]
			}

			return Run(cmd.Root().Context(), cfg)
		},
	}

	cmd.Flags().StringVar(&cfg.EnvSlug, "env", "", "The slug of the environment to run the view against. Defaults to your team's default environment.")

	return cmd
}

func Run(ctx context.Context, cfg Config) error {
	if !fsx.Exists(cfg.FileOrDir) {
		return errors.Errorf("Unable to open: %s", cfg.FileOrDir)
	}

	return StartView(cfg)
}

const (
	hostEnvKey    = "AIRPLANE_API_HOST"
	tokenEnvKey   = "AIRPLANE_TOKEN"
	apiKeyEnvKey  = "AIRPLANE_API_KEY"
	envSlugEnvKey = "AIRPLANE_ENV_SLUG"
)

func findRoot(fileOrDir string) (string, error) {
	var err error
	fileOrDir, err = filepath.Abs(fileOrDir)
	if err != nil {
		return "", errors.Wrap(err, "getting absolute path")
	}
	fileInfo, err := os.Stat(fileOrDir)
	if err != nil {
		return "", errors.Wrapf(err, "describing %s", fileOrDir)
	}
	if !fileInfo.IsDir() {
		fileOrDir = filepath.Dir(fileOrDir)
	}

	if p, ok := fsx.Find(fileOrDir, "package.json"); ok {
		return p, nil
	}
	return filepath.Dir(fileOrDir), nil
}

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

// StartView starts a view development server.
func StartView(cfg Config) (rerr error) {
	ctx := context.Background()
	rootDir, err := findRoot(cfg.FileOrDir)
	if err != nil {
		return err
	}
	v, err := viewdir.NewViewDirectory(ctx, cfg.Root, rootDir, cfg.FileOrDir, cfg.EnvSlug)
	if err != nil {
		return err
	}
	root := v.Root()
	logger.Debug("Root directory: %s", v.Root())
	logger.Debug("Entrypoint: %s", v.EntrypointPath())
	tmpdir := v.CacheDir()
	if _, err := os.Stat(tmpdir); os.IsNotExist(err) {
		if err := os.Mkdir(tmpdir, 0755); err != nil {
			return errors.Wrap(err, "creating temporary dir")
		}
		logger.Debug("created temporary dir %s", tmpdir)

	} else {
		logger.Debug("temporary dir %s exists", tmpdir)
	}
	entrypointFile, err := filepath.Rel(v.Root(), v.EntrypointPath())
	if err != nil {
		return errors.Wrap(err, "figuring out entrypoint")
	}
	if err := createWrapperTemplates(tmpdir, entrypointFile); err != nil {
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

func createWrapperTemplates(tmpdir string, entrypointFile string) error {
	if !strings.HasSuffix(entrypointFile, ".tsx") {
		return errors.New("expected entrypoint file to end in .tsx")
	}
	entrypointModule := entrypointFile[:len(entrypointFile)-4]

	indexHtmlPath := filepath.Join(tmpdir, "index.html")
	// TODO(zhan): put the view slug instead of Airplane as the title.
	indexHtmlStr, err := libbuild.IndexHtmlString()
	if err != nil {
		return errors.Wrap(err, "loading index.html value")
	}
	if err := os.WriteFile(indexHtmlPath, []byte(indexHtmlStr), 0644); err != nil {
		return errors.Wrap(err, "writing index.html")
	}

	mainTsxStr, err := libbuild.MainTsxString(entrypointModule)
	if err != nil {
		return errors.Wrap(err, "loading main.tsx value")
	}
	mainTsxPath := filepath.Join(tmpdir, "main.tsx")
	if err := os.WriteFile(mainTsxPath, []byte(mainTsxStr), 0644); err != nil {
		return errors.Wrap(err, "writing main.tsx")
	}
	return nil
}

func createViteConfig(tmpdir string) error {
	viteConfigStr, err := libbuild.ViteConfigString()
	if err != nil {
		return errors.Wrap(err, "loading vite.config.ts value")
	}
	viteConfigPath := filepath.Join(tmpdir, "vite.config.ts")
	if err := os.WriteFile(viteConfigPath, []byte(viteConfigStr), 0644); err != nil {
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
		"@vitejs/plugin-react": "^2.0.0",
		"vite":                 "^3.0.0",
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
			logger.Debug(m)
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
		// TODO: write integration test to make sure this doesn't break
		viteServerRegex := regexp.MustCompile(`[>|➜]\s+Local:\s+(http.+)`)
		var viteServer string
		for scanner.Scan() {
			m := scanner.Text()
			logger.Debug(m)
			if submatch := viteServerRegex.FindStringSubmatch(m); submatch != nil {
				viteServer = submatch[1]
				logger.Log("Started development server at %s (^C to quit)", logger.Blue("%s", viteServer))
				logger.Log("Press ENTER to preview your view in the browser")

				fmt.Scanln()
				if ok := utils.Open(viteServer); !ok {
					logger.Log("Something went wrong. Try running the command with the --debug flag for more details.")
				}
			}
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
