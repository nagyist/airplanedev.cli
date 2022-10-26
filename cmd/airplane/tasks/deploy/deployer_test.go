package deploy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/build"
	"github.com/airplanedev/cli/pkg/cli"
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

	fixturesPath, _ := filepath.Abs("./fixtures")
	testCases := []struct {
		desc                  string
		taskConfigs           []discover.TaskConfig
		viewConfigs           []discover.ViewConfig
		absoluteEntrypoints   []string
		existingTasks         map[string]libapi.Task
		existingViews         map[string]libapi.View
		changedFiles          []string
		envVars               map[string]string
		local                 bool
		envSlug               string
		gitRepo               *git.Repository
		getDeploymentResponse *api.Deployment
		expectedError         error
		deploys               []api.CreateDeploymentRequest
		resources             []libapi.Resource
	}{
		{
			desc: "no tasks",
		},
		{
			desc: "deploys a task",
			taskConfigs: []discover.TaskConfig{
				{
					TaskID:   "tsk123",
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {ID: "tsk123", Slug: "my_task", Name: "My Task", InterpolationMode: "jst"}},
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "tsk123",
							Kind:   "node",
							BuildConfig: libBuild.BuildConfig{
								"entrypoint": "",
								"runtime":    libBuild.TaskRuntimeStandard,
								"shim":       "true",
							},
							UploadID: "uploadID",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Resources:  map[string]string{},
								Configs:    &[]libapi.ConfigAttachment{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint": "",
								},
								Runtime: "",
								ExecuteRules: libapi.UpdateExecuteRulesRequest{
									DisallowSelfApprove: pointers.Bool(false),
									RequireRequests:     pointers.Bool(false),
								},
								InterpolationMode: pointers.String("jst"),
								Timeout:           3600,
							},
						},
					},
				},
			},
		},
		{
			desc: "deploys a workflow task",
			taskConfigs: []discover.TaskConfig{
				{
					TaskID:   "tsk123",
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name:    "My Task",
						Slug:    "my_workflow_task",
						Node:    &definitions.NodeDefinition_0_3{},
						Runtime: libBuild.TaskRuntimeWorkflow,
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_workflow_task": {ID: "tsk123", Slug: "my_workflow_task", Name: "My Task", InterpolationMode: "jst"}},
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "tsk123",
							Kind:   "node",
							BuildConfig: libBuild.BuildConfig{
								"entrypoint": "",
								"runtime":    libBuild.TaskRuntimeWorkflow,
								"shim":       "true",
							},
							UploadID: "uploadID",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_workflow_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Resources:  map[string]string{},
								Configs:    &[]libapi.ConfigAttachment{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint": "",
								},
								Runtime: libBuild.TaskRuntimeWorkflow,
								ExecuteRules: libapi.UpdateExecuteRulesRequest{
									DisallowSelfApprove: pointers.Bool(false),
									RequireRequests:     pointers.Bool(false),
								},
								InterpolationMode: pointers.String("jst"),
								Timeout:           3600,
							},
						},
					},
				},
			},
		},
		{
			desc: "deploys a task - deployment fails",
			taskConfigs: []discover.TaskConfig{
				{
					TaskID:   "tsk123",
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
					},
				},
			},
			existingTasks:         map[string]libapi.Task{"my_task": {ID: "tsk123", Slug: "my_task", Name: "My Task", InterpolationMode: "jst"}},
			getDeploymentResponse: &api.Deployment{FailedAt: &now},
			expectedError:         errors.New("Deploy failed"),
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "tsk123",
							Kind:   "node",
							BuildConfig: libBuild.BuildConfig{
								"entrypoint": "",
								"shim":       "true",
							},
							UploadID: "uploadID",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Resources:  map[string]string{},
								Configs:    &[]libapi.ConfigAttachment{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint": "",
								},
								ExecuteRules: libapi.UpdateExecuteRulesRequest{
									DisallowSelfApprove: pointers.Bool(false),
									RequireRequests:     pointers.Bool(false),
								},
								InterpolationMode: pointers.String("jst"),
								Timeout:           3600,
							},
						},
					},
				},
			},
		},
		{
			desc: "deploys a task - deployment cancelled",
			taskConfigs: []discover.TaskConfig{
				{
					TaskID:   "tsk123",
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
					},
				},
			},
			existingTasks:         map[string]libapi.Task{"my_task": {ID: "tsk123", Slug: "my_task", Name: "My Task", InterpolationMode: "jst"}},
			getDeploymentResponse: &api.Deployment{CancelledAt: &now},
			expectedError:         errors.New("Deploy cancelled"),
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "tsk123",
							Kind:   "node",
							BuildConfig: libBuild.BuildConfig{
								"entrypoint": "",
								"shim":       "true",
							},
							UploadID: "uploadID",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Resources:  map[string]string{},
								Configs:    &[]libapi.ConfigAttachment{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint": "",
								},
								ExecuteRules: libapi.UpdateExecuteRulesRequest{
									DisallowSelfApprove: pointers.Bool(false),
									RequireRequests:     pointers.Bool(false),
								},
								InterpolationMode: pointers.String("jst"),
								Timeout:           3600,
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
					TaskID:   "tsk123",
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {ID: "tsk123", Slug: "my_task", Name: "My Task", InterpolationMode: "jst"}},
			envSlug:       "myEnv",
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "tsk123",
							Kind:   "node",
							BuildConfig: libBuild.BuildConfig{
								"entrypoint": "",
								"runtime":    libBuild.TaskRuntimeStandard,
								"shim":       "true",
							},
							UploadID: "uploadID",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Resources:  map[string]string{},
								Configs:    &[]libapi.ConfigAttachment{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint": "",
								},
								ExecuteRules: libapi.UpdateExecuteRulesRequest{
									DisallowSelfApprove: pointers.Bool(false),
									RequireRequests:     pointers.Bool(false),
								},
								InterpolationMode: pointers.String("jst"),
								EnvSlug:           "myEnv",
								Timeout:           3600,
							},
						},
					},
					EnvSlug: "myEnv",
				},
			},
		},
		{
			desc: "deploys a task that doesn't need to be built",
			taskConfigs: []discover.TaskConfig{
				{
					TaskID:   "tsk123",
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name:  "My Task",
						Slug:  "my_task",
						Image: &definitions.ImageDefinition_0_3{Image: "myImage"},
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {ID: "tsk123", Slug: "my_task", Name: "My Task", InterpolationMode: "jst"}},
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "tsk123",
							Kind:   "image",
							BuildConfig: libBuild.BuildConfig{
								"runtime": libBuild.TaskRuntimeStandard,
							},
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Resources:  map[string]string{},
								Configs:    &[]libapi.ConfigAttachment{},
								Kind:       "image",
								Runtime:    "",
								Command:    []string{},
								Image:      pointers.String("myImage"),
								Arguments:  []string{},
								ExecuteRules: libapi.UpdateExecuteRulesRequest{
									DisallowSelfApprove: pointers.Bool(false),
									RequireRequests:     pointers.Bool(false),
								},
								InterpolationMode: pointers.String("jst"),
								Timeout:           3600,
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
					TaskID:   "tsk123",
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {ID: "tsk123", Slug: "my_task", Name: "My Task", InterpolationMode: "jst"}},
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "tsk123",
							Kind:   "node",
							BuildConfig: libBuild.BuildConfig{
								"entrypoint": "",
								"runtime":    libBuild.TaskRuntimeStandard,
								"shim":       "true",
							},

							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Resources:  map[string]string{},
								Configs:    &[]libapi.ConfigAttachment{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint": "",
								},
								Runtime: "",
								Image:   pointers.String("imageURL"),
								ExecuteRules: libapi.UpdateExecuteRulesRequest{
									DisallowSelfApprove: pointers.Bool(false),
									RequireRequests:     pointers.Bool(false),
								},
								InterpolationMode: pointers.String("jst"),
								Timeout:           3600,
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
				// Task that has no root e.g. a REST task.
				{
					TaskRoot: "",
				},
			},
			changedFiles: []string{"some/random/path.js"},
		},
		{
			desc: "deploys a task with git metadata",
			taskConfigs: []discover.TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: "/json/short.json",
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {ID: "tsk123", Slug: "my_task", Name: "My Task", InterpolationMode: "jst"}},
			gitRepo:       mockRepo,
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "tsk123",
							Kind:   "node",
							BuildConfig: libBuild.BuildConfig{
								"entrypoint": "",
								"runtime":    libBuild.TaskRuntimeStandard,
								"shim":       "true",
							},
							UploadID: "uploadID",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Resources:  map[string]string{},
								Configs:    &[]libapi.ConfigAttachment{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint": "",
								},
								ExecuteRules: libapi.UpdateExecuteRulesRequest{
									DisallowSelfApprove: pointers.Bool(false),
									RequireRequests:     pointers.Bool(false),
								},
								InterpolationMode: pointers.String("jst"),
								Timeout:           3600,
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
		{
			desc: "deploys a task with owner and repo from env var",
			taskConfigs: []discover.TaskConfig{
				{
					TaskID:         "tsk123",
					TaskRoot:       fixturesPath,
					TaskEntrypoint: "/json/short.json",
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {ID: "tsk123", Slug: "my_task", Name: "My Task", InterpolationMode: "jst"}},
			gitRepo:       mockRepo,
			envVars: map[string]string{
				"AP_GIT_REPO": "airplanedev/airport",
			},
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "tsk123",
							Kind:   "node",
							BuildConfig: libBuild.BuildConfig{
								"entrypoint": "",
								"runtime":    libBuild.TaskRuntimeStandard,
								"shim":       "true",
							},
							UploadID: "uploadID",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Resources:  map[string]string{},
								Configs:    &[]libapi.ConfigAttachment{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint": "",
								},
								Runtime: "",
								ExecuteRules: libapi.UpdateExecuteRulesRequest{
									DisallowSelfApprove: pointers.Bool(false),
									RequireRequests:     pointers.Bool(false),
								},
								InterpolationMode: pointers.String("jst"),
								Timeout:           3600,
							},
							GitFilePath: "json/short.json",
						},
					},
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
		{
			desc: "deploys and updates an SQL task with config attachments",
			taskConfigs: []discover.TaskConfig{
				{
					TaskID:   "tsk123",
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						SQL: &definitions.SQLDefinition_0_3{
							Entrypoint: "./fixtures/test.sql",
							QueryArgs:  map[string]interface{}{},
							Resource:   "db",
							Configs:    []string{"API_KEY"},
						},
					},
				},
			},
			absoluteEntrypoints: []string{
				fixturesPath + "/test.sql",
			},
			existingTasks: map[string]libapi.Task{"my_task": {ID: "tsk123", Slug: "my_task", Name: "My Task", InterpolationMode: "jst"}},
			resources: []libapi.Resource{
				{
					ID:   "db_id",
					Name: "db",
				},
			},
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "tsk123",
							Kind:   "sql",
							BuildConfig: libBuild.BuildConfig{
								"entrypoint":      "./fixtures/test.sql",
								"query":           "SELECT 1;\n",
								"queryArgs":       map[string]interface{}{},
								"runtime":         libBuild.TaskRuntimeStandard,
								"transactionMode": "auto",
							},
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Configs: &[]libapi.ConfigAttachment{{
									NameTag: "API_KEY",
								}},
								Kind: "sql",
								KindOptions: libBuild.KindOptions{
									"entrypoint":      "./fixtures/test.sql",
									"query":           "SELECT 1;\n",
									"queryArgs":       map[string]interface{}{},
									"transactionMode": "auto",
								},
								Runtime: "",
								Resources: map[string]string{
									"db": "db_id",
								},
								ExecuteRules: libapi.UpdateExecuteRulesRequest{
									DisallowSelfApprove: pointers.Bool(false),
									RequireRequests:     pointers.Bool(false),
								},
								InterpolationMode: pointers.String("jst"),
								Timeout:           3600,
							},
						},
					},
				},
			},
		},
		{
			desc: "deploys a task with schedules",
			taskConfigs: []discover.TaskConfig{
				{
					TaskID:   "tsk123",
					TaskRoot: fixturesPath,
					Def: &definitions.Definition_0_3{
						Name: "My Task",
						Slug: "my_task",
						Node: &definitions.NodeDefinition_0_3{},
						Schedules: map[string]definitions.ScheduleDefinition_0_3{
							"my_schedule": {
								Name:        "foo",
								Description: "foo",
								CronExpr:    "0 0 * * *",
								ParamValues: map[string]interface{}{
									"my_param": 5,
								},
							},
						},
					},
				},
			},
			existingTasks: map[string]libapi.Task{"my_task": {ID: "tsk123", Slug: "my_task", Name: "My Task", InterpolationMode: "jst"}},
			deploys: []api.CreateDeploymentRequest{
				{
					Tasks: []api.DeployTask{
						{
							TaskID: "tsk123",
							Kind:   "node",
							BuildConfig: libBuild.BuildConfig{
								"entrypoint": "",
								"runtime":    libBuild.TaskRuntimeStandard,
								"shim":       "true",
							},
							UploadID: "uploadID",
							UpdateTaskRequest: libapi.UpdateTaskRequest{
								Slug:       "my_task",
								Name:       "My Task",
								Parameters: libapi.Parameters{},
								Resources:  map[string]string{},
								Configs:    &[]libapi.ConfigAttachment{},
								Kind:       "node",
								KindOptions: libBuild.KindOptions{
									"entrypoint": "",
								},
								Runtime: "",
								ExecuteRules: libapi.UpdateExecuteRulesRequest{
									DisallowSelfApprove: pointers.Bool(false),
									RequireRequests:     pointers.Bool(false),
								},
								InterpolationMode: pointers.String("jst"),
								Timeout:           3600,
							},
							Schedules: map[string]libapi.Schedule{
								"my_schedule": {
									Name:        "foo",
									Description: "foo",
									CronExpr:    "0 0 * * *",
									ParamValues: map[string]interface{}{
										"my_param": 5,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "deploys a view",
			viewConfigs: []discover.ViewConfig{
				{
					Root: fixturesPath, Source: discover.ConfigSourceDefn, ID: "view123", Def: definitions.ViewDefinition{
						Slug:        "my_view",
						Name:        "my view",
						Entrypoint:  filepath.Join(fixturesPath, "my_view.tsx"),
						Description: "My view's description",
					},
				},
			},
			deploys: []api.CreateDeploymentRequest{
				{
					Views: []api.DeployView{
						{
							ID:       "view123",
							UploadID: "uploadID",
							UpdateViewRequest: libapi.UpdateViewRequest{
								Slug:        "my_view",
								Name:        "my view",
								Description: "My view's description",
							},
							BuildConfig: libBuild.BuildConfig{
								"apiHost":    api.Host,
								"entrypoint": "my_view.tsx",
							},
						},
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
				Tasks:                 tC.existingTasks,
				Views:                 tC.existingViews,
				GetDeploymentResponse: tC.getDeploymentResponse,
				Resources:             tC.resources,
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
				local:        tC.local,
				envSlug:      tC.envSlug,
				root: &cli.Config{
					Client: &api.Client{
						Host: api.Host,
					},
				},
			}
			d := NewDeployer(cfg, &logger.MockLogger{}, DeployerOpts{
				BuildCreator: &build.MockBuildCreator{},
				Archiver:     &archive.MockArchiver{},
				RepoGetter:   &MockGitRepoGetter{Repo: tC.gitRepo},
			})
			for i, absEntrypoint := range tC.absoluteEntrypoints {
				err := tC.taskConfigs[i].Def.SetAbsoluteEntrypoint(absEntrypoint)
				require.NoError(err)
			}
			err := d.Deploy(context.Background(), tC.taskConfigs, tC.viewConfigs, map[string]bool{})
			if tC.expectedError != nil {
				assert.Error(err)
				return
			} else {
				require.NoError(err)
			}

			assert.Equal(tC.existingTasks, client.Tasks)
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
			desc:      "git http with .git suffix",
			remote:    "https://github.com/airplanedev/airport.git",
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

func TestGetDefinitionDiff(t *testing.T) {
	for _, test := range []struct {
		name          string
		taskConfig    discover.TaskConfig
		isNew         bool
		existingTasks map[string]libapi.Task
		expected      []string
	}{
		{
			name:     "new task",
			isNew:    true,
			expected: []string{"(new task)"},
		},
		{
			name: "no changes",
			taskConfig: discover.TaskConfig{
				TaskID: "my_task",
				Def: &definitions.Definition_0_3{
					Name: "My Task",
					Slug: "my_task",
					Image: &definitions.ImageDefinition_0_3{
						Image:   "ubuntu:latest",
						Command: "echo 'hello world'",
					},
				},
			},
			existingTasks: map[string]libapi.Task{
				"my_task": {
					ID:        "my_task",
					Slug:      "my_task",
					Name:      "My Task",
					Kind:      "image",
					Image:     pointers.String("ubuntu:latest"),
					Arguments: []string{"echo", "hello world"},
				},
			},
			expected: []string{"(no changes to task definition)"},
		},
		{
			name: "show diff",
			taskConfig: discover.TaskConfig{
				TaskID: "my_task",
				Def: &definitions.Definition_0_3{
					Name:        "My Task",
					Description: "Says hello!",
					Slug:        "my_task",
					Image: &definitions.ImageDefinition_0_3{
						Image:   "ubuntu:latest",
						Command: "echo 'hello world'",
					},
				},
			},
			existingTasks: map[string]libapi.Task{
				"my_task": {
					ID:        "my_task",
					Slug:      "my_task",
					Name:      "My Task",
					Kind:      "image",
					Image:     pointers.String("ubuntu:latest"),
					Arguments: []string{"echo", "hello world"},
				},
			},
			expected: []string{
				"--- a/",
				"+++ b/",
				"@@ -1,5 +1,6 @@",
				" name: My Task",
				" slug: my_task",
				"+description: Says hello!",
				" docker:",
				"   image: ubuntu:latest",
				"   command: echo 'hello world'",
				"",
			},
		},
		{
			name: "deploy task into new environment",
			taskConfig: discover.TaskConfig{
				TaskID: "my_task",
				Def: &definitions.Definition_0_3{
					Name: "My Task",
					Slug: "my_task",
					Image: &definitions.ImageDefinition_0_3{
						Image:   "ubuntu:latest",
						Command: "echo 'hello world'",
					},
				},
			},
			existingTasks: map[string]libapi.Task{},
			expected:      []string{"(task created in new environment)"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)

			cfg := config{
				client: &api.MockClient{
					Tasks: test.existingTasks,
				},
			}
			d := NewDeployer(cfg, &logger.MockLogger{}, DeployerOpts{})
			diff, err := d.getDefinitionDiff(context.Background(), test.taskConfig, test.isNew)
			require.NoError(err)
			require.Equal(test.expected, diff)
		})
	}
}
