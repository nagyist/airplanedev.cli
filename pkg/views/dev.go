package views

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/server/network"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/views/viewdir"
	libbuild "github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/utils/airplane_directory"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
)

const (
	hostEnvKey        = "AIRPLANE_API_HOST"
	tokenEnvKey       = "AIRPLANE_TOKEN"
	apiKeyEnvKey      = "AIRPLANE_API_KEY"
	envSlugEnvKey     = "AIRPLANE_ENV_SLUG"
	tunnelTokenEnvKey = "AIRPLANE_TUNNEL_TOKEN"
	depHashFile       = "dep-hash"
)

func Dev(ctx context.Context, v viewdir.ViewDirectoryInterface, viteOpts ViteOpts) (*exec.Cmd, string, io.Closer, error) {
	root := v.Root()
	l := logger.NewStdErrLogger(logger.StdErrLoggerOpts{WithLoader: true})
	defer l.StopLoader()

	err := utils.CheckNodeVersion()
	if err != nil {
		return nil, "", nil, err
	}

	l.Debug("Root directory: %s", v.Root())
	l.Debug("Entrypoint: %s", v.EntrypointPath())
	airplaneViewDir := filepath.Join(root, ".airplane-view")
	if err := ensureAirplaneViewDir(airplaneViewDir, l); err != nil {
		return nil, "", nil, err
	}

	viewSubdir := filepath.Join(airplaneViewDir, v.Slug())
	// Remove the previous view-specific subdirectory and its contents, if it exists.
	if err := os.RemoveAll(viewSubdir); err != nil {
		return nil, "", nil, errors.Wrap(err, "unable to remove previous view-specific subdir")
	}

	if err := os.Mkdir(viewSubdir, 0755); err != nil {
		return nil, "", nil, errors.Wrap(err, "creating view-specific subdir")
	}
	l.Debug("created view-specific subdir %s", viewSubdir)
	closer := airplane_directory.CloseFunc(func() error {
		return errors.Wrap(os.RemoveAll(viewSubdir), "unable to remove view-specific subdir")
	})
	defer func() {
		// If we encountered an error before returning, then we should remove the view-specific subdirectory ourselves.
		if err != nil {
			closer.Close()
		}
	}()

	entrypointFile, err := filepath.Rel(v.Root(), v.EntrypointPath())
	if err != nil {
		return nil, "", nil, errors.Wrap(err, "figuring out entrypoint")
	}
	if err := createWrapperTemplates(airplaneViewDir, viewSubdir, entrypointFile); err != nil {
		return nil, "", nil, err
	}

	// Create a package.json in the .airplane-view subdirectory with mandatory development dependencies.

	// Read in the root package.json
	rootPackageJSONFile, err := os.Open(filepath.Join(root, "package.json"))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", nil, errors.Wrap(err, "opening package.json")
		}
	}
	rootPackageJSON := libbuild.PackageJSON{}
	if rootPackageJSONFile != nil {
		if err := json.NewDecoder(rootPackageJSONFile).Decode(&rootPackageJSON); err != nil {
			return nil, "", nil, errors.Wrap(err, "decoding package.json")
		}
	}
	if rootPackageJSON.Dependencies == nil {
		rootPackageJSON.Dependencies = map[string]string{}
	}
	if rootPackageJSON.DevDependencies == nil {
		rootPackageJSON.DevDependencies = map[string]string{}
	}

	// Add relevant fields to the development package.json.
	devPackageJSON := libbuild.PackageJSON{
		Dependencies:    map[string]string{},
		DevDependencies: map[string]string{},
	}
	if err := addDevDepsToPackageJSON(rootPackageJSON, devPackageJSON); err != nil {
		return nil, "", nil, errors.Wrap(err, "patching package.json")
	}

	// Write the development package.json.
	newPackageJSONFile, err := os.Create(filepath.Join(airplaneViewDir, "package.json"))
	if err != nil {
		return nil, "", nil, errors.Wrap(err, "creating new package.json")
	}
	enc := json.NewEncoder(newPackageJSONFile)
	enc.SetIndent("", "  ")
	if err := enc.Encode(devPackageJSON); err != nil {
		return nil, "", nil, errors.Wrap(err, "writing new package.json")
	}
	if err := newPackageJSONFile.Close(); err != nil {
		return nil, "", nil, errors.Wrap(err, "closing new package.json file")
	}

	// Add postcss config if tailwind config is detected.
	tailwindConfig := filepath.Join(root, "tailwind.config.js")
	if _, err := os.Stat(tailwindConfig); err == nil {
		postcssConfigStr, err := libbuild.PostcssConfigString("../tailwind.config.js")
		if err != nil {
			return nil, "", nil, errors.Wrap(err, "loading postcss.config.js value")
		}
		postcssConfigPath := filepath.Join(airplaneViewDir, "postcss.config.js")
		if err := os.WriteFile(postcssConfigPath, []byte(postcssConfigStr), 0644); err != nil {
			return nil, "", nil, errors.Wrap(err, "writing postcss.config.js")
		}
	}

	// Create vite config.
	if err := createViteConfig(root, airplaneViewDir, viteOpts.Port, viteOpts.Token); err != nil {
		return nil, "", nil, errors.Wrap(err, "creating vite config")
	}

	l.Log("Installing development dependencies for view...")
	if err := utils.InstallDependencies(airplaneViewDir, utils.InstallOptions{
		Yarn: viteOpts.UsesYarn,
	}); err != nil {
		l.Debug(err.Error())
		if errors.Is(err, exec.ErrNotFound) {
			if viteOpts.UsesYarn {
				return nil, "", nil, errors.New("error installing dependencies using yarn. Try installing yarn.")
			} else {
				return nil, "", nil, errors.New("error installing dependencies using npm. Try installing npm.")
			}
		}
		return nil, "", nil, errors.Wrap(err, "running npm/yarn install")
	}
	l.Log("Done")

	// Run vite.
	cmd, viteServer, err := runVite(ctx, viteOpts, airplaneViewDir, v.Slug())
	if err != nil {
		return nil, "", nil, errors.Wrap(err, "running vite")
	}

	return cmd, viteServer, closer, nil
}

