package deploy

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/airplanedev/cli/cmd/airplane/tasks/deploy"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/cli"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/archive"
	"github.com/airplanedev/lib/pkg/deploy/bundlediscover"
	"github.com/go-git/go-billy/v5/memfs"
	fixtures "github.com/go-git/go-git-fixtures/v4"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploy(t *testing.T) {
	dotgit := fixtures.Basic().One().DotGit()
	worktree := memfs.New()
	st := filesystem.NewStorage(dotgit, cache.NewObjectLRUDefault())
	mockRepo, err := git.Open(st, worktree)
	if err != nil {
		panic(err)
	}
	now := time.Now()

	testCases := []struct {
		desc                  string
		bundles               []bundlediscover.Bundle
		changedFiles          []string
		envVars               map[string]string
		envSlug               string
		gitRepo               *git.Repository
		getDeploymentResponse *api.Deployment
		expectedError         error
		deploys               []api.CreateDeploymentRequest
	}{
		{
			desc: "nothing to deploy",
		},
		{
			desc: "deploys one bundle",
			bundles: []bundlediscover.Bundle{
				{
					RootPath:     "myRoot",
					TargetPaths:  []string{"myPath"},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode16,
				},
			},
			deploys: []api.CreateDeploymentRequest{
				{
					Bundles: []api.DeployBundle{
						{
							UploadID:    "uploadID",
							TargetFiles: []string{"myPath"},
							BuildContext: api.BuildContext{
								Type:    build.NodeBuildType,
								Version: build.BuildTypeVersionNode16,
							},
						},
					},
				},
			},
		},
		{
			desc: "deploys multiple bundles",
			bundles: []bundlediscover.Bundle{
				{
					RootPath:     "myRoot",
					TargetPaths:  []string{"myPath"},
					BuildType:    build.NodeBuildType,
					BuildVersion: build.BuildTypeVersionNode16,
				},
				{
					RootPath:    "myRoot2",
					TargetPaths: []string{"myPath", "myPath2"},
				},
			},
			deploys: []api.CreateDeploymentRequest{
				{
					Bundles: []api.DeployBundle{
						{
							UploadID:    "uploadID",
							TargetFiles: []string{"myPath"},
							BuildContext: api.BuildContext{
								Type:    build.NodeBuildType,
								Version: build.BuildTypeVersionNode16,
							},
						},
						{
							UploadID:    "uploadID",
							TargetFiles: []string{"myPath", "myPath2"},
						},
					},
				},
			},
		},
		{
			desc:                  "deployment fails",
			bundles:               []bundlediscover.Bundle{{RootPath: "myRoot"}},
			getDeploymentResponse: &api.Deployment{FailedAt: &now},
			expectedError:         errors.New("Deploy failed"),
			deploys: []api.CreateDeploymentRequest{
				{Bundles: []api.DeployBundle{{UploadID: "uploadID"}}},
			},
		},
		{
			desc:                  "deployment cancelled",
			bundles:               []bundlediscover.Bundle{{RootPath: "myRoot"}},
			getDeploymentResponse: &api.Deployment{CancelledAt: &now},
			expectedError:         errors.New("Deploy cancelled"),
			deploys: []api.CreateDeploymentRequest{
				{Bundles: []api.DeployBundle{{UploadID: "uploadID"}}},
			},
		},
		{
			desc:    "deploys to environment",
			envSlug: "myEnv",
			bundles: []bundlediscover.Bundle{{RootPath: "myRoot"}},
			deploys: []api.CreateDeploymentRequest{
				{
					Bundles: []api.DeployBundle{{UploadID: "uploadID"}},
					EnvSlug: "myEnv",
				},
			},
		},
		{
			desc:         "bundles filtered out by changed files",
			bundles:      []bundlediscover.Bundle{{RootPath: "myRoot"}},
			changedFiles: []string{"some/random/path.js"},
		},
		{
			desc:         "bundles filtered in by changed files",
			bundles:      []bundlediscover.Bundle{{RootPath: "myRoot"}},
			changedFiles: []string{"myRoot/some/sub/file"},
			deploys: []api.CreateDeploymentRequest{
				{
					Bundles: []api.DeployBundle{{UploadID: "uploadID"}},
				},
			},
		},
		{
			desc:    "deploys a bundle with git metadata",
			bundles: []bundlediscover.Bundle{{RootPath: "myRoot"}},
			gitRepo: mockRepo,
			deploys: []api.CreateDeploymentRequest{
				{
					Bundles: []api.DeployBundle{{UploadID: "uploadID"}},
					GitMetadata: api.GitMetadata{
						CommitHash:          "6ecf0ef2c2dffb796033e5a02219af86ec6584e5",
						Ref:                 "master",
						IsDirty:             true,
						CommitMessage:       "vendor stuff\n",
						RepositoryOwnerName: "git-fixtures",
						RepositoryName:      "basic",
						Vendor:              "GitHub",
					},
				},
			},
		},
		{
			desc:    "deploys a bundle with owner and repo from env var",
			bundles: []bundlediscover.Bundle{{RootPath: "myRoot"}},
			gitRepo: mockRepo,
			envVars: map[string]string{
				"AP_GIT_REPO": "airplanedev/airport",
			},
			deploys: []api.CreateDeploymentRequest{
				{
					Bundles: []api.DeployBundle{{UploadID: "uploadID"}},
					GitMetadata: api.GitMetadata{
						CommitHash:          "6ecf0ef2c2dffb796033e5a02219af86ec6584e5",
						Ref:                 "master",
						IsDirty:             true,
						CommitMessage:       "vendor stuff\n",
						RepositoryOwnerName: "airplanedev",
						RepositoryName:      "airport",
						Vendor:              "GitHub",
					},
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)
			client := &api.MockClient{
				GetDeploymentResponse: tC.getDeploymentResponse,
				Configs: []api.Config{
					{
						Name: "API_KEY",
					},
				},
			}
			for k, v := range tC.envVars {
				os.Setenv(k, v)
			}
			cfg := config{
				changedFiles: tC.changedFiles,
				client:       client,
				envSlug:      tC.envSlug,
				root: &cli.Config{
					Client: &api.Client{
						Host: api.Host,
					},
				},
			}
			d := NewDeployer(cfg, &logger.MockLogger{}, DeployerOpts{
				Archiver:   &archive.MockArchiver{},
				RepoGetter: &deploy.MockGitRepoGetter{Repo: tC.gitRepo},
			})

			err := d.Deploy(context.Background(), tC.bundles)
			if tC.expectedError != nil {
				assert.Error(err)
				return
			} else {
				require.NoError(err)
			}

			assert.Equal(tC.deploys, client.Deploys)
		})
	}
}