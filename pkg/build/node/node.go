package node

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/airplanedev/lib/pkg/build/hooks"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/lib/pkg/build/utils"
	buildversions "github.com/airplanedev/lib/pkg/build/versions"
	"github.com/airplanedev/lib/pkg/deploy/config"
	"github.com/airplanedev/lib/pkg/deploy/discover/parser"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
)

const (
	defaultSDKVersion     = "~0.2"
	minWorkflowSDKVersion = "0.2.10"
	workflowRuntimePkg    = "@airplane/workflow-runtime"
)

type templateParams struct {
	Workdir                          string
	Base                             string
	HasPackageJSON                   bool
	UsesWorkspaces                   bool
	InlineTaskShim                   string
	InlineWorkerShim                 string
	InlineShimPackageJSON            string
	InlineWorkflowShimPackageJSON    string
	InlineWorkflowBundlerScript      string
	InlineWorkflowInterceptorsScript string
	InlineWorkflowShim               string
	IsWorkflow                       bool
	NodeVersion                      string

	// For bundle builds, External is a stringified JSON array of external dependencies. For non-bundle builds, External
	// is a string of space-separated external flags (e.g. --external:package).
	External     string
	Args         string
	UseSlimImage bool
	Esbuild      string
	Instructions string

	// Use Instructions instead of the below
	InstallCommand      string
	InstallRequiresCode bool
	PreInstallCommand   string
	PostInstallCommand  string
	PreInstallPath      string
	PostInstallPath     string
	PackageCopyCmds     []string

	// FilesToBuild is a string of space-separated js/ts files to esbuild (user code) for
	// running tasks and discovering inline configuration.
	FilesToBuild string
	// FilesToDiscover is a string of space-separated built js files to discover entity configs from.
	// These files are the output of esbuild on FilesToBuild.
	FilesToDiscover string
}

func GetNodeBundleBuildInstructions(
	root string,
	options buildtypes.KindOptions,
) (buildtypes.BuildInstructions, error) {
	var err error

	// For backwards compatibility, continue to build old Node tasks
	// in the same way. Tasks built with the latest CLI will set
	// shim=true which enables the new code path.
	if shim, ok := options["shim"].(string); !ok || shim != "true" {
		return getNodeLegacyBuildInstructions(root, options)
	}

	instructions := []buildtypes.InstallInstruction{
		// Support setting BUILD_NPM_RC or BUILD_NPM_TOKEN to configure private registry auth
		{
			Cmd: `[ -z "${BUILD_NPM_RC}" ] || echo "${BUILD_NPM_RC}" > .npmrc`,
		},
		{
			Cmd: `[ -z "${BUILD_NPM_TOKEN}" ] || echo "//registry.npmjs.org/:_authToken=${BUILD_NPM_TOKEN}" > .npmrc`,
		},
	}

	installInstructions, err := GetNodeInstallInstructions(root, "/airplane")
	if err != nil {
		return buildtypes.BuildInstructions{}, err
	}
	instructions = append(instructions, installInstructions...)

	instructions = append(instructions, buildtypes.InstallInstruction{
		Cmd: fmt.Sprintf(`mkdir -p /airplane/.airplane && \
			%s > /airplane/.airplane/esbuild.js`, utils.InlineString(Esbuild)),
	})

	return buildtypes.BuildInstructions{
		InstallInstructions: instructions,
		BuildArgs: []string{
			"BUILD_NPM_RC",
			"BUILD_NPM_TOKEN",
		},
	}, nil
}