func createWrapperTemplates(airplaneViewDir string, viewSubdir string, entrypointFile string) error {
	// We use slashes instead of filepath.Join because this string is templated into
	// a JS file as an import.
	entrypointFile = fmt.Sprintf("../../%s", entrypointFile)
	entrypointModule := fsx.TrimExtension(entrypointFile)

	indexHtmlPath := filepath.Join(viewSubdir, "index.html")
	title := strings.Split(filepath.Base(entrypointFile), ".")[0]
	indexHtmlStr, err := libbuild.IndexHtmlString(title)
	if err != nil {
		return errors.Wrap(err, "loading index.html value")
	}
	if err := os.WriteFile(indexHtmlPath, []byte(indexHtmlStr), 0644); err != nil {
		return errors.Wrap(err, "writing index.html")
	}

	// We used to write the index.html file into .airplane-view/, as opposed to the view-specific subdirectory. Remove
	// it in case it exists.
	deprecatedIndexHTMLPath := filepath.Join(airplaneViewDir, "index.html")
	if err := os.RemoveAll(deprecatedIndexHTMLPath); err != nil {
		logger.Warning("unable to remove deprecated .airplane-view/index.html file: %v", err)
	}

	mainTsxStr, err := libbuild.MainTsxString(entrypointModule, true)
	if err != nil {
		return errors.Wrap(err, "loading main.tsx value")
	}
	mainTsxPath := filepath.Join(viewSubdir, "main.tsx")
	if err := os.WriteFile(mainTsxPath, []byte(mainTsxStr), 0644); err != nil {
		return errors.Wrap(err, "writing main.tsx")
	}

	// Similar to index.html, remove the old .airplane-view/main.tsx file.
	deprecatedMainTsxPath := filepath.Join(airplaneViewDir, "main.tsx")
	if err := os.RemoveAll(deprecatedMainTsxPath); err != nil {
		logger.Warning("unable to remove deprecated .airplane-view/main.tsx file: %v", err)
	}

	return nil
}

