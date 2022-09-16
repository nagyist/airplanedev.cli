package views

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/views/viewdir"
	libbuild "github.com/airplanedev/lib/pkg/build"
	"github.com/pkg/errors"
)

const (
	hostEnvKey    = "AIRPLANE_API_HOST"
	tokenEnvKey   = "AIRPLANE_TOKEN"
	apiKeyEnvKey  = "AIRPLANE_API_KEY"
	envSlugEnvKey = "AIRPLANE_ENV_SLUG"
)

func Dev(v viewdir.ViewDirectory, viteOpts ViteOpts) (*exec.Cmd, string, error) {
	root := v.Root()
	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{WithLoader: true})
	defer l.StopLoader()

	l.Debug("Root directory: %s", v.Root())
	l.Debug("Entrypoint: %s", v.EntrypointPath())
	tmpdir := filepath.Join(root, ".airplane-view")
	if _, err := os.Stat(tmpdir); os.IsNotExist(err) {
		if err := os.Mkdir(tmpdir, 0755); err != nil {
			return nil, "", errors.Wrap(err, "creating temporary dir")
		}
		l.Debug("created temporary dir %s", tmpdir)
	} else {
		l.Debug("temporary dir %s exists", tmpdir)
	}

	entrypointFile, err := filepath.Rel(v.Root(), v.EntrypointPath())
	if err != nil {
		return nil, "", errors.Wrap(err, "figuring out entrypoint")
	}
	if err := createWrapperTemplates(tmpdir, entrypointFile); err != nil {
		return nil, "", err
	}

	// Read existing package.json file, copy it over with vite stuff.
	packageJSONFile, err := os.Open(filepath.Join(root, "package.json"))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", errors.Wrap(err, "opening package.json")
		}
	}
	var packageJSON interface{}
	if packageJSONFile != nil {
		if err := json.NewDecoder(packageJSONFile).Decode(&packageJSON); err != nil {
			return nil, "", errors.Wrap(err, "decoding package.json")
		}
	} else {
		packageJSON = map[string]interface{}{}
	}

	// Add relevant fields to package.json.
	packageJSONMap, ok := packageJSON.(map[string]interface{})
	if !ok {
		return nil, "", errors.New("expected package.json to be an object")
	}
	if err := patchPackageJSON(&packageJSONMap); err != nil {
		return nil, "", errors.Wrap(err, "patching package.json")
	}

	// Write package.json back out.
	newPackageJSONFile, err := os.Create(filepath.Join(tmpdir, "package.json"))
	if err != nil {
		return nil, "", errors.Wrap(err, "creating new package.json")
	}
	if err := json.NewEncoder(newPackageJSONFile).Encode(packageJSON); err != nil {
		return nil, "", errors.Wrap(err, "writing new package.json")
	}
	if err := newPackageJSONFile.Close(); err != nil {
		return nil, "", errors.Wrap(err, "closing new package.json file")
	}

	// Create vite config.
	if err := createViteConfig(tmpdir); err != nil {
		return nil, "", errors.Wrap(err, "creating vite config")
	}

	// Run npm/yarn install.
	useYarn := utils.ShouldUseYarn(tmpdir)
	if err := utils.InstallDependencies(tmpdir, useYarn); err != nil {
		l.Debug(err.Error())
		if useYarn {
			return nil, "", errors.New("error installing dependencies using yarn. Try installing yarn.")
		}
		return nil, "", errors.Wrap(err, "running npm/yarn install")
	}

	l.StopLoader()

	// Run vite.
	cmd, viteServer, err := runVite(viteOpts, tmpdir)
	if err != nil {
		return nil, "", errors.Wrap(err, "running vite")
	}

	return cmd, viteServer, nil
}

func createWrapperTemplates(tmpdir string, entrypointFile string) error {
	entrypointFile = filepath.Join("..", entrypointFile)
	if !strings.HasSuffix(entrypointFile, ".tsx") {
		return errors.New("expected entrypoint file to end in .tsx")
	}
	entrypointModule := entrypointFile[:len(entrypointFile)-4]

	indexHtmlPath := filepath.Join(tmpdir, "index.html")
	title := strings.Split(filepath.Base(entrypointFile), ".")[0]
	indexHtmlStr, err := libbuild.IndexHtmlString(title)
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

func patchPackageJSON(packageJSON *map[string]interface{}) error {
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

type ViteOpts struct {
	Client  *api.Client
	EnvSlug string
	TTY     bool
}

func runVite(opts ViteOpts, tmpdir string) (*exec.Cmd, string, error) {
	cmd := exec.Command("node_modules/.bin/vite", "dev")
	// TODO - View def might not be in the same location as the view itself. If
	// we decide to support this, use the entrypoint to determine where to run
	// the `dev` command.
	cmd.Dir = tmpdir
	cmd.Env = append(os.Environ(), getAdditionalEnvs(opts.Client.Host, opts.Client.APIKey, opts.Client.Token, opts.EnvSlug)...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", err
	}
	cmd.Stderr = cmd.Stdout
	scanner := bufio.NewScanner(stdout)
	var viteServer string
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		// TODO: write integration test to make sure this doesn't break
		viteServerRegex := regexp.MustCompile(`[>|➜]\s+Local:\s+(http.+)`)
		for scanner.Scan() {
			// Wait until Vite prints out the server URL.
			m := scanner.Text()
			logger.Debug(m)
			if submatch := viteServerRegex.FindStringSubmatch(m); submatch != nil {
				viteServer = submatch[1]
				if opts.TTY {
					logger.Log("Started development server at %s (^C to quit)", logger.Blue("%s", viteServer))
					logger.Log("Press ENTER to preview your view in the browser")

					fmt.Scanln()
					if ok := utils.Open(viteServer); !ok {
						logger.Log("Something went wrong. Try running the command with the --debug flag for more details.")
					}
				} else {
					// In non TTY mode the command is backgrounded - we return as soon as Vite tells us what its server URL is.
					break
				}
			}
		}
	}()

	if err = cmd.Start(); err != nil {
		return nil, "", err
	}
	logger.Debug("Started vite with process id: %v", cmd.Process.Pid)

	wg.Wait()

	if !opts.TTY {
		// Debug log in the background in non TTY mode so we always log out Vite logs.
		// If we don't do this the scanner buffer fills up and Vite crashes.
		go func() {
			for scanner.Scan() {
				m := scanner.Text()
				logger.Debug(m)
			}
		}()
	}

	return cmd, viteServer, nil
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