func GetNodeInstallInstructions(
	root string,
	sourceCodeDest string,
) ([]buildtypes.InstallInstruction, error) {
	var err error
	var instructions []buildtypes.InstallInstruction

	rootPackageJSON := filepath.Join(root, "package.json")
	hasPackageJSON := fsx.AssertExistsAll(rootPackageJSON) == nil

	pathYarnLock := filepath.Join(root, "yarn.lock")
	isYarn := fsx.AssertExistsAll(pathYarnLock) == nil

	pathPackageLock := filepath.Join(root, "package-lock.json")
	hasPackageLock := fsx.AssertExistsAll(pathPackageLock) == nil

	dotYarn := filepath.Join(root, ".yarn")
	hasDotYarn := fsx.Exists(dotYarn)
	dotAirplaneDotYarn := filepath.Join(root, ".airplane.yarn")
	hasDotAirplaneDotYarn := fsx.Exists(dotAirplaneDotYarn)

	yarnRC := filepath.Join(root, ".yarnrc.yml")
	hasYarnRC := fsx.Exists(yarnRC)

	// This case is solely for testing purposes. We are unable to test .yarn
	// because of permissions errors with the Docker daemon.
	if hasDotAirplaneDotYarn {
		instructions = append(instructions, buildtypes.InstallInstruction{
			SrcPath: "./.airplane.yarn",
			DstPath: filepath.Join(sourceCodeDest, ".airplane.yarn") + string(filepath.Separator),
		})
	} else if hasDotYarn {
		instructions = append(instructions, buildtypes.InstallInstruction{
			SrcPath: "./.yarn",
			DstPath: filepath.Join(sourceCodeDest, ".yarn") + string(filepath.Separator),
		})
	}
	if hasYarnRC {
		instructions = append(instructions, buildtypes.InstallInstruction{
			SrcPath: ".yarnrc.yml",
			DstPath: sourceCodeDest,
		})
	}

	var pkg PackageJSON
	if hasPackageJSON {
		pkg, err = ReadPackageJSON(rootPackageJSON)
		if err != nil {
			return nil, err
		}
	}

	// Install hooks can only exist in the task root for bundle builds
	installHooks, err := hooks.GetInstallHooks("", root)
	if err != nil {
		return nil, err
	}

	packageJSONs, _, err := GetPackageJSONs(rootPackageJSON)
	if err != nil {
		return nil, err
	}

	var hasPackageInstallHooks bool
	if hasPackageJSON {
		packageCopyInstructions, err := GetPackageCopyInstructions(root, packageJSONs, sourceCodeDest)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, packageCopyInstructions...)

		// Check all files for pre- or post-install scripts. If there are any found, then
		// we to run the install with the entire codebase to be safe as opposed to
		// just the package.json and yarn files (since the postinstall scripts might assume
		// that all code is present).
		for _, packageJSONPath := range packageJSONs {
			hasPackageInstallHooks, err = hasInstallHooks(packageJSONPath)
			if err != nil {
				return nil, err
			}
			if hasPackageInstallHooks {
				break
			}
		}
	} else {
		// Just create an empty package.json in the root
		instructions = append(instructions, buildtypes.InstallInstruction{
			Cmd: fmt.Sprintf("echo '{}' > %s", filepath.Join(sourceCodeDest, "package.json")),
		})
	}

	var airplaneConfig config.AirplaneConfig
	hasAirplaneConfig := fsx.Exists(filepath.Join(root, config.FileName))
	if hasAirplaneConfig {
		airplaneConfig, err = config.NewAirplaneConfigFromFile(root)
		if err != nil {
			return nil, err
		}
	}

	preinstall := []buildtypes.InstallInstruction{}
	install := ""
	postinstall := []buildtypes.InstallInstruction{}
	if pkg.Settings.PreInstallCommand != "" {
		preinstall = append(preinstall, buildtypes.InstallInstruction{
			Cmd: pkg.Settings.PreInstallCommand,
		})
	} else if airplaneConfig.Javascript.PreInstall != "" {
		preinstall = append(preinstall, buildtypes.InstallInstruction{
			Cmd: airplaneConfig.Javascript.PreInstall,
		})
	} else if installHooks.PreInstallFilePath != "" {
		preinstall = append(preinstall, buildtypes.InstallInstruction{
			Cmd:        "./airplane_preinstall.sh",
			SrcPath:    installHooks.PreInstallFilePath,
			DstPath:    "airplane_preinstall.sh",
			Executable: true,
		})
	}

	if pkg.Settings.InstallCommand != "" {
		install = pkg.Settings.InstallCommand
	} else if airplaneConfig.Javascript.Install != "" {
		install = airplaneConfig.Javascript.Install
	}

	if pkg.Settings.PostInstallCommand != "" {
		postinstall = append(postinstall, buildtypes.InstallInstruction{
			Cmd: pkg.Settings.PostInstallCommand,
		})
	} else if airplaneConfig.Javascript.PostInstall != "" {
		postinstall = append(postinstall, buildtypes.InstallInstruction{
			Cmd: airplaneConfig.Javascript.PostInstall,
		})
	} else if installHooks.PostInstallFilePath != "" {
		postinstall = append(postinstall, buildtypes.InstallInstruction{
			Cmd:        "./airplane_postinstall.sh",
			SrcPath:    installHooks.PostInstallFilePath,
			DstPath:    "airplane_postinstall.sh",
			Executable: true,
		})
	}

	// For safety purposes, we need to install from the full code if either (1) there are any
	// hook scripts in the package.json files or (2) there's an airplane preinstall
	// command; any of these could be assuming that the full code is present. This prevents us
	// from caching the user's dependencies separately from their code.
	//
	// TODO: Investigate whether we can get around this by doing an npm or yarn install with
	// an '--ignore-scripts' flag, then run it again without this flag.
	installRequiresCode := hasPackageInstallHooks || len(preinstall) > 0

	if installRequiresCode {
		instructions = append(instructions, buildtypes.InstallInstruction{
			SrcPath: ".",
			DstPath: sourceCodeDest,
		})
	}

	instructions = append(instructions, preinstall...)

	installCmd := makeInstallCommand(makeInstallCommandReq{
		PkgInstallCommand: install,
		RootPackageJSON:   rootPackageJSON,
		IsYarn:            isYarn,
		HasPackageLock:    hasPackageLock,
	})
	instructions = append(instructions, buildtypes.InstallInstruction{
		Cmd: installCmd,
	})

	if !installRequiresCode {
		instructions = append(instructions, buildtypes.InstallInstruction{
			SrcPath: ".",
			DstPath: sourceCodeDest,
		})
	}

	instructions = append(instructions, postinstall...)

	return instructions, nil
}

