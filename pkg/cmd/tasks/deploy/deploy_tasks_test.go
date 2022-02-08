package deploy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/build"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	libapi "github.com/airplanedev/lib/pkg/api"
	libBuild "github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/archive"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/go-git/go-billy/v5/memfs"
	fixtures "github.com/go-git/go-git-fixtures/v4"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployTasks(t *testing.T) {
	dotgit := fixtures.Basic().One().DotGit()
	worktree := memfs.New()
	st := filesystem.NewStorage(dotgit, cache.NewObjectLRUDefault())
	mockRepo, err := git.Open(st, worktree)
	if err != nil {
		panic(err)
	}

	fixturesPath, _ := filepath.Abs("./fixtures")
	testCases := []struct {
		desc          string
		taskConfigs   []discover.TaskConfig
		existingTasks map[string]libapi.Task
		changedFiles  []string
		local         bool
		envSlug       string
		gitRepo       *git.Repository
		updatedTasks  map[string]libapi.Task
		deploys       []api.CreateDeploymentRequest
	}{
		{
			desc: "no tasks",
		},
		{
			desc: "deploys a task",
			taskConfigs: []discover.TaskConfig{
				{
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
					},
					Task: libapi.Task{
						ID:   "my_task",
						Slug: "my_task",
						Name: "My Task",
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {Slug: "my_task", Name: "My Task"}},
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "my_task",
							Kind:   "node",
							BuildConfig: libBuild.KindOptions{
								"entrypoint":  "",
								"nodeVersion": "",
								"shim":        "true",
							},
							UploadID: "uploadID",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint":  "",
									"nodeVersion": "",
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "deploys a task to an environment",
			taskConfigs: []discover.TaskConfig{
				{
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
					},
					Task: libapi.Task{
						ID:   "my_task",
						Slug: "my_task",
						Name: "My Task",
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {Slug: "my_task", Name: "My Task"}},
			envSlug:       "myEnv",
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "my_task",
							Kind:   "node",
							BuildConfig: libBuild.KindOptions{
								"entrypoint":  "",
								"nodeVersion": "",
								"shim":        "true",
							},
							UploadID: "uploadID",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint":  "",
									"nodeVersion": "",
								},
								EnvSlug: "myEnv",
							},
						},
					},
				},
			},
		},
		{
			desc: "deploys a task that doesn't need to be built",
			taskConfigs: []discover.TaskConfig{
				{
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name:  "My Task",
						Slug:  "my_task",
						Image: &definitions.ImageDefinition_0_3{Image: "myImage"},
					},
					Task: libapi.Task{
						ID:   "my_task",
						Slug: "my_task",
						Name: "My Task",
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {Slug: "my_task", Name: "My Task"}},
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "my_task",
							Kind:   "image",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Kind:       "image",
								Command:    []string{},
								Image:      pointers.String("myImage"),
							},
						},
					},
				},
			},
		},
		{
			desc: "deploys a task - local",
			taskConfigs: []discover.TaskConfig{
				{
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
					},
					Task: libapi.Task{
						ID:   "my_task",
						Slug: "my_task",
						Name: "My Task",
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {Slug: "my_task", Name: "My Task"}},
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "my_task",
							Kind:   "node",
							BuildConfig: libBuild.KindOptions{
								"entrypoint":  "",
								"nodeVersion": "",
								"shim":        "true",
							},
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint":  "",
									"nodeVersion": "",
								},
								Image: pointers.String("imageURL"),
							},
						},
					},
				},
			},
			local: true,
		},
		{
			desc: "tasks filtered out by changed files",
			taskConfigs: []discover.TaskConfig{
				{
					TaskRoot: "some/other/root.js",
				},
			},
			changedFiles: []string{"some/random/path.js"},
		},
		{
			desc: "deploys a task with git metadata",
			taskConfigs: []discover.TaskConfig{
				{
					TaskRoot:       fixturesPath,
					TaskEntrypoint: "/json/short.json",
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
					},
					Task: libapi.Task{
						ID:   "my_task",
						Slug: "my_task",
						Name: "My Task",
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {Slug: "my_task", Name: "My Task"}},
			gitRepo:       mockRepo,
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "my_task",
							Kind:   "node",
							BuildConfig: libBuild.KindOptions{
								"entrypoint":  "",
								"nodeVersion": "",
								"shim":        "true",
							},
							UploadID: "uploadID",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint":  "",
									"nodeVersion": "",
								},
							},
							GitFilePath: "json/short.json",
						},
					},
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
		// TODO uncomment when sql deploys work.
		// {
		// 	desc: "deploys and updates an SQL task",
		// 	taskConfigs: []discover.TaskConfig{
		// 		{
		// 			TaskRoot: fixturesPath,
		// 			Def: &definitions.Definition_0_3{
		// 				Name: "My Task",
		// 				Slug: "my_task",
		// 				SQL: &definitions.SQLDefinition_0_3{
		// 					Entrypoint: "./fixtures/test.sql",
		// 				},
		// 			},
		// 			Task: libapi.Task{
		// 				Slug: "my_task",
		// 				Name: "My Task",
		// 			},
		// 		},
		// 	},
		// 	existingTasks: map[string]libapi.Task{"my_task": {Slug: "my_task", Name: "My Task"}},
		// 	updatedTasks: map[string]libapi.Task{
		// 		"my_task": {
		// 			Name:       "My Task",
		// 			Slug:       "my_task",
		// 			Parameters: libapi.Parameters{},
		// 			Kind:       "sql",
		// 		},
		// 	},
		// },
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)
			client := &api.MockClient{
				Tasks: tC.existingTasks,
			}
			cfg := config{
				changedFiles: tC.changedFiles,
				client:       client,
				local:        tC.local,
				envSlug:      tC.envSlug,
			}
			d := NewDeployer(cfg, &logger.MockLogger{}, DeployerOpts{
				BuildCreator: &build.MockBuildCreator{},
				Archiver:     &archive.MockArchiver{},
				RepoGetter:   &MockGitRepoGetter{Repo: tC.gitRepo},
			})
			err := d.DeployTasks(context.Background(), tC.taskConfigs)
			require.NoError(err)

			if tC.updatedTasks != nil {
				assert.Equal(tC.updatedTasks, client.Tasks)
			} else {
				assert.Equal(tC.existingTasks, client.Tasks)
			}
			assert.Equal(tC.deploys, client.Deploys)
		})
	}
}

func TestParseRemote(t *testing.T) {
	testCases := []struct {
		desc      string
		remote    string
		ownerName string
		repoName  string
		vendor    api.GitVendor
	}{
		{
			desc:      "git http",
			remote:    "https://github.com/airplanedev/airport",
			ownerName: "airplanedev",
			repoName:  "airport",
			vendor:    api.GitVendorGitHub,
		},
		{
			desc:      "git ssh",
			remote:    "git@github.com:airplanedev/airport.git",
			ownerName: "airplanedev",
			repoName:  "airport",
			vendor:    api.GitVendorGitHub,
		},
		{
			desc:   "unknown - no error returned",
			remote: "some remote",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()
			assert := assert.New(t)
			require := require.New(t)

			owner, name, vendor, err := parseRemote(tC.remote)
			require.NoError(err)

			assert.Equal(tC.ownerName, owner)
			assert.Equal(tC.repoName, name)
			assert.Equal(tC.vendor, vendor)
		})
	}
}
