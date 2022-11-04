package build

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/airplanedev/lib/pkg/build/ignore"
	"github.com/airplanedev/lib/pkg/utils/bufiox"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	dockerJSONMessage "github.com/docker/docker/pkg/jsonmessage"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
)

// LocalConfig configures a (local) builder.
type BundleLocalConfig struct {
	// Root is the root directory.
	//
	// It must be an absolute path to the project directory.
	Root string

	// Build context describes the type of build. It must be valid.
	BuildContext BuildContext

	// Options are the build arguments to use.
	//
	// When nil, it uses an empty map of options.
	Options KindOptions

	// FilesToBuild are the target files to be built (if applicable).
	FilesToBuild []string

	// FilesToDiscover are the target files to discover (if applicable).
	FilesToDiscover []string

	// Auth represents the registry auth to use.
	//
	// If nil, Push will produce an error.
	Auth *RegistryAuth
}

type BundleDockerfileConfig struct {
	BuildContext    BuildContext
	Root            string
	Options         KindOptions
	BuildArgKeys    []string
	FilesToBuild    []string
	FilesToDiscover []string
}

// Builder implements an image builder.
type BundleBuilder struct {
	root            string
	buildContext    BuildContext
	options         KindOptions
	filesToBuild    []string
	filesToDiscover []string
	auth            *RegistryAuth
	client          *client.Client
}

// New returns a new local builder with c.
func NewBundleBuilder(c BundleLocalConfig) (*BundleBuilder, *client.Client, error) {
	if !filepath.IsAbs(c.Root) {
		return nil, nil, fmt.Errorf("build: expected an absolute root path, got %q", c.Root)
	}

	if !c.BuildContext.Valid() {
		return nil, nil, fmt.Errorf("build: unexpected build context: (%s:%s)", c.BuildContext.Type, c.BuildContext.Version)
	}

	if c.Options == nil {
		c.Options = KindOptions{}
	}

	client, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, nil, err
	}

	return &BundleBuilder{
		root:            c.Root,
		buildContext:    c.BuildContext,
		options:         c.Options,
		filesToBuild:    c.FilesToBuild,
		filesToDiscover: c.FilesToDiscover,
		auth:            c.Auth,
		client:          client,
	}, client, nil
}

func (b *BundleBuilder) Close() error {
	return b.client.Close()
}

// Build runs the docker build.
//
// Depending on the configured `Config.Builder` the method verifies that
// the directory can be built and all necessary files exist.
//
// The method creates a Dockerfile depending on the configured builder
// and adds it to the tree, it passes the tree as the build context
// and initializes the build.
func (b *BundleBuilder) Build(ctx context.Context, bundleBuildID, version string) (*Response, error) {
	name := "bundle-build-" + SanitizeID(bundleBuildID)
	uri := name + ":" + version
	if b.auth != nil {
		uri = b.auth.Repo + "/" + uri
	}

	patterns, err := ignore.DockerignorePatterns(b.root)
	if err != nil {
		return nil, err
	}
	tree, err := NewTree(TreeOptions{
		ExcludePatterns: patterns,
	})
	if err != nil {
		return nil, errors.Wrap(err, "new tree")
	}
	defer tree.Close()

	dockerfile, err := BuildBundleDockerfile(BundleDockerfileConfig{
		BuildContext:    b.buildContext,
		Root:            b.root,
		Options:         b.options,
		FilesToBuild:    b.filesToBuild,
		FilesToDiscover: b.filesToDiscover,
	})
	if err != nil {
		return nil, errors.Wrap(err, "creating dockerfile")
	}

	dockerfilePath := ".airplane/Dockerfile"
	if err := tree.MkdirAll(filepath.Dir(dockerfilePath)); err != nil {
		return nil, err
	}
	if err := tree.Write(dockerfilePath, strings.NewReader(dockerfile)); err != nil {
		return nil, err
	}

	if err := tree.Copy(b.root); err != nil {
		return nil, err
	}

	bc, err := tree.Archive()
	if err != nil {
		return nil, err
	}
	defer bc.Close()

	opts := types.ImageBuildOptions{
		Dockerfile:  dockerfilePath,
		Tags:        []string{uri},
		Platform:    "linux/amd64",
		AuthConfigs: b.authconfigs(),
	}

	resp, err := b.client.ImageBuild(ctx, bc, opts)
	if err != nil {
		return nil, errors.Wrap(err, "image build")
	}
	defer resp.Body.Close()

	scanner := bufiox.NewScanner(resp.Body)
	for scanner.Scan() {
		var event *dockerJSONMessage.JSONMessage
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, errors.Wrap(err, "unmarshalling docker build event")
		}

		if err := event.Display(os.Stderr, isatty.IsTerminal(os.Stderr.Fd())); err != nil {
			return nil, errors.Wrap(err, "docker build")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "scanning")
	}

	return &Response{
		ImageURL: uri,
	}, nil
}

// Push pushes the given image.
func (b *BundleBuilder) Push(ctx context.Context, uri string) error {
	if b.auth == nil {
		return errors.New("push requires registry auth")
	}

	authjson, err := json.Marshal(b.registryAuth())
	if err != nil {
		return err
	}

	resp, err := b.client.ImagePush(ctx, uri, types.ImagePushOptions{
		RegistryAuth: base64.URLEncoding.EncodeToString(authjson),
	})
	if err != nil {
		return err
	}
	defer resp.Close()

	scanner := bufiox.NewScanner(resp)
	for scanner.Scan() {
		var event *dockerJSONMessage.JSONMessage
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return errors.Wrap(err, "unmarshalling docker build event")
		}

		if err := event.Display(os.Stderr, isatty.IsTerminal(os.Stderr.Fd())); err != nil {
			return errors.Wrap(err, "docker push")
		}
	}
	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "scanning")
	}

	return nil
}

// RegistryAuth returns the registry auth.
func (b *BundleBuilder) registryAuth() types.AuthConfig {
	return types.AuthConfig{
		Username: "oauth2accesstoken",
		Password: b.auth.Token,
	}
}

// Authconfigs returns the authconfigs to use.
func (b *BundleBuilder) authconfigs() map[string]types.AuthConfig {
	if b.auth == nil {
		return map[string]types.AuthConfig{}
	}

	return map[string]types.AuthConfig{
		b.auth.host(): b.registryAuth(),
	}
}

func BuildBundleDockerfile(c BundleDockerfileConfig) (string, error) {
	switch c.BuildContext.Type {
	case NodeBuildType:
		// TODO: pipe in build args
		return nodeBundle(c.Root, c.BuildContext, c.Options, c.BuildArgKeys, c.FilesToBuild, c.FilesToDiscover)
	case ViewBuildType:
		return viewBundle(c.Root, c.BuildContext, c.Options, c.FilesToBuild, c.FilesToDiscover)
	default:
		return "", errors.Errorf("build: unknown build type %v", c.BuildContext.Type)
	}
}