func getNodeLegacyBuildInstructions(root string, options buildtypes.KindOptions) (buildtypes.BuildInstructions, error) {
	entrypoint, _ := options["entrypoint"].(string)
	main := filepath.Join(root, entrypoint)
	deps := filepath.Join(root, "package.json")
	yarnlock := filepath.Join(root, "yarn.lock")
	pkglock := filepath.Join(root, "package-lock.json")
	lang, _ := options["language"].(string)
	// `workdir` is fixed usually - `buildWorkdir` is a subdirectory of `workdir` if there's
	// `buildCommand` and is ultimately where `entrypoint` is run from.
	buildCommand, _ := options["buildCommand"].(string)
	buildDir, _ := options["buildDir"].(string)
	workdir := "/airplane"

	// Make sure that entrypoint and `package.json` exist.
	if err := fsx.AssertExistsAll(main, deps); err != nil {
		return buildtypes.BuildInstructions{}, err
	}

	instructions := []buildtypes.InstallInstruction{
		// Support setting BUILD_NPM_RC or BUILD_NPM_TOKEN to configure private registry auth
		{
			Cmd: `[ -z "${BUILD_NPM_RC}" ] || echo "${BUILD_NPM_RC}" > .npmrc`,
		},
		{
			Cmd: `[ -z "${BUILD_NPM_TOKEN}" ] || echo "//registry.npmjs.org/:_authToken=${BUILD_NPM_TOKEN}" > .npmrc`,
		},

		{
			SrcPath: ".",
			DstPath: workdir,
		},
	}

	// Determine the install command to use.
	if err := fsx.AssertExistsAll(pkglock); err == nil {
		instructions = append(instructions, buildtypes.InstallInstruction{
			Cmd: `npm install package-lock.json`,
		})
	} else if err := fsx.AssertExistsAll(yarnlock); err == nil {
		instructions = append(instructions, buildtypes.InstallInstruction{
			Cmd: `yarn install`,
		})
	}

	// Language specific.
	switch lang {
	case "typescript":
		if buildDir == "" {
			buildDir = ".airplane"
		}
		instructions = append(instructions, buildtypes.InstallInstruction{
			Cmd: `npm install -g typescript@4.1`,
		})
		instructions = append(instructions, buildtypes.InstallInstruction{
			Cmd: `[ -f tsconfig.json ] || echo '{"include": ["*", "**/*"], "exclude": ["node_modules"]}' >tsconfig.json`,
		})
		instructions = append(instructions, buildtypes.InstallInstruction{
			Cmd: fmt.Sprintf(`rm -rf %s && tsc --outDir %s --rootDir .`, buildDir, buildDir),
		})
		if buildCommand != "" {
			// It's not totally expected, but if you do set buildCommand we'll run it after tsc
			instructions = append(instructions, buildtypes.InstallInstruction{
				Cmd: buildCommand,
			})
		}
	case "javascript":
		if buildCommand != "" {
			instructions = append(instructions, buildtypes.InstallInstruction{
				Cmd: buildCommand,
			})
		}
	default:
		return buildtypes.BuildInstructions{}, errors.Errorf("build: unknown language %q, expected \"javascript\" or \"typescript\"", lang)
	}

	return buildtypes.BuildInstructions{
		InstallInstructions: instructions,
		BuildArgs: []string{
			"BUILD_NPM_RC",
			"BUILD_NPM_TOKEN",
		},
	}, nil
}