// CheckLockfile checks if we should use npm or yarn to install dependencies, and also if the hash of the lockfile has
// changed since the last time we ran vite.
func CheckLockfile(v viewdir.ViewDirectoryInterface, l logger.Logger) (usesYarn bool, hashesEqual bool, err error) {
	airplaneViewDir := filepath.Join(v.Root(), ".airplane-view")

	var prevHash string
	contents, err := os.ReadFile(filepath.Join(airplaneViewDir, depHashFile))
	if err != nil {
		if os.IsNotExist(err) {
			prevHash = ""
		} else {
			return usesYarn, false, errors.Wrap(err, "reading dependency hash file")
		}
	} else {
		prevHash = string(contents)
	}

	usesYarn = utils.ShouldUseYarn(airplaneViewDir)
	lockfile := "package-lock.json"
	if usesYarn {
		lockfile = "yarn.lock"
	}
	lockfileContents, err := os.ReadFile(filepath.Join(v.Root(), lockfile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			lockfileContents = []byte{}
		} else {
			return usesYarn, false, errors.Wrap(err, "reading package.json")
		}
	}

	sha := sha256.Sum256(lockfileContents)
	currHash := hex.EncodeToString(sha[:])

	if prevHash == currHash {
		return usesYarn, true, nil
	} else {
		// Write new dependency hash file.
		if err := ensureAirplaneViewDir(airplaneViewDir, l); err != nil {
			return usesYarn, false, err
		}

		if err := os.WriteFile(filepath.Join(airplaneViewDir, depHashFile), []byte(currHash), 0644); err != nil {
			return usesYarn, false, errors.Wrap(err, "writing dependency hash file")
		}
		return usesYarn, false, nil
	}
}

// ensureAirplaneViewDir ensures that the .airplane-view directory exists.
func ensureAirplaneViewDir(airplaneViewDir string, l logger.Logger) error {
	if _, err := os.Stat(airplaneViewDir); os.IsNotExist(err) {
		if err := os.Mkdir(airplaneViewDir, 0755); err != nil {
			return errors.Wrap(err, "creating .airplane-view dir")
		}
		l.Debug("created .airplane-view dir %s", airplaneViewDir)
	} else {
		l.Debug(".airplane-view dir %s exists", airplaneViewDir)
	}

	return nil
}

func createViteConfig(root string, airplaneViewDir string, port int, token *string) error {
	viteConfigStr, err := libbuild.ViteConfigString(libbuild.ViteConfigOpts{
		Root:  root,
		Port:  port,
		Token: token,
	})

	if err != nil {
		return errors.Wrap(err, "loading vite.config.ts value")
	}
	viteConfigPath := filepath.Join(airplaneViewDir, "vite.config.ts")
	if err := os.WriteFile(viteConfigPath, []byte(viteConfigStr), 0644); err != nil {
		return errors.Wrap(err, "writing vite.config.ts")
	}
	return nil
}

func addMissingDeps(depsToAdd map[string]string, existingDeps map[string]string, deps map[string]string, alwaysAdd bool) {
	for k, v := range depsToAdd {
		_, dependencyExists := existingDeps[k]
		if !dependencyExists || alwaysAdd {
			deps[k] = v
		}
	}
}

