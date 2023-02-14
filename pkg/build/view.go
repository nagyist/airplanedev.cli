package build

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/airplanedev/lib/pkg/deploy/discover/parser"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
)

// view creates a dockerfile for a view.
func view(root string, options KindOptions) (string, error) {
	// Assert that the entrypoint file exists:
	entrypoint, _ := options["entrypoint"].(string)
	if entrypoint == "" {
		return "", errors.New("expected an entrypoint")
	}
	if err := fsx.AssertExistsAll(filepath.Join(root, entrypoint)); err != nil {
		return "", err
	}

	// Assert that API host is set.
	apiHost, _ := options["apiHost"].(string)
	if apiHost == "" {
		return "", errors.New("expected an api host")
	}
	if !strings.HasPrefix(apiHost, "https://") {
		apiHost = "https://" + apiHost
	}

	// TODO: possibly support multiple build tools.
	base, err := getBaseNodeImage("", false)
	if err != nil {
		return "", err
	}

	mainTsxStr, err := MainTsxString("./src/"+entrypoint, false)
	if err != nil {
		return "", err
	}
	indexHtmlStr, err := IndexHtmlString("Airplane")
	if err != nil {
		return "", err
	}
	viteConfigStr, err := ViteConfigString(ViteConfigOpts{})
	if err != nil {
		return "", err
	}

	packageJSONPath := filepath.Join(root, "package.json")
	var packageJSON interface{}
	if fsx.Exists(packageJSONPath) {
		packageJSONFile, err := os.ReadFile(packageJSONPath)
		if err != nil {
			return "", errors.Wrap(err, "reading package JSON")
		}
		if err := json.Unmarshal([]byte(packageJSONFile), &packageJSON); err != nil {
			return "", errors.Wrap(err, "parsing package JSON")
		}
	}
	packageJSONMap, ok := packageJSON.(map[string]interface{})
	if !ok {
		packageJSON = map[string]interface{}{}
		packageJSONMap = packageJSON.(map[string]interface{})
	}

	packagesToCheck := []string{"vite", "@vitejs/plugin-react", "react", "react-dom", "@airplane/views"}
	packagesToAdd := []string{}
	deps, depsOk := packageJSONMap["dependencies"].(map[string]interface{})
	devDeps, devDepsOk := packageJSONMap["devDependencies"].(map[string]interface{})
	for _, pkg := range packagesToCheck {
		hasPackage := false
		if depsOk {
			if _, ok := deps[pkg]; ok {
				hasPackage = true
			}
		}
		if devDepsOk {
			if _, ok := devDeps[pkg]; ok {
				hasPackage = true
			}
		}
		if !hasPackage {
			packagesToAdd = append(packagesToAdd, pkg)
		}
	}
	if len(packagesToAdd) > 0 {
		if !depsOk {
			packageJSONMap["dependencies"] = map[string]interface{}{}
			deps = packageJSONMap["dependencies"].(map[string]interface{})
		}
		for _, pkg := range packagesToAdd {
			deps[pkg] = "*"
		}
	}

	packageJSONByte, err := json.Marshal(packageJSON)
	if err != nil {
		return "", errors.Wrap(err, "encoding new package.json")
	}

	cfg := struct {
		Base              string
		InstallCommand    string
		OutDir            string
		InlineMainTsx     string
		InlineIndexHtml   string
		InlineViteConfig  string
		APIHost           string
		InlinePackageJSON string
	}{
		Base: base,
		// Because the install command is running in the context of a docker build, the yarn cache
		// isn't used after the packages are installed, so we clean the cache to keep the image
		// lean. This doesn't apply to Yarn v2 (specifically Plug'n'Play), which uses the cache
		// directory for storing packages.
		InstallCommand:    "yarn install --non-interactive && yarn cache clean",
		OutDir:            "dist",
		InlineMainTsx:     inlineString(mainTsxStr),
		InlineIndexHtml:   inlineString(indexHtmlStr),
		InlineViteConfig:  inlineString(viteConfigStr),
		APIHost:           apiHost,
		InlinePackageJSON: inlineString(string(packageJSONByte)),
	}

	return applyTemplate(heredoc.Doc(`
		FROM {{.Base}} as builder
		WORKDIR /airplane

		COPY package*.json yarn.* /airplane/
		RUN {{.InlinePackageJSON}} > /airplane/package.json
		RUN {{.InstallCommand}}

		RUN mkdir /airplane/src/
		RUN {{.InlineIndexHtml}} > /airplane/index.html
		RUN {{.InlineMainTsx}} > /airplane/main.tsx
		RUN {{.InlineViteConfig}} > /airplane/vite.config.ts
		ENV AIRPLANE_API_HOST={{.APIHost}}

		COPY . /airplane/src/
		RUN /airplane/node_modules/.bin/vite build --outDir {{.OutDir}}
		RUN yarn list --pattern @airplane/views | grep @airplane/views | sed "s/^.*@airplane\/views@\(.*\)$/\1/" > /airplane/{{.OutDir}}/.airplane-views-version

		# Docker's minimal image - we just need an empty place to copy the build artifacts.
		FROM scratch
		COPY --from=builder /airplane/{{.OutDir}}/ .
	`), cfg)
}