// Node creates a dockerfile for Node (typescript/javascript).
func Node(
	root string,
	options buildtypes.KindOptions,
	buildArgs []string,
) (string, error) {
	var err error

	// For backwards compatibility, continue to build old Node tasks
	// in the same way. Tasks built with the latest CLI will set
	// shim=true which enables the new code path.
	if shim, ok := options["shim"].(string); !ok || shim != "true" {
		return nodeLegacyBuilder(root, options)
	}

	// Assert that the entrypoint file exists:
	entrypoint, _ := options["entrypoint"].(string)
	if entrypoint == "" {
		return "", errors.New("expected an entrypoint")
	}
	if err := fsx.AssertExistsAll(filepath.Join(root, entrypoint)); err != nil {
		return "", err
	}

	workdir, _ := options["workdir"].(string)
	rootPackageJSON := filepath.Join(root, "package.json")
	hasPackageJSON := fsx.AssertExistsAll(rootPackageJSON) == nil

	pathYarnLock := filepath.Join(root, "yarn.lock")
	isYarn := fsx.AssertExistsAll(pathYarnLock) == nil

	pathPackageLock := filepath.Join(root, "package-lock.json")
	hasPackageLock := fsx.AssertExistsAll(pathPackageLock) == nil

	isWorkflow := isWorkflowRuntime(options)

	var pkg PackageJSON
	if hasPackageJSON {
		pkg, err = ReadPackageJSON(rootPackageJSON)
		if err != nil {
			return "", err
		}
	}

	var airplaneConfig config.AirplaneConfig
	hasAirplaneConfig := fsx.Exists(filepath.Join(root, config.FileName))
	if hasAirplaneConfig {
		airplaneConfig, err = config.NewAirplaneConfigFromFile(root)
		if err != nil {
			return "", err
		}
	}

	// Install hooks can only exist in the task root for bundle builds
	installHooks, err := hooks.GetInstallHooks(entrypoint, root)
	if err != nil {
		return "", err
	}
	preinstallCommand := pkg.Settings.PreInstallCommand
	if preinstallCommand == "" {
		preinstallCommand = airplaneConfig.Javascript.PreInstall
	}
	postInstallCommand := pkg.Settings.PostInstallCommand
	if postInstallCommand == "" {
		postInstallCommand = airplaneConfig.Javascript.PostInstall
	}
	installCommand := pkg.Settings.InstallCommand
	if installCommand == "" {
		installCommand = airplaneConfig.Javascript.Install
	}

	cfg := templateParams{
		Workdir:        workdir,
		HasPackageJSON: hasPackageJSON,
		UsesWorkspaces: len(pkg.Workspaces.Workspaces) > 0,
		// esbuild is relatively generous in the node versions it supports:
		// https://esbuild.github.io/api/#target
		NodeVersion:        GetNodeVersion(options),
		PreInstallCommand:  preinstallCommand,
		PostInstallCommand: postInstallCommand,
		Args:               makeArgsCommand(buildArgs),
		IsWorkflow:         isWorkflow,
		PreInstallPath:     installHooks.PreInstallFilePath,
		PostInstallPath:    installHooks.PostInstallFilePath,
	}

	packageJSONs, usesWorkspaces, err := GetPackageJSONs(rootPackageJSON)
	if err != nil {
		return "", err
	}

	var hasPackageInstallHooks bool

	if cfg.HasPackageJSON {
		cfg.PackageCopyCmds, err = GetPackageCopyCmds(root, packageJSONs, "/airplane")
		if err != nil {
			return "", err
		}

		// Check all files for pre- or post-install scripts. If there are any found, then
		// we to run the install with the entire codebase to be safe as opposed to
		// just the package.json and yarn files (since the postinstall scripts might assume
		// that all code is present).
		for _, packageJSONPath := range packageJSONs {
			hasPackageInstallHooks, err = hasInstallHooks(packageJSONPath)
			if err != nil {
				return "", err
			}
			if hasPackageInstallHooks {
				break
			}
		}
	} else {
		// Just create an empty package.json in the root
		cfg.PackageCopyCmds = []string{"RUN echo '{}' > /airplane/package.json"}
	}

	if cfg.HasPackageJSON {
		// Workaround to get esbuild to not bundle dependencies.
		// See build.ExternalPackages for details.
		deps, err := ExternalPackages(packageJSONs, usesWorkspaces)
		if err != nil {
			return "", err
		}
		var flags []string
		for _, dep := range deps {
			flags = append(flags, fmt.Sprintf("--external:%s", dep))
		}
		if isWorkflow {
			// Even if these are imported, we need to mark the root packages
			// as external for esbuild to work properly. Esbuild doesn't
			// care about repeats, so no need to dedupe.
			flags = append(flags, "--external:@temporalio", "--external:@swc")
		}

		cfg.External = strings.Join(flags, " ")
	}

	if !strings.HasPrefix(cfg.Workdir, "/") {
		cfg.Workdir = "/" + cfg.Workdir
	}

	baseImageType, _ := options["base"].(buildtypes.BuildBase)
	cfg.UseSlimImage = baseImageType == buildtypes.BuildBaseSlim
	cfg.Base, err = GetBaseNodeImage(cfg.NodeVersion, cfg.UseSlimImage)
	if err != nil {
		return "", err
	}

	pjson, err := GenShimPackageJSON(GenShimPackageJSONOpts{
		RootDir:      root,
		PackageJSONs: packageJSONs,
		IsWorkflow:   isWorkflow,
		IsBundle:     false,
	})
	if err != nil {
		return "", err
	}
	cfg.InlineShimPackageJSON = utils.InlineString(string(pjson))

	entrypointFunc, _ := options["entrypointFunc"].(string)
	if isWorkflow {
		cfg.InlineTaskShim = utils.InlineString(workerShim)
		cfg.InlineWorkflowBundlerScript = utils.InlineString(workflowBundlerScript)
		cfg.InlineWorkflowInterceptorsScript = utils.InlineString(workflowInterceptorsScript)

		workflowShimTemplated, err := TemplateEntrypoint(workflowShim, NodeShimParams{
			Entrypoint:     entrypoint,
			EntrypointFunc: entrypointFunc,
		})
		if err != nil {
			return "", err
		}
		cfg.InlineWorkflowShim = utils.InlineString(workflowShimTemplated)
	} else {
		shim, err := TemplatedNodeShim(NodeShimParams{
			Entrypoint:     entrypoint,
			EntrypointFunc: entrypointFunc,
		})
		if err != nil {
			return "", err
		}
		cfg.InlineTaskShim = utils.InlineString(shim)
	}

	cfg.InstallCommand = makeInstallCommand(makeInstallCommandReq{
		PkgInstallCommand: installCommand,
		RootPackageJSON:   rootPackageJSON,
		IsYarn:            isYarn,
		HasPackageLock:    hasPackageLock,
	})

	// For safety purposes, we need to install from the full code if either (1) there are any
	// hook scripts in the package.json files or (2) there's an airplane preinstall
	// command; any of these could be assuming that the full code is present. This prevents us
	// from caching the user's dependencies separately from their code.
	//
	// TODO: Investigate whether we can get around this by doing an npm or yarn install with
	// an '--ignore-scripts' flag, then run it again without this flag.
	cfg.InstallRequiresCode = hasPackageInstallHooks ||
		cfg.PreInstallCommand != "" ||
		cfg.PreInstallPath != ""

	// The following Dockerfile can build both JS and TS tasks. In general, we're
	// aiming for recent EC202x support and for support for import/export syntax.
	// The former is easier, since recent versions of Node have excellent coverage
	// of the ECMAScript spec. The latter could be achieved through ECMAScript
	// modules (ESM), but those are not well-supported within the Node community.
	// Basic functionality of ESM is also still in the experimental stage, such as
	// module resolution for relative paths (f.e. ./main.js vs. ./main). Therefore,
	// we have to fallback to a separate build step to offer import/export support.
	// We have a few options -- f.e. babel, tsc, or swc -- but we go with esbuild
	// since it is native to Go.
	//
	// Down the road, we may want to give customers more control over this build process
	// in which case we could introduce an extra step for performing build commands.
	return utils.ApplyTemplate(heredoc.Doc(`
		FROM {{.Base}}

		{{if .UseSlimImage}}
		RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
			&& apt-get -y install --no-install-recommends \
				curl ca-certificates \
			&& apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/*
		{{end}}

		ENV NODE_ENV=production
		WORKDIR /airplane{{.Workdir}}
		# Support setting BUILD_NPM_RC or BUILD_NPM_TOKEN to configure private registry auth
		ARG BUILD_NPM_RC
		ARG BUILD_NPM_TOKEN
		RUN [ -z "${BUILD_NPM_RC}" ] || echo "${BUILD_NPM_RC}" > .npmrc
		RUN [ -z "${BUILD_NPM_TOKEN}" ] || echo "//registry.npmjs.org/:_authToken=${BUILD_NPM_TOKEN}" > .npmrc
		# qemu (on m1 at least) segfaults while looking up a UID/GID for running
		# postinstall scripts. We run as root with --unsafe-perm instead, skipping
		# that lookup. Possibly could fix by building for linux/arm on m1 instead
		# of always building for linux/amd64.
		RUN npm install -g esbuild@0.12 --unsafe-perm

		# npm >= 7 will automatically install peer dependencies, even if they're satisfied by the root. This is
		# problematic because we need the @airplane/workflow-runtime package to register the workflow runtime in the
		# runtime map that is utilized by the user's code, and so we explicitly request legacy behavior in this
		# instance, which does not install peer dependencies by default.
		RUN mkdir -p /airplane/.airplane && \
			cd /airplane/.airplane && \
			{{.InlineShimPackageJSON}} > package.json && \
			npm install --legacy-peer-deps

		{{range .PackageCopyCmds}}
		{{.}}
		{{end}}

		{{if .InstallRequiresCode}}
		COPY . /airplane
		{{end}}

		{{.Args}}

		{{if .PreInstallCommand}}
		RUN {{.PreInstallCommand}}
		{{else if .PreInstallPath}}
		COPY {{ .PreInstallPath }} airplane_preinstall.sh
		RUN chmod +x airplane_preinstall.sh && ./airplane_preinstall.sh
		{{end}}

		RUN {{.InstallCommand}}

		{{if not .InstallRequiresCode}}
		COPY . /airplane
		{{end}}

		{{if .PostInstallCommand}}
		RUN {{.PostInstallCommand}}
		{{else if .PostInstallPath}}
		COPY {{ .PostInstallPath }} airplane_postinstall.sh
		RUN chmod +x airplane_postinstall.sh && ./airplane_postinstall.sh
		{{end}}

		{{if .IsWorkflow}}
		RUN {{.InlineWorkflowShim}} >> /airplane/.airplane/workflow-shim.js \
			&& {{.InlineWorkflowInterceptorsScript}} >> /airplane/.airplane/workflow-interceptors.js \
			&& {{.InlineWorkflowBundlerScript}} >> /airplane/.airplane/workflow-bundler.js
		RUN node /airplane/.airplane/workflow-bundler.js
		{{end}}

		RUN {{.InlineTaskShim}} > /airplane/.airplane/shim.js && \
			esbuild /airplane/.airplane/shim.js \
				--bundle \
				--platform=node {{.External}} \
				--target=node{{.NodeVersion}} \
				--outfile=/airplane/.airplane/dist/shim.js

		ENTRYPOINT ["node", "/airplane/.airplane/dist/shim.js"]
	`), cfg)
}

