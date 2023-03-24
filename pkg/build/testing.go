package build

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airplanedev/dlog"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/lib/pkg/examples"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
)

type BundleTestRun struct {
	RelEntrypoint string
	ExportName    string
	SearchString  string
}

type Test struct {
	// Root is the task root to perform a build inside of.
	Root        string
	Kind        buildtypes.TaskKind
	Options     buildtypes.KindOptions
	ParamValues buildtypes.Values
	BuildArgs   map[string]string
	SkipRun     bool
	// SearchString is a string to look for in the example's output
	// to validate that the task completed successfully. If not set,
	// defaults to a random value which is passed into the example
	// via the `id` parameter.
	SearchString  string
	ExpectedError bool

	// Bundle-specific test config
	Bundle          bool
	BuildContext    buildtypes.BuildContext
	FilesToBuild    []string
	FilesToDiscover []string
	BundleRuns      []BundleTestRun
	// Target is the docker target to build.
	Target string

	ExpectedStatusCode int

	// TODO: pipe to actual build/container run etc. set increased timeout if times out
}

// RunTests performs a series of builder tests and looks for a given SearchString
// in the task's output to validate that the task built + ran correctly.
func RunTests(tt *testing.T, ctx context.Context, tests []Test) {
	for _, test := range tests {
		test := test // loop local reference
		tt.Run(filepath.Base(test.Root), func(t *testing.T) {
			// These tests can run in parallel, but it may exhaust all memory
			// allocated to the Docker daemon on your computer. For that reason,
			// we don't currently run them in parallel. We could gate parallel
			// execution to CI via `os.Getenv("CI") == "true"`, but that may
			// lead to scenarios where tests break in CI but not locally. If
			// test performance in CI becomes an issue, we should look into caching
			// Docker builds in CI since (locally) that appears to have a significant
			// impact on e2e times for this test suite.
			//
			// t.Parallel()

			require := require.New(t)
			var b ImageBuilder
			var client *client.Client
			var err error

			if test.Bundle {
				b, client, err = NewBundleBuilder(BundleLocalConfig{
					Root:            examples.Path(t, test.Root),
					BuildContext:    test.BuildContext,
					Options:         test.Options,
					FilesToBuild:    test.FilesToBuild,
					FilesToDiscover: test.FilesToDiscover,
					Target:          test.Target,
				})
			} else {
				b, client, err = New(LocalConfig{
					Root:      examples.Path(t, test.Root),
					Builder:   string(test.Kind),
					Options:   test.Options,
					BuildArgs: test.BuildArgs,
				})
			}

			require.NoError(err)
			t.Cleanup(func() {
				require.NoError(b.Close())
			})

			// Perform the docker build:
			resp, err := b.Build(ctx, "builder-tests", ksuid.New().String())
			if test.ExpectedError {
				require.Error(err)
				return
			}

			require.NoError(err)
			defer func() {
				_, err := client.ImageRemove(ctx, resp.ImageURL, types.ImageRemoveOptions{})
				require.NoError(err)
			}()

			if test.ParamValues == nil {
				test.ParamValues = buildtypes.Values{}
			}
			if test.SearchString == "" {
				test.SearchString = ksuid.New().String()
				test.ParamValues["id"] = test.SearchString
			}

			if !test.SkipRun {
				if test.Bundle {
					switch test.BuildContext.Type {
					case buildtypes.NodeBuildType:
						for _, testRun := range test.BundleRuns {
							out := runTask(t, ctx, client, runTaskConfig{
								Image:              resp.ImageURL,
								ParamValues:        test.ParamValues,
								Entrypoint:         []string{"node", "/airplane/.airplane/dist/universal-shim.js", path.Join("/airplane/.airplane/", testRun.RelEntrypoint), testRun.ExportName},
								Kind:               test.Kind,
								ExpectedStatusCode: test.ExpectedStatusCode,
							})
							ss := testRun.SearchString
							if ss == "" {
								ss = test.SearchString
							}
							require.True(strings.Contains(string(out), ss), "unable to find %q in output:\n%s", ss, string(out))
						}
					case buildtypes.PythonBuildType:
						for _, testRun := range test.BundleRuns {
							out := runTask(t, ctx, client, runTaskConfig{
								Image:              resp.ImageURL,
								ParamValues:        test.ParamValues,
								Entrypoint:         []string{"python", "/airplane/.airplane/shim.py", path.Join("/airplane/", testRun.RelEntrypoint), testRun.ExportName},
								Kind:               test.Kind,
								ExpectedStatusCode: test.ExpectedStatusCode,
							})
							ss := testRun.SearchString
							if ss == "" {
								ss = test.SearchString
							}
							require.True(strings.Contains(string(out), ss), "unable to find %q in output:\n%s", ss, string(out))
						}
					case buildtypes.ShellBuildType:
						for _, testRun := range test.BundleRuns {
							out := runTask(t, ctx, client, runTaskConfig{
								Image:              resp.ImageURL,
								ParamValues:        test.ParamValues,
								Cmd:                []string{"bash", ".airplane/shim.sh", "./" + testRun.RelEntrypoint},
								Kind:               test.Kind,
								ExpectedStatusCode: test.ExpectedStatusCode,
							})
							require.True(strings.Contains(string(out), testRun.SearchString), "unable to find %q in output:\n%s", test.SearchString, string(out))
						}
					default:
						require.Fail("bundle tests are not available for build context type")
					}
				} else {
					// Run the produced docker image:
					out := runTask(t, ctx, client, runTaskConfig{
						Image:              resp.ImageURL,
						ParamValues:        test.ParamValues,
						Kind:               test.Kind,
						ExpectedStatusCode: test.ExpectedStatusCode,
					})
					require.True(strings.Contains(string(out), test.SearchString), "unable to find %q in output:\n%s", test.SearchString, string(out))
				}
			}
		})
	}
}