// viewBundle creates a dockerfile for all views within a root.
func viewBundle(root string, buildContext BuildContext, options KindOptions, filesToBuild []string,
	filesToDiscover []string) (string, error) {
	// Assert that API host is set.
	apiHost, _ := options["apiHost"].(string)
	if apiHost == "" {
		return "", errors.New("expected an api host")
	}
	if !strings.HasPrefix(apiHost, "https://") {
		apiHost = "https://" + apiHost
	}

	useSlimImage := buildContext.Base == BuildBaseSlim
	nodeVersion := GetNodeVersion(options)
	base, err := getBaseNodeImage(nodeVersion, useSlimImage)
	if err != nil {
		return "", err
	}

	var filesToBuildWithoutExtension []string
	for _, fileToBuild := range filesToBuild {
		// We use the files without their extension to generate unique paths to a specific
		// view of the bundle. See gen_view.sh for more information on how this is used.
		filesToBuildWithoutExtension = append(filesToBuildWithoutExtension,
			fsx.TrimExtension(fileToBuild))
	}

	mainTsxStr, err := MainTsxString("{{.Entrypoint}}", false)
	if err != nil {
		return "", err
	}
	indexHtmlStr, err := IndexHtmlString("Airplane")
	if err != nil {
		return "", err
	}
	viteConfigStr, err := UniversalViteConfigString(filesToBuildWithoutExtension)
	if err != nil {
		return "", err
	}
	postcssConfigStr, err := PostcssConfigString("src/tailwind.config.js")
	if err != nil {
		return "", err
	}

	tailwindPath := filepath.Join(root, "tailwind.config.js")
	hasTailwind := fsx.Exists(tailwindPath)

	packageJSONPath := filepath.Join(root, "package.json")
	var packageJSON interface{}
	if fsx.Exists(packageJSONPath) {
		packageJSONFile, err := os.ReadFile(packageJSONPath)
		if err != nil {
			return "", errors.Wrap(err, "reading package JSON")
		}
		if err := json.Unmarshal([]byte(packageJSONFile), &packageJSON); err != nil {
			return "", errors.Wrap(err, "parsing package JSON")
		}
	}
	packageJSONMap, ok := packageJSON.(map[string]interface{})
	if !ok {
		packageJSON = map[string]interface{}{}
		packageJSONMap = packageJSON.(map[string]interface{})
	}

	packagesToAdd, err := getRequiredViewsDependencies([]string{packageJSONPath})
	if err != nil {
		return "", err
	}
	if len(packagesToAdd) > 0 {
		// Add required views dependencies to the root package.json.
		deps, depsOk := packageJSONMap["dependencies"].(map[string]interface{})
		if !depsOk {
			packageJSONMap["dependencies"] = map[string]interface{}{}
			deps = packageJSONMap["dependencies"].(map[string]interface{})
		}
		for pkg, version := range packagesToAdd {
			deps[pkg] = version
		}
	}

	packageJSONByte, err := json.Marshal(packageJSON)
	if err != nil {
		return "", errors.Wrap(err, "encoding new package.json")
	}

	// Workaround to get esbuild to not bundle dependencies.
	// See build.ExternalPackages for details.
	externalPackages, err := ExternalPackages([]string{packageJSONPath}, false)
	if err != nil {
		return "", err
	}
	var flags []string
	for _, dep := range externalPackages {
		flags = append(flags, fmt.Sprintf(`"%s"`, dep))
	}
	esbuildFlags := strings.Join(flags, ", ")

	directoryToBuildTo := ".airplane"

	// Generate a list of all of the files to discover
	var discoverEntrypoints []string
	for _, fileToDiscover := range filesToDiscover {
		fileToDiscoverExt := filepath.Ext(fileToDiscover)
		// These should point at the location that esbuild will build to.
		discoverEntrypoints = append(discoverEntrypoints,
			filepath.Join("/airplane", directoryToBuildTo, strings.TrimSuffix(fileToDiscover, fileToDiscoverExt)+".js"))
	}

	var buildEntrypoints []string
	for _, fileToBuild := range filesToBuild {
		buildEntrypoints = append(buildEntrypoints, fmt.Sprintf(`"%s"`, fileToBuild))
	}
	esbuildFilesToBuild := strings.Join(buildEntrypoints, ", ")

	// Add build tools.
	buildToolsPath := path.Join(root, ".airplane-build-tools")
	if err := os.MkdirAll(buildToolsPath, 0755); err != nil {
		return "", errors.Wrapf(err, "creating build tools path")
	}

	if err := os.WriteFile(path.Join(buildToolsPath, "gen_view.sh"), []byte(genViewStr), 0755); err != nil {
		return "", errors.Wrap(err, "writing gen view script")
	}
	if err := os.WriteFile(path.Join(buildToolsPath, "esbuild.js"), []byte(Esbuild), 0755); err != nil {
		return "", errors.Wrap(err, "writing esbuild script")
	}

	var buildToolsPackageJSON PackageJSON
	if err := json.Unmarshal([]byte(BuildToolsPackageJSON), &buildToolsPackageJSON); err != nil {
		return "", errors.Wrap(err, "unmarshaling build tools package.json")
	}

	if len(filesToDiscover) > 0 {
		// Generate parser and store on context
		parserPath := path.Join(buildToolsPath, "inlineParser.js")
		if err := os.WriteFile(parserPath, []byte(parser.NodeParserScript), 0755); err != nil {
			return "", errors.Wrap(err, "writing parser script")
		}
	}

	cfg := struct {
		Base                         string
		InstallCommand               string
		OutDir                       string
		InlineMainTsx                string
		InlineIndexHtml              string
		InlineViteConfig             string
		APIHost                      string
		InlinePackageJSON            string
		EsbuildFlags                 string
		FilesToBuild                 string
		FilesToBuildWithoutExtension string
		FilesToDiscover              string
		DirectoryToBuildTo           string
		NodeVersion                  string
		UseSlimImage                 bool
		HasTailwind                  bool
		InlinePostcssConfig          string
	}{
		Base: base,
		// Because the install command is running in the context of a docker build, the yarn cache
		// isn't used after the packages are installed, so we clean the cache to keep the image
		// lean. This doesn't apply to Yarn v2 (specifically Plug'n'Play), which uses the cache
		// directory for storing packages.
		InstallCommand:               "yarn install --non-interactive && yarn cache clean",
		OutDir:                       "dist",
		InlineMainTsx:                inlineString(mainTsxStr),
		InlineIndexHtml:              inlineString(indexHtmlStr),
		InlineViteConfig:             inlineString(viteConfigStr),
		APIHost:                      apiHost,
		InlinePackageJSON:            inlineString(string(packageJSONByte)),
		EsbuildFlags:                 esbuildFlags,
		FilesToBuild:                 esbuildFilesToBuild,
		FilesToBuildWithoutExtension: strings.Join(filesToBuildWithoutExtension, " "),
		FilesToDiscover:              strings.Join(discoverEntrypoints, " "),
		DirectoryToBuildTo:           directoryToBuildTo,
		NodeVersion:                  nodeVersion,
		UseSlimImage:                 useSlimImage,
		HasTailwind:                  hasTailwind,
		InlinePostcssConfig:          inlineString(postcssConfigStr),
	}

	return applyTemplate(heredoc.Doc(`
		FROM {{.Base}} as builder
		WORKDIR /airplane

		ENV AIRPLANE_API_HOST={{.APIHost}}

		{{if .UseSlimImage}}
		RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
			&& apt-get -y install --no-install-recommends \
				curl ca-certificates \
			&& apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/*
		{{end}}

		# Copy build tools.
		COPY .airplane-build-tools .airplane-build-tools/

		# Support setting BUILD_NPM_RC or BUILD_NPM_TOKEN to configure private registry auth
		ARG BUILD_NPM_RC
		ARG BUILD_NPM_TOKEN
		RUN [ -z "${BUILD_NPM_RC}" ] || echo "${BUILD_NPM_RC}" > .npmrc
		RUN [ -z "${BUILD_NPM_TOKEN}" ] || echo "//registry.npmjs.org/:_authToken=${BUILD_NPM_TOKEN}" > .npmrc

		# Copy and install dependencies.
		COPY package*.json yarn.* /airplane/
		RUN {{.InlinePackageJSON}} > /airplane/package.json && {{.InstallCommand}}
		{{if .HasTailwind}}
		RUN {{.InlinePostcssConfig}} > /airplane/postcss.config.js
		{{end}}

		# Copy all source code to /src.
		COPY . src/

		{{if .FilesToDiscover}}
		# Build and discover inline views.
		RUN cd src && node .airplane-build-tools/esbuild.js \
			'[{{.FilesToBuild}}]' \
			node{{.NodeVersion}} \
			'[{{.EsbuildFlags}}]' \
			"" \
			/airplane/{{.DirectoryToBuildTo}} \
			/airplane/src \
			true
		RUN node /airplane/.airplane-build-tools/inlineParser.js {{.FilesToDiscover}}
		{{end}}

		# Generate index.html and main.tsx for each entrypoint.
		RUN {{.InlineIndexHtml}} > index.html && {{.InlineMainTsx}} > main.tsx && .airplane-build-tools/gen_view.sh "{{.FilesToBuildWithoutExtension}}" index.html main.tsx

		# Copy in universal Vite config and build view
		RUN {{.InlineViteConfig}} > vite.config.ts && /airplane/node_modules/.bin/vite build --outDir {{.OutDir}}
		RUN yarn list --pattern @airplane/views | grep @airplane/views | sed "s/^.*@airplane\/views@\(.*\)$/\1/" > /airplane/{{.OutDir}}/.airplane-views-version

		# Docker's minimal image - we just need an empty place to copy the build artifacts.
		FROM scratch
		COPY --from=builder /airplane/{{.OutDir}}/ .
	`), cfg)
}