type ShimPackageJSON struct {
	Dependencies map[string]string `json:"dependencies"`
	Type         string            `json:"type,omitempty"`
}

type GenShimPackageJSONOpts struct {
	RootDir      string
	PackageJSONs []string
	IsWorkflow   bool
	IsBundle     bool
}

// GenShimPackageJSON generates the `package.json` that contains the dependencies required for the shim to run. If the
// dependency is satisfied by a parent directory (i.e. the user's code), then no need to include it here.
func GenShimPackageJSON(opts GenShimPackageJSONOpts) ([]byte, error) {
	existingDeps, err := ListDependenciesFromPackageJSONs(opts.PackageJSONs)
	if err != nil {
		return nil, err
	}

	var buildToolsPackageJSON PackageJSON
	if err := json.Unmarshal([]byte(BuildToolsPackageJSON), &buildToolsPackageJSON); err != nil {
		return nil, errors.Wrap(err, "unmarshaling build tools package.json")
	}

	shimDeps := []string{"airplane"}
	buildDeps := []string{}

	if opts.IsWorkflow {
		shimDeps = append(shimDeps, workflowRuntimePkg)
	}
	if opts.IsBundle {
		buildDeps = append(buildDeps, []string{"esbuild", "esbuild-plugin-tsc", "typescript", "jsdom"}...)
	}

	requiredDepsMap := make(map[string]string, len(shimDeps)+len(buildDeps))
	for _, de := range shimDeps {
		requiredDepsMap[de] = buildToolsPackageJSON.Dependencies[de]
	}

	// Always keep the versions of airplane and @airplane/workflow-runtime in sync, unless the task's dependencies
	// explicitly include @airplane/workflow-runtime.
	if opts.IsWorkflow {
		if depVersion, containsAirplane := existingDeps["airplane"]; containsAirplane {
			apVersion := getLockPackageVersion(opts.RootDir, "airplane", depVersion)
			if _, containsWorkflowRuntime := existingDeps[workflowRuntimePkg]; !containsWorkflowRuntime {
				requiredDepsMap[workflowRuntimePkg] = apVersion
			}
		}
	}

	// Allow users to override shim dependencies. If the user has specified a dependency, we won't
	// install it for them and will rely on their version instead.
	for dep := range existingDeps {
		delete(requiredDepsMap, dep)
	}
	// Don't allow users to override build dependencies.
	for _, de := range buildDeps {
		requiredDepsMap[de] = buildToolsPackageJSON.Dependencies[de]
	}

	pjson := ShimPackageJSON{
		Dependencies: requiredDepsMap,
	}
	b, err := json.Marshal(pjson)
	return b, errors.Wrap(err, "marshalling shim dependencies")
}

func GetNodeVersion(opts buildtypes.KindOptions) string {
	if opts == nil || opts["nodeVersion"] == nil {
		return string(buildtypes.DefaultNodeVersion)
	}
	nv, ok := opts["nodeVersion"].(string)
	if !ok {
		return string(buildtypes.DefaultNodeVersion)
	}
	if nv == "" {
		return string(buildtypes.DefaultNodeVersion)
	}

	return nv
}

//go:embed node-shim.js
var nodeShim string

//go:embed universal-node-shim.js
var UniversalNodeShim string

//go:embed workflow/worker-shim.js
var workerShim string

//go:embed workflow/workflow-bundler.js
var workflowBundlerScript string

//go:embed workflow/workflow-interceptors.js
var workflowInterceptorsScript string

//go:embed workflow/workflow-shim.js
var workflowShim string

//go:embed workflow/universal-workflow-shim.js
var universalWorkflowShim string

//go:embed esbuild.js
var Esbuild string

//go:embed package.json
var BuildToolsPackageJSON string

type NodeShimParams struct {
	Entrypoint     string
	EntrypointFunc string
}

func TemplatedNodeShim(params NodeShimParams) (string, error) {
	return TemplateEntrypoint(nodeShim, params)
}

func TemplateEntrypoint(script string, params NodeShimParams) (string, error) {
	// Remove the `.ts` suffix if one exists, since tsc doesn't accept
	// import paths with `.ts` endings. `.js` endings are fine.
	entrypoint := strings.TrimSuffix(params.Entrypoint, ".ts")
	// The shim is stored under the .airplane directory.
	entrypoint = filepath.Join("../", entrypoint)
	// Escape for embedding into a string
	entrypoint = utils.BackslashEscape(entrypoint, `"`)

	shim, err := utils.ApplyTemplate(script, struct {
		Entrypoint     string
		EntrypointFunc string
	}{
		Entrypoint:     entrypoint,
		EntrypointFunc: params.EntrypointFunc,
	})
	if err != nil {
		return "", errors.Wrap(err, "templating shim")
	}

	return shim, nil
}

