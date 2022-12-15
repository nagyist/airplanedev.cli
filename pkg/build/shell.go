package build

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/airplanedev/lib/pkg/utils/fsx"
	"github.com/pkg/errors"
)

func shell(root string, options KindOptions) (string, error) {
	// Assert that the entrypoint file exists:
	entrypoint, _ := options["entrypoint"].(string)
	if entrypoint == "" {
		return "", errors.New("entrypoint is unexpectedly missing")
	}
	if err := fsx.AssertExistsAll(filepath.Join(root, entrypoint)); err != nil {
		return "", err
	}

	dockerfileTemplate, workDir, err := getBaseDockerfileTemplate(root)
	if err != nil {
		return "", err
	}

	// Extend template with our own logic - set up a WORKDIR and shim.
	dockerfileTemplate = dockerfileTemplate + heredoc.Doc(`
		WORKDIR {{.Workdir}}
		RUN mkdir -p .airplane && {{.InlineShim}} > .airplane/shim.sh

		COPY . .
		RUN chmod +x {{.Entrypoint}}

		ENTRYPOINT ["bash", ".airplane/shim.sh", "./{{.Entrypoint}}"]
	`)
	return applyTemplate(dockerfileTemplate, struct {
		InlineShim string
		Entrypoint string
		Workdir    string
	}{
		InlineShim: inlineString(ShellShim()),
		Entrypoint: backslashEscape(entrypoint, `"`),
		Workdir:    workDir,
	})
}

func shellBundle(root string) (string, error) {
	dockerfileTemplate, workDir, err := getBaseDockerfileTemplate(root)
	if err != nil {
		return "", err
	}

	// Extend template with our own logic - set up a WORKDIR and shim.
	dockerfileTemplate = dockerfileTemplate + heredoc.Doc(`
		WORKDIR {{.Workdir}}
		RUN mkdir -p .airplane && {{.InlineShim}} > .airplane/shim.sh

		COPY --chmod=700 . .
		# Set an empty entrypoint to override any entrypoints that may be set in the base image.
		ENTRYPOINT []
	`)
	return applyTemplate(dockerfileTemplate, struct {
		InlineShim string
		Entrypoint string
		Workdir    string
	}{
		InlineShim: inlineString(ShellShim()),
		Workdir:    workDir,
	})
}

//go:embed shell-shim.sh
var shellShim string

func ShellShim() string {
	return shellShim
}

func DockerfilePaths() []string {
	return []string{
		"Dockerfile.airplane",
		"Dockerfile",
	}
}

// FindDockerfile looks for variants of supported Dockerfile locations and returns non-empty path
// to the file, if found.
func FindDockerfile(root string) string {
	for _, filePath := range DockerfilePaths() {
		dockerfilePath := filepath.Join(root, filePath)
		if fsx.Exists(dockerfilePath) {
			return dockerfilePath
		}
	}
	return ""
}

func getBaseDockerfileTemplate(root string) (dockerfileTemplate, workDir string, err error) {
	if dockerfilePath := FindDockerfile(root); dockerfilePath != "" {
		contents, err := os.ReadFile(dockerfilePath)
		if err != nil {
			return "", "", errors.Wrap(err, "opening dockerfile")
		}
		dockerfileTemplate = string(contents)

		if !strings.HasSuffix(dockerfileTemplate, "\n") {
			dockerfileTemplate = dockerfileTemplate + "\n"
		}

		workDir = "."
	} else {
		dockerfileTemplate = heredoc.Doc(`
			FROM ubuntu:22.10
			# Install some common libraries
			RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
				&& apt-get -y install --no-install-recommends \
					apt-utils \
					openssh-client \
					gnupg2 \
					iproute2 \
					procps \
					lsof \
					htop \
					net-tools \
					curl \
					wget \
					ca-certificates \
					unzip \
					zip \
					nano \
					vim-tiny \
					less \
					jq \
					lsb-release \
					apt-transport-https \
					dialog \
					zlib1g \
					locales \
					strace \
				&& apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/*
		`)
		workDir = "/airplane"
	}

	return dockerfileTemplate, workDir, nil
}