// getRequiredViewsDependencies gets dependencies that required to build and run a view.
func getRequiredViewsDependencies(packageJSONs []string) (map[string]string, error) {
	deps, err := ListDependenciesFromPackageJSONs(packageJSONs)
	if err != nil {
		return nil, err
	}

	var buildToolsPackageJSON PackageJSON
	if err := json.Unmarshal([]byte(BuildToolsPackageJSON), &buildToolsPackageJSON); err != nil {
		return nil, errors.Wrap(err, "unmarshaling build tools package.json")
	}

	requiredDeps := []string{"@airplane/views", "react", "react-dom", "vite", "esbuild", "@vitejs/plugin-react"}
	requiredDepsMap := make(map[string]string, len(requiredDeps))
	for _, de := range requiredDeps {
		requiredDepsMap[de] = buildToolsPackageJSON.Dependencies[de]
	}

	// Allow users to override dependencies.
	// TODO: don't do this. We want to install our own versions of "toolchain"
	// dependencies (esbuild, vite, etc) to ensure they're compatible with our
	// build system. We're doing this for now because we add these deps to the
	// user's package.json rather than creating a higher package.json.
	for dep := range deps {
		delete(requiredDepsMap, dep)
	}

	return requiredDepsMap, nil
}

//go:embed views/vite.config.ts
var viteConfigTemplateStr string