// nodeLegacyBuilder creates a dockerfile for Node (typescript/javascript).
//
// TODO(amir): possibly just run `npm start` instead of exposing lots
// of options to users?
func nodeLegacyBuilder(root string, options buildtypes.KindOptions) (string, error) {
	instructions, err := getNodeLegacyBuildInstructions(root, options)
	if err != nil {
		return "", err
	}

	entrypoint, _ := options["entrypoint"].(string)
	lang, _ := options["language"].(string)
	// `workdir` is fixed usually - `buildWorkdir` is a subdirectory of `workdir` if there's
	buildDir, _ := options["buildDir"].(string)
	workdir := "/airplane"
	buildWorkdir := "/airplane"

	// Language specific.
	switch lang {
	case "typescript":
		if buildDir == "" {
			buildDir = ".airplane"
		}
		buildWorkdir = path.Join(workdir, buildDir)
		// If entrypoint ends in .ts, replace it with .js
		entrypoint = strings.TrimSuffix(entrypoint, ".ts") + ".js"
	case "javascript":
		if buildDir != "" {
			buildWorkdir = path.Join(workdir, buildDir)
		}
	default:
		return "", errors.Errorf("build: unknown language %q, expected \"javascript\" or \"typescript\"", lang)
	}
	entrypoint = path.Join(buildWorkdir, entrypoint)

	baseImage, err := GetBaseNodeImage(GetNodeVersion(options), false)
	if err != nil {
		return "", err
	}

	dockerfileInstructions, err := instructions.DockerfileString()
	if err != nil {
		return "", err
	}

	return utils.ApplyTemplate(heredoc.Doc(`
		FROM {{ .Base }}
		WORKDIR {{ .Workdir }}
		{{ .Instructions }}
		WORKDIR {{ .BuildWorkdir }}
		ENTRYPOINT ["node", "{{ .Main }}"]
	`), struct {
		Base         string
		Workdir      string
		BuildWorkdir string
		Instructions string
		Main         string
	}{
		Base:         baseImage,
		Workdir:      workdir,
		BuildWorkdir: buildWorkdir,
		Instructions: dockerfileInstructions,
		Main:         entrypoint,
	})
}

func GetBaseNodeImage(version string, slim bool) (string, error) {
	if version == "" {
		version = string(buildtypes.DefaultNodeVersion)
	}
	v, err := buildversions.GetVersion(buildtypes.NameNode, version, slim)
	if err != nil {
		return "", err
	}
	base := v.String()
	if base == "" {
		// Assume the version is already a more-specific version - default to just returning it back
		base = "node:" + version + "-buster"
		if slim {
			base += "-slim"
		}
	}

	return base, nil
}

// Settings represent Airplane specific settings.
type Settings struct {
	Root               string `json:"root"`
	InstallCommand     string `json:"install"`
	PreInstallCommand  string `json:"preinstall"`
	PostInstallCommand string `json:"postinstall"`
}

type PackageJSON struct {
	Name                 string                 `json:"name"`
	Settings             Settings               `json:"airplane"`
	Workspaces           PackageJSONWorkspaces  `json:"workspaces"`
	Scripts              map[string]interface{} `json:"scripts"`
	Engines              PackageJSONEngines     `json:"engines"`
	Dependencies         map[string]string      `json:"dependencies"`
	DevDependencies      map[string]string      `json:"devDependencies"`
	OptionalDependencies map[string]string      `json:"optionalDependencies"`
}

type PackageJSONEngines struct {
	NodeVersion string `json:"node"`
}

type PackageJSONWorkspaces struct {
	Workspaces []string
}

func (p *PackageJSONWorkspaces) UnmarshalJSON(data []byte) error {
	// Workspaces might be an array of strings...
	var workspaces []string
	if err := json.Unmarshal(data, &workspaces); err == nil {
		p.Workspaces = workspaces
		return nil
	}

	// Or it might be an object with an array of strings.
	var workspacesObject struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(data, &workspacesObject); err != nil {
		return err
	}
	p.Workspaces = workspacesObject.Packages
	return nil
}

// ReadPackageJSON reads a directory containing a package.json or a package.json file itself
// into a PackageJSON. If there is no package.json, an os.ErrNotExist is returned.
func ReadPackageJSON(path string) (PackageJSON, error) {
	packageJSONPath := path

	fileInfo, err := os.Stat(path)
	if err != nil {
		return PackageJSON{}, err
	}

	if fileInfo.IsDir() {
		packageJSONPath = filepath.Join(path, "package.json")
	}

	f, err := os.Open(packageJSONPath)
	if err != nil {
		return PackageJSON{}, errors.Wrap(err, "opening package.json")
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return PackageJSON{}, errors.Wrap(err, "reading package.json")
	}

	var p PackageJSON
	if err := json.Unmarshal(b, &p); err != nil {
		return PackageJSON{}, errors.Wrap(err, "unmarshaling package.json")
	}

	return p, nil
}