// addDevDepsToPackageJSON adds mandatory development dependencies to packageJSON. If any dependencies are already
// installed in existingPackageJSON they are not added.
func addDevDepsToPackageJSON(existingPackageJSON libbuild.PackageJSON, packageJSON libbuild.PackageJSON) error {
	var buildToolsPackageJSON libbuild.PackageJSON
	if err := json.Unmarshal([]byte(libbuild.BuildToolsPackageJSON), &buildToolsPackageJSON); err != nil {
		return errors.Wrap(err, "unmarshaling build tools package.json")
	}

	defaultDeps := map[string]string{
		"react":           buildToolsPackageJSON.Dependencies["react"],
		"react-dom":       buildToolsPackageJSON.Dependencies["react-dom"],
		"@airplane/views": buildToolsPackageJSON.Dependencies["@airplane/views"],
		"object-hash":     buildToolsPackageJSON.Dependencies["object-hash"],
	}
	defaultDevDeps := map[string]string{
		"@vitejs/plugin-react": buildToolsPackageJSON.Dependencies["@vitejs/plugin-react"],
		"vite":                 buildToolsPackageJSON.Dependencies["vite"],
	}

	existingDeps := existingPackageJSON.Dependencies
	deps := packageJSON.Dependencies
	addMissingDeps(defaultDeps, existingDeps, deps, false)

	existingDevDeps := existingPackageJSON.DevDependencies
	devDeps := packageJSON.DevDependencies
	addMissingDeps(defaultDevDeps, existingDevDeps, devDeps, true)

	return nil
}

func FindVitePort() (int, error) {
	return network.FindOpenPortFrom("localhost", 5173, 100)
}

type ViteOpts struct {
	Client               api.APIClient
	EnvSlug              string
	TTY                  bool
	RebundleDependencies bool
	UsesYarn             bool
	Port                 int
	Token                *string
}

func runVite(ctx context.Context, opts ViteOpts, airplaneViewDir string, viewSlug string) (*exec.Cmd, string, error) {
	// By default, Vite attempts to locate a config file `vite.config.ts` inside the project root, which it determines
	// based on the location of the index.html file. Because this is part of the view-specific subdirectory, Vite will
	// not find the config file there, and we instead need to tell it to use the config file one level higher, at
	// .airplane-view/vite.config.ts.
	args := []string{"dev", "--config", "vite.config.ts", viewSlug}

	if opts.RebundleDependencies {
		logger.Debug("package.json changed, forcing Vite to re-bundle")
		args = append(args, "--force")
	}

	if opts.Port == 0 {
		var err error
		opts.Port, err = FindVitePort()
		if err != nil {
			return nil, "", err
		}
	}
	logger.Debug("starting Vite server on port %d", opts.Port)
	// --port will find the next available port after the specified port, so we need to also specify --strictPort to
	// ensure that this is fixed.
	args = append(args, "--port", strconv.Itoa(opts.Port), "--strictPort")
	viteServer := fmt.Sprintf("http://localhost:%d", opts.Port)

	cmd := exec.Command(filepath.Join("node_modules", ".bin", "vite"), args...)
	cmd.Dir = airplaneViewDir
	cmd.Env = append(os.Environ(), getAdditionalEnvs(opts.Client.Host(), opts.Client.APIKey(), opts.Client.Token(), opts.EnvSlug, opts.Client.TunnelToken())...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", err
	}
	cmd.Stderr = cmd.Stdout
	scanner := bufio.NewScanner(stdout)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		// TODO: write integration test to make sure this doesn't break
		viteServerRegex := regexp.MustCompile(`.*http.*`)
		for scanner.Scan() {
			// Wait until Vite prints out the server URL.
			m := scanner.Text()
			logger.Debug(m)
			if viteServerRegex.MatchString(m) {
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

	// We wait in a separate goroutine and send back a signal to the original, so that
	// we can also check ctx.Done(), which will handle signals correct.
	quitCh := make(chan interface{})
	go func() {
		wg.Wait()
		quitCh <- struct{}{}
	}()
	select {
	case <-quitCh:
	case <-ctx.Done():
	}

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

func getAdditionalEnvs(host, apiKey, token, envSlug string, tunnelToken *string) []string {
	var envs []string
	if _, ok := os.LookupEnv(hostEnvKey); !ok && host != "" {
		if !strings.HasPrefix(host, "http") {
			// The local dev server currently only supports http.
			if strings.HasPrefix(host, "127.0.0.1") {
				host = "http://" + host
			} else {
				host = "https://" + host
			}
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

	if tunnelToken != nil {
		if _, ok := os.LookupEnv(tunnelTokenEnvKey); !ok {
			envs = append(envs, fmt.Sprintf("%s=%s", tunnelTokenEnvKey, *tunnelToken))
		}
	}

	return envs
}