//go:embed views/universal-vite.config.ts
var universalViteConfigTemplateStr string

type ViteConfigOpts struct {
	Port  int
	Token *string
	Root  string
}

func BasePath(port int, token *string) string {
	var base string
	if port != 0 {
		// If a port is specified, we are proxying requests to Vite. Additionally, in local dev, we embed views in an
		// iframe, and Vite will by default request assets from the origin of the iframe. For example, if we are
		// proxying from localhost:4000, and Vite requests main.tsx, it will request it from localhost:4000/main.tsx.
		// Instead, we want it to request it from localhost:4000/dev/views/{port}/main.tsx, so that the request can be
		// properly forwarded to the Vite server (running on localhost:{port}). We can accomplish this by setting the
		// base to /dev/views/{port}/, which prefixes all HTTP request paths with `base`. For more information, see
		// https://vitejs.dev/config/server-options.html#server-base
		base = fmt.Sprintf("/dev/views/%d/", port)

		// If a token is specified, we are proxying requests to Vite, and we are using a token to authenticate the
		// request. The dev server has middleware that will verify the token, and then forward the request to Vite.
		// Vite does not support appending query params or headers to requests, so we need to include the token in the
		// base path. The resulting URL will be /dev/views/{port}/{token}/main.tsx.
		if token != nil {
			base = fmt.Sprintf("%s%s/", base, *token)
		}
	}
	return base
}