// NodeBundle creates a dockerfile for all Node tasks/workflows within a task root (typescript/javascript).
func NodeBundle(
	root string,
	buildContext buildtypes.BuildContext,
	options buildtypes.KindOptions,
	buildArgs []string,
	filesToBuild []string,
	filesToDiscover []string,
) (string, error) {
	var err error

	// For backwards compatibility, continue to build old Node tasks
	// in the same way. Tasks built with the latest CLI will set
	// shim=true which enables the new code path.
	if shim, ok := options["shim"].(string); !ok || shim != "true" {
		return nodeLegacyBuilder(root, options)
	}

	instructions, err := GetNodeBundleBuildInstructions(root, options)
	if err != nil {
		return "", err
	}

	workdir, _ := options["workdir"].(string)
	rootPackageJSON := filepath.Join(root, "package.json")
	hasPackageJSON := fsx.AssertExistsAll(rootPackageJSON) == nil

	var pkg PackageJSON
	if hasPackageJSON {
		pkg, err = ReadPackageJSON(rootPackageJSON)
		if err != nil {
			return "", err
		}
	}

	type TaskImport struct {
		CompiledFile string
		UserFile     string
	}

	var taskImports []TaskImport
	for _, file := range filesToBuild {
		fileToBuildExt := filepath.Ext(file)
		compiledFile := strings.TrimSuffix(file, fileToBuildExt) + ".js"
		taskImports = append(taskImports, TaskImport{
			CompiledFile: compiledFile,
			UserFile:     file,
		})
	}

	universalWorkflowShimTemplated, err := utils.ApplyTemplate(universalWorkflowShim, struct {
		Entrypoints []string
		TaskImports []TaskImport
	}{
		Entrypoints: filesToBuild,
		TaskImports: taskImports,
	})
	if err != nil {
		return "", err
	}

	dockerfileInstructions, err := instructions.DockerfileString()
	if err != nil {
		return "", err
	}

	cfg := templateParams{
		Workdir:        workdir,
		HasPackageJSON: hasPackageJSON,
		UsesWorkspaces: len(pkg.Workspaces.Workspaces) > 0,
		// esbuild is relatively generous in the node versions it supports:
		// https://esbuild.github.io/api/#target
		NodeVersion:                      string(buildContext.VersionOrDefault()),
		Args:                             makeArgsCommand(buildArgs),
		Instructions:                     dockerfileInstructions,
		InlineTaskShim:                   utils.InlineString(UniversalNodeShim),
		InlineWorkerShim:                 utils.InlineString(workerShim),
		InlineWorkflowShim:               utils.InlineString(universalWorkflowShimTemplated),
		InlineWorkflowBundlerScript:      utils.InlineString(workflowBundlerScript),
		InlineWorkflowInterceptorsScript: utils.InlineString(workflowInterceptorsScript),
	}

	// Generate a list of all of the files to build
	var buildEntrypoints []string
	for _, fileToBuild := range filesToBuild {
		buildEntrypoints = append(buildEntrypoints, filepath.Join("/airplane", fileToBuild))
	}
	filesToBuildBytes, err := json.Marshal(buildEntrypoints)
	if err != nil {
		return "", errors.Wrap(err, "marshaling build entrypoints")
	}
	cfg.FilesToBuild = string(filesToBuildBytes)

	// Generate a list of all of the files to discover
	var discoverEntrypoints []string
	for _, fileToDiscover := range filesToDiscover {
		fileToDiscoverExt := filepath.Ext(fileToDiscover)
		// esbuild will output entrypoint bundles to /airplane/.airplane
		discoverEntrypoints = append(discoverEntrypoints,
			filepath.Join("/airplane/.airplane", strings.TrimSuffix(fileToDiscover, fileToDiscoverExt)+".js"))
	}
	cfg.FilesToDiscover = strings.Join(discoverEntrypoints, " ")

	packageJSONs, usesWorkspaces, err := GetPackageJSONs(rootPackageJSON)
	if err != nil {
		return "", err
	}

	if cfg.HasPackageJSON {
		// Workaround to get esbuild to not bundle dependencies.
		// See build.ExternalPackages for details.
		externalDeps, err := ExternalPackages(packageJSONs, usesWorkspaces)
		if err != nil {
			return "", err
		}

		// Even if these are imported, we need to mark the root packages
		// as external for esbuild to work properly. Esbuild doesn't
		// care about repeats, so no need to dedupe.
		externalDeps = append(externalDeps, "@temporalio", "@swc")
		externalDepsBytes, err := json.Marshal(externalDeps)
		if err != nil {
			return "", errors.Wrap(err, "marshaling external deps")
		}
		cfg.External = string(externalDepsBytes)
	}

	if !strings.HasPrefix(cfg.Workdir, "/") {
		cfg.Workdir = "/" + cfg.Workdir
	}

	cfg.UseSlimImage = buildContext.Base == buildtypes.BuildBaseSlim
	cfg.Base, err = GetBaseNodeImage(cfg.NodeVersion, cfg.UseSlimImage)
	if err != nil {
		return "", err
	}

	pjson, err := GenShimPackageJSON(GenShimPackageJSONOpts{
		RootDir:      root,
		PackageJSONs: packageJSONs,
		IsWorkflow:   false,
		IsBundle:     true,
	})
	if err != nil {
		return "", err
	}
	cfg.InlineShimPackageJSON = utils.InlineString(string(pjson))

	workflowpjson, err := GenShimPackageJSON(GenShimPackageJSONOpts{
		RootDir:      root,
		PackageJSONs: packageJSONs,
		IsWorkflow:   true,
		IsBundle:     true,
	})
	if err != nil {
		return "", err
	}
	cfg.InlineWorkflowShimPackageJSON = utils.InlineString(string(workflowpjson))

	if len(filesToDiscover) > 0 {
		// Generate parser and store on context
		parserPath := path.Join(root, ".airplane-build-tools", "inlineParser.cjs")
		if err := os.MkdirAll(path.Dir(parserPath), 0755); err != nil {
			return "", errors.Wrapf(err, "creating parser file")
		}
		if err := os.WriteFile(parserPath, []byte(parser.NodeParserScript), 0755); err != nil {
			return "", errors.Wrap(err, "writing parser script")
		}
	}

	// The following Dockerfile can build both JS and TS tasks. In general, we're
	// aiming for recent EC202x support and for support for import/export syntax.
	// The former is easier, since recent versions of Node have excellent coverage
	// of the ECMAScript spec. The latter could be achieved through ECMAScript
	// modules (ESM), but those are not well-supported within the Node community.
	// Basic functionality of ESM is also still in the experimental stage, such as
	// module resolution for relative paths (f.e. ./main.js vs. ./main). Therefore,
	// we have to fallback to a separate build step to offer import/export support.
	// We have a few options -- f.e. babel, tsc, or swc -- but we go with esbuild
	// since it is native to Go.
	//
	// Down the road, we may want to give customers more control over this build process
	// in which case we could introduce an extra step for performing build commands.
	return utils.ApplyTemplate(heredoc.Doc(`
		FROM {{.Base}} as base
		ENV NODE_ENV=production
		WORKDIR /airplane{{.Workdir}}

		{{if .UseSlimImage}}
		RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
			&& apt-get -y install --no-install-recommends \
				curl ca-certificates \
			&& apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/*
		{{end}}

		{{.Args}}
		{{.Instructions}}

		FROM base as workflow-build
		ENV NODE_ENV=production
		WORKDIR /airplane{{.Workdir}}

		RUN mkdir -p /airplane/.airplane && \
			cd /airplane/.airplane && \
			{{.InlineWorkflowShimPackageJSON}} > package.json && \
			npm install --legacy-peer-deps

		RUN {{.InlineWorkerShim}} > /airplane/.airplane/universal-shim.js && \
			node /airplane/.airplane/esbuild.js \
			'["/airplane/.airplane/universal-shim.js"]' \
			node{{.NodeVersion}} \
			'{{.External}}' \
			/airplane/.airplane/dist/universal-shim.js

		RUN {{.InlineWorkflowShim}} >> /airplane/.airplane/workflow-shim.js \
			&& {{.InlineWorkflowInterceptorsScript}} >> /airplane/.airplane/workflow-interceptors.js \
			&& {{.InlineWorkflowBundlerScript}} >> /airplane/.airplane/workflow-bundler.js
		RUN node /airplane/.airplane/workflow-bundler.js
		ENTRYPOINT ["node", "/airplane/.airplane/dist/universal-shim.js"]

		FROM base as task-build
		ENV NODE_ENV=production
		WORKDIR /airplane{{.Workdir}}

		# npm >= 7 will automatically install peer dependencies, even if they're satisfied by the root. This is
		# problematic because we need the @airplane/workflow-runtime package to register the workflow runtime in the
		# runtime map that is utilized by the user's code, and so we explicitly request legacy behavior in this
		# instance, which does not install peer dependencies by default.
		RUN mkdir -p /airplane/.airplane && \
			cd /airplane/.airplane && \
			{{.InlineShimPackageJSON}} > package.json && \
			npm install --legacy-peer-deps

		RUN {{.InlineTaskShim}} > /airplane/.airplane/universal-shim.js && \
			node /airplane/.airplane/esbuild.js \
				'["/airplane/.airplane/universal-shim.js"]' \
				node{{.NodeVersion}} \
				'{{.External}}' \
				/airplane/.airplane/dist/universal-shim.js

		RUN node /airplane/.airplane/esbuild.js \
			'{{.FilesToBuild}}' \
			node{{.NodeVersion}} \
			'{{.External}}' \
			"" \
			/airplane/.airplane \
			/airplane

		# Discover inline tasks now that dependencies are installed and entrypoint files
		# are built.
		# FilesToDiscover is the location of the output of the transpiled js files
		# that should be discovered.
		{{if .FilesToDiscover}}
		RUN node /airplane/.airplane-build-tools/inlineParser.cjs {{.FilesToDiscover}}
		{{end}}
	`), cfg)
}