type runTaskConfig struct {
	Image              string
	ParamValues        buildtypes.Values
	Entrypoint         strslice.StrSlice
	Cmd                strslice.StrSlice
	Kind               buildtypes.TaskKind
	ExpectedStatusCode int
}

func runTask(t *testing.T, ctx context.Context, dclient *client.Client, c runTaskConfig) []byte {
	require := require.New(t)

	cmd := c.Cmd
	if c.Kind == buildtypes.TaskKindShell {
		var params []string
		for k, v := range c.ParamValues {
			params = append(params, fmt.Sprintf("%s=%s", k, v))
		}
		if len(c.Entrypoint) == 0 {
			cmd = append(cmd, params...)
		} else {
			for k, v := range c.ParamValues {
				c.Entrypoint = append(c.Entrypoint, fmt.Sprintf("%s=%s", k, v))
			}
		}
	} else {
		pv, err := json.Marshal(c.ParamValues)
		require.NoError(err)
		if len(c.Entrypoint) == 0 {
			cmd = append(cmd, string(pv))
		} else {
			c.Entrypoint = append(c.Entrypoint, string(pv))
		}

	}

	resp, err := dclient.ContainerCreate(ctx, &container.Config{
		Image:      c.Image,
		Tty:        false,
		Cmd:        cmd,
		Entrypoint: c.Entrypoint,
	}, nil, nil, nil, "")
	require.NoError(err)
	containerID := resp.ID
	defer func() {
		// Cleanup this container when we complete these tests:
		require.NoError(dclient.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
			Force: true,
		}))
	}()

	require.NoError(dclient.ContainerStart(ctx, containerID, types.ContainerStartOptions{}))

	resultC, errC := dclient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	logr, err := dclient.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Follow:     true,
		Tail:       "all",
	})
	require.NoError(err)
	defer logr.Close()

	logs, err := io.ReadAll(dlog.NewReader(logr, dlog.Options{
		AppendNewline: true,
	}))
	//nolint:forbidigo
	fmt.Println("Output logs")
	//nolint:forbidigo
	fmt.Println(string(logs))
	require.NoError(err)

	select {
	case result := <-resultC:
		require.Nil(result.Error)
		require.Equal(int64(c.ExpectedStatusCode), result.StatusCode, "container exited with non-zero status code: %v", result.StatusCode)
	case err := <-errC:
		require.NoError(err)
	}

	return logs
}