func ViteConfigString(opts ViteConfigOpts) (string, error) {
	return applyTemplate(viteConfigTemplateStr, struct {
		Base string
		Root string
	}{
		Base: BasePath(opts.Port, opts.Token),
		Root: opts.Root,
	})
}

func UniversalViteConfigString(entrypoints []string) (string, error) {
	return applyTemplate(universalViteConfigTemplateStr, struct {
		Entrypoints []string
	}{
		Entrypoints: entrypoints,
	})
}

//go:embed views/index.html
var indexHtmlTemplateStr string

//go:embed views/gen_view.sh
var genViewStr string

func IndexHtmlString(title string) (string, error) {
	return applyTemplate(indexHtmlTemplateStr, struct {
		Title string
	}{
		Title: title,
	})
}

//go:embed views/main.tsx
var mainTsxTemplateStr string

func MainTsxString(entrypoint string, isInStudio bool) (string, error) {
	entrypoint = strings.TrimSuffix(entrypoint, ".tsx")
	return applyTemplate(mainTsxTemplateStr, struct {
		Entrypoint string
		IsInStudio bool
	}{
		Entrypoint: entrypoint,
		IsInStudio: isInStudio,
	})
}

//go:embed views/postcss.config.js
var postcssConfigTemplateStr string

func PostcssConfigString(tailwindLocation string) (string, error) {
	return applyTemplate(postcssConfigTemplateStr, struct {
		TailwindLocation string
	}{
		TailwindLocation: tailwindLocation,
	})
}