func isWorkflowRuntime(options buildtypes.KindOptions) bool {
	runtime, ok := options["runtime"]
	if !ok {
		return false
	}

	// Depending on how the options were serialized, the runtime can be
	// either a string or TaskRuntime; handle both.
	switch v := runtime.(type) {
	case string:
		return v == string(buildtypes.TaskRuntimeWorkflow)
	case buildtypes.TaskRuntime:
		return v == buildtypes.TaskRuntimeWorkflow
	default:
		return false
	}
}

type makeInstallCommandReq struct {
	PkgInstallCommand string
	RootPackageJSON   string
	IsYarn            bool
	HasPackageLock    bool
}

func makeInstallCommand(req makeInstallCommandReq) string {
	installCommand := "npm install"
	if req.PkgInstallCommand != "" {
		installCommand = req.PkgInstallCommand
	} else if req.IsYarn {
		if yarnBerry, _ := isYarnBerry(req.RootPackageJSON); yarnBerry {
			// Yarn Berry has different flags.
			// --frozen-lockfile is equivalent to --immutable.
			// --non-interactive does not exist.
			installCommand = "yarn install --immutable"
		} else {
			// Because the install command is running in the context of a docker build, the yarn cache
			// isn't used after the packages are installed, and so we clean the cache to keep the
			// image lean. This doesn't apply to Yarn v2 (specifically Plug'n'Play), which uses the
			// cache directory for storing packages.
			installCommand = "yarn install --non-interactive --frozen-lockfile && yarn cache clean"
		}
	} else if req.HasPackageLock {
		// Use npm ci if possible, since it's faster and behaves better:
		// https://docs.npmjs.com/cli/v8/commands/npm-ci
		installCommand = "npm ci"
	}
	// Remove large binaries for platforms that we aren't using
	// TODO: Remove the ARM binaries if on AMD64 and vice versa to save a bit of extra space
	installCommand += " && rm -Rf /airplane/node_modules/@swc/core-linux-x64-musl /airplane/node_modules/@temporalio/core-bridge/releases/*windows* /airplane/node_modules/@temporalio/core-bridge/releases/*darwin*"

	return strings.ReplaceAll(installCommand, "\n", "\\n")
}

func makeArgsCommand(buildArgs []string) string {
	for i, a := range buildArgs {
		buildArgs[i] = fmt.Sprintf("ARG %s", a)
	}
	return strings.Join(buildArgs, "\n")
}
