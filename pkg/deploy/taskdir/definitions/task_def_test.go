package definitions

import (
	"testing"
	"time"

	"github.com/airplanedev/cli/pkg/api"
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/stretchr/testify/require"
)

// Contains explicit defaults.
var fullYAML = []byte(
	`name: Hello World
slug: hello_world
description: A starter task.
parameters:
- name: Name
  slug: name
  type: shorttext
  description: Someone's name.
  default: World
  required: true
resources:
  db: demo_db
python:
  entrypoint: hello_world.py
timeout: 3600
schedules:
  every_midnight:
    name: Every Midnight
    cron: 0 0 * * *
  no_name_params:
    cron: 0 0 * * *
    paramValues:
      param_one: 5.5
      param_two: memes
permissions:
  viewers:
    groups:
    - group1
    users:
    - user1
  requesters:
    groups:
    - group2
  executers:
    groups:
    - group3
    - group4
  admins:
    groups:
    - group5
`)

// Contains explicit defaults.
var fullJSON = []byte(
	`{
	"name": "Hello World",
	"slug": "hello_world",
	"description": "A starter task.",
	"parameters": [
		{
			"name": "Name",
			"slug": "name",
			"type": "shorttext",
			"description": "Someone's name.",
			"default": "World",
			"required": true
		}
	],
	"resources": {
		"db": "demo_db"
	},
	"python": {
		"entrypoint": "hello_world.py"
	},
	"timeout": 3600,
	"schedules": {
		"every_midnight": {
			"name": "Every Midnight",
			"cron": "0 0 * * *"
		},
		"no_name_params": {
			"cron": "0 0 * * *",
			"paramValues": {
				"param_one": 5.5,
				"param_two": "memes"
			}
		}
	},
	"permissions": {
		"viewers": {
			"groups": [
				"group1"
			],
			"users": [
				"user1"
			]
		},
		"requesters": {
			"groups": [
				"group2"
			]
		},
		"executers": {
			"groups": [
				"group3",
				"group4"
			]
		},
		"admins": {
			"groups": [
				"group5"
			]
		}
	}
}`)

// Contains no explicit defaults.
var yamlWithDefault = []byte(
	`slug: hello_world
name: Hello World
description: A starter task.
parameters:
- slug: name
  name: Name
  description: Someone's name.
  type: shorttext
  default: World
resources:
  db: demo_db
python:
  entrypoint: hello_world.py
timeout: 3600
schedules:
  every_midnight:
    name: Every Midnight
    cron: 0 0 * * *
  no_name_params:
    cron: 0 0 * * *
    paramValues:
      param_one: 5.5
      param_two: memes
permissions:
  admins:
    groups:
    - group5
  executers:
    groups:
    - group3
    - group4
  requesters:
    groups:
    - group2
  viewers:
    groups:
    - group1
    users:
    - user1
`)

// Contains no explicit defaults.
var jsonWithDefault = []byte(
	`{
	"slug": "hello_world",
	"name": "Hello World",
	"description": "A starter task.",
	"parameters": [
		{
			"slug": "name",
			"name": "Name",
			"description": "Someone's name.",
			"type": "shorttext",
			"default": "World"
		}
	],
	"resources": {
		"db": "demo_db"
	},
	"python": {
		"entrypoint": "hello_world.py"
	},
	"timeout": 3600,
	"schedules": {
		"every_midnight": {
			"name": "Every Midnight",
			"cron": "0 0 * * *"
		},
		"no_name_params": {
			"cron": "0 0 * * *",
			"paramValues": {
				"param_one": 5.5,
				"param_two": "memes"
			}
		}
	},
	"permissions": {
		"admins": {
			"groups": [
				"group5"
			]
		},
		"executers": {
			"groups": [
				"group3",
				"group4"
			]
		},
		"requesters": {
			"groups": [
				"group2"
			]
		},
		"viewers": {
			"groups": [
				"group1"
			],
			"users": [
				"user1"
			]
		}
	}
}
`)

// Permissions are set to team_access.
// Contains no explicit defaults.
var yamlWithDefaultAndTeamAccessPermissions = []byte(
	`slug: hello_world
name: Hello World
description: A starter task.
parameters:
- slug: name
  name: Name
  description: Someone's name.
  type: shorttext
  default: World
resources:
  db: demo_db
python:
  entrypoint: hello_world.py
timeout: 3600
schedules:
  every_midnight:
    name: Every Midnight
    cron: 0 0 * * *
  no_name_params:
    cron: 0 0 * * *
    paramValues:
      param_one: 5.5
      param_two: memes
permissions: team_access
`)

// Permissions are set to team_access.
// Contains no explicit defaults.
var jsonWithDefaultAndTeamAccessPermissions = []byte(
	`{
	"slug": "hello_world",
	"name": "Hello World",
	"description": "A starter task.",
	"parameters": [
		{
			"slug": "name",
			"name": "Name",
			"description": "Someone's name.",
			"type": "shorttext",
			"default": "World"
		}
	],
	"resources": {
		"db": "demo_db"
	},
	"python": {
		"entrypoint": "hello_world.py"
	},
	"timeout": 3600,
	"schedules": {
		"every_midnight": {
			"name": "Every Midnight",
			"cron": "0 0 * * *"
		},
		"no_name_params": {
			"cron": "0 0 * * *",
			"paramValues": {
				"param_one": 5.5,
				"param_two": "memes"
			}
		}
	},
	"permissions": "team_access"
}
`)

// Contains explicit defaults.
var fullDef = Definition{
	Name:        "Hello World",
	Slug:        "hello_world",
	Description: "A starter task.",
	Parameters: []ParameterDefinition{
		{
			Name:        "Name",
			Slug:        "name",
			Type:        "shorttext",
			Description: "Someone's name.",
			Default:     "World",
			Required:    DefaultTrueDefinition{pointers.Bool(true)},
		},
	},
	Resources: map[string]string{"db": "demo_db"},
	Python: &PythonDefinition{
		Entrypoint: "hello_world.py",
	},
	Timeout: 3600,
	Schedules: map[string]ScheduleDefinition{
		"every_midnight": {
			Name:     "Every Midnight",
			CronExpr: "0 0 * * *",
		},
		"no_name_params": {
			CronExpr: "0 0 * * *",
			ParamValues: map[string]interface{}{
				"param_one": 5.5,
				"param_two": "memes",
			},
		},
	},
	Permissions: &PermissionsDefinition{
		Viewers:                    PermissionRecipients{Groups: []string{"group1"}, Users: []string{"user1"}},
		Requesters:                 PermissionRecipients{Groups: []string{"group2"}},
		Executers:                  PermissionRecipients{Groups: []string{"group3", "group4"}},
		Admins:                     PermissionRecipients{Groups: []string{"group5"}},
		RequireExplicitPermissions: true,
	},
}

// Contains no explicit defaults.
var defWithDefault = Definition{
	Name:        "Hello World",
	Slug:        "hello_world",
	Description: "A starter task.",
	Parameters: []ParameterDefinition{
		{
			Name:        "Name",
			Slug:        "name",
			Type:        "shorttext",
			Description: "Someone's name.",
			Default:     "World",
		},
	},
	Python: &PythonDefinition{
		Entrypoint: "hello_world.py",
	},
	Timeout:   3600,
	Resources: map[string]string{"db": "demo_db"},
	Schedules: map[string]ScheduleDefinition{
		"every_midnight": {
			Name:     "Every Midnight",
			CronExpr: "0 0 * * *",
		},
		"no_name_params": {
			CronExpr: "0 0 * * *",
			ParamValues: map[string]interface{}{
				"param_one": 5.5,
				"param_two": "memes",
			},
		},
	},
	Permissions: &PermissionsDefinition{
		Viewers:                    PermissionRecipients{Groups: []string{"group1"}, Users: []string{"user1"}},
		Requesters:                 PermissionRecipients{Groups: []string{"group2"}},
		Executers:                  PermissionRecipients{Groups: []string{"group3", "group4"}},
		Admins:                     PermissionRecipients{Groups: []string{"group5"}},
		RequireExplicitPermissions: true,
	},
}

var defWithDefaultAndTeamAccessPermissions = Definition{
	Name:        "Hello World",
	Slug:        "hello_world",
	Description: "A starter task.",
	Parameters: []ParameterDefinition{
		{
			Name:        "Name",
			Slug:        "name",
			Type:        "shorttext",
			Description: "Someone's name.",
			Default:     "World",
		},
	},
	Python: &PythonDefinition{
		Entrypoint: "hello_world.py",
	},
	Timeout:   3600,
	Resources: map[string]string{"db": "demo_db"},
	Schedules: map[string]ScheduleDefinition{
		"every_midnight": {
			Name:     "Every Midnight",
			CronExpr: "0 0 * * *",
		},
		"no_name_params": {
			CronExpr: "0 0 * * *",
			ParamValues: map[string]interface{}{
				"param_one": 5.5,
				"param_two": "memes",
			},
		},
	},
	Permissions: &PermissionsDefinition{
		RequireExplicitPermissions: false,
	},
}

func TestDefinitionMarshal(t *testing.T) {
	// marshalling tests
	for _, test := range []struct {
		name     string
		format   DefFormat
		def      Definition
		expected []byte
	}{
		{
			name:     "marshal yaml",
			format:   DefFormatYAML,
			def:      fullDef,
			expected: yamlWithDefault,
		},
		{
			name:     "marshal json",
			format:   DefFormatJSON,
			def:      fullDef,
			expected: jsonWithDefault,
		},
		{
			name:     "marshal yaml with team access permissions",
			format:   DefFormatYAML,
			def:      defWithDefaultAndTeamAccessPermissions,
			expected: yamlWithDefaultAndTeamAccessPermissions,
		},
		{
			name:     "marshal json with team access permissions",
			format:   DefFormatJSON,
			def:      defWithDefaultAndTeamAccessPermissions,
			expected: jsonWithDefaultAndTeamAccessPermissions,
		},

		{
			name:   "marshal yaml with multiline",
			format: DefFormatYAML,
			def: Definition{
				Name: "REST task",
				Slug: "rest_task",
				REST: &RESTDefinition{
					Resource: "httpbin",
					Method:   "POST",
					Path:     "/post",
					BodyType: "json",
					Body:     "{\n  \"name\": \"foo\",\n  \"number\": 30\n}\n",
				},
				Timeout: 300,
			},
			expected: []byte(
				`slug: rest_task
name: REST task
rest:
  resource: httpbin
  method: POST
  path: /post
  bodyType: json
  body: |
    {
      "name": "foo",
      "number": 30
    }
timeout: 300
`),
		},
		{
			name:   "marshal json with multiline",
			format: DefFormatJSON,
			def: Definition{
				Name: "REST task",
				Slug: "rest_task",
				REST: &RESTDefinition{
					Resource: "httpbin",
					Method:   "POST",
					Path:     "/post",
					BodyType: "json",
					Body:     "{\n  \"name\": \"foo\",\n  \"number\": 30\n}\n",
				},
				Timeout: 300,
			},
			expected: []byte(
				`{
	"slug": "rest_task",
	"name": "REST task",
	"rest": {
		"resource": "httpbin",
		"method": "POST",
		"path": "/post",
		"bodyType": "json",
		"body": "{\n  \"name\": \"foo\",\n  \"number\": 30\n}\n"
	},
	"timeout": 300
}
`),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert := require.New(t)
			bytestr, err := test.def.Marshal(test.format)
			assert.NoError(err)
			assert.Equal(test.expected, bytestr)
		})
	}
}

func TestDefinitionUnmarshal(t *testing.T) {

	for _, test := range []struct {
		name     string
		format   DefFormat
		bytestr  []byte
		expected Definition
	}{
		{
			name:     "unmarshal yaml",
			format:   DefFormatYAML,
			bytestr:  fullYAML,
			expected: fullDef,
		},
		{
			name:     "unmarshal json",
			format:   DefFormatJSON,
			bytestr:  fullJSON,
			expected: fullDef,
		},
		{
			name:     "unmarshal yaml with default",
			format:   DefFormatYAML,
			bytestr:  yamlWithDefault,
			expected: defWithDefault,
		},
		{
			name:     "unmarshal json with default",
			format:   DefFormatJSON,
			bytestr:  jsonWithDefault,
			expected: defWithDefault,
		},
		{
			name:     "marshal yaml with team access permissions",
			format:   DefFormatYAML,
			bytestr:  yamlWithDefaultAndTeamAccessPermissions,
			expected: defWithDefaultAndTeamAccessPermissions,
		},
		{
			name:     "marshal json with team access permissions",
			format:   DefFormatJSON,
			bytestr:  jsonWithDefaultAndTeamAccessPermissions,
			expected: defWithDefaultAndTeamAccessPermissions,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert := require.New(t)
			d := Definition{}
			err := d.Unmarshal(test.format, test.bytestr)
			assert.NoError(err)
			assert.Equal(test.expected, d)
		})
	}
}

func TestTaskToDefinition(t *testing.T) {
	exampleCron := api.CronExpr{
		Minute:     "0",
		Hour:       "0",
		DayOfMonth: "1",
		Month:      "*",
		DayOfWeek:  "*",
	}
	exampleTime := time.Date(1996, time.May, 3, 0, 0, 0, 0, time.UTC)

	for _, test := range []struct {
		name       string
		task       api.Task
		definition Definition
		resources  []api.ResourceMetadata
	}{
		{
			name: "python task",
			task: api.Task{
				Name:        "Python Task",
				Slug:        "python_task",
				Description: "A task for testing",
				Arguments:   []string{"{{JSON.stringify(params)}}"},
				Kind:        buildtypes.TaskKindPython,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.py",
				},
				Env: api.TaskEnv{
					"value": api.EnvVarValue{
						Value: pointers.String("value"),
					},
					"config": api.EnvVarValue{
						Config: pointers.String("config"),
					},
				},
			},
			definition: Definition{
				Name:        "Python Task",
				Slug:        "python_task",
				Description: "A task for testing",
				Parameters:  []ParameterDefinition{},
				Python: &PythonDefinition{
					Entrypoint: "main.py",
					EnvVars: api.TaskEnv{
						"value": api.EnvVarValue{
							Value: pointers.String("value"),
						},
						"config": api.EnvVarValue{
							Config: pointers.String("config"),
						},
					},
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
				Resources:          map[string]string{},
			},
		},
		{
			name: "node task",
			task: api.Task{
				Name:      "Node Task",
				Slug:      "node_task",
				Arguments: []string{"{{JSON.stringify(params)}}"},
				Kind:      buildtypes.TaskKindNode,
				KindOptions: buildtypes.KindOptions{
					"entrypoint":  "main.ts",
					"nodeVersion": "14",
				},
				Env: api.TaskEnv{
					"value": api.EnvVarValue{
						Value: pointers.String("value"),
					},
					"config": api.EnvVarValue{
						Config: pointers.String("config"),
					},
				},
			},
			definition: Definition{
				Name:       "Node Task",
				Slug:       "node_task",
				Parameters: []ParameterDefinition{},
				Node: &NodeDefinition{
					Entrypoint:  "main.ts",
					NodeVersion: "14",
					EnvVars: api.TaskEnv{
						"value": api.EnvVarValue{
							Value: pointers.String("value"),
						},
						"config": api.EnvVarValue{
							Config: pointers.String("config"),
						},
					},
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
				Resources:          map[string]string{},
			},
		},
		{
			name: "node task without version",
			task: api.Task{
				Name:      "Node Task",
				Slug:      "node_task",
				Arguments: []string{"{{JSON.stringify(params)}}"},
				Kind:      buildtypes.TaskKindNode,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.ts",
				},
			},
			definition: Definition{
				Name:       "Node Task",
				Slug:       "node_task",
				Parameters: []ParameterDefinition{},
				Node: &NodeDefinition{
					Entrypoint: "main.ts",
					EnvVars:    api.TaskEnv{},
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
				Resources:          map[string]string{},
			},
		},
		{
			name: "node task slim image",
			task: api.Task{
				Name:      "Node Task",
				Slug:      "node_task",
				Arguments: []string{"{{JSON.stringify(params)}}"},
				Kind:      buildtypes.TaskKindNode,
				KindOptions: buildtypes.KindOptions{
					"entrypoint":  "main.ts",
					"nodeVersion": "14",
					"base":        buildtypes.BuildBaseSlim,
				},
				Env: api.TaskEnv{
					"value": api.EnvVarValue{
						Value: pointers.String("value"),
					},
					"config": api.EnvVarValue{
						Config: pointers.String("config"),
					},
				},
			},
			definition: Definition{
				Name:       "Node Task",
				Slug:       "node_task",
				Parameters: []ParameterDefinition{},
				Node: &NodeDefinition{
					Entrypoint:  "main.ts",
					NodeVersion: "14",
					EnvVars: api.TaskEnv{
						"value": api.EnvVarValue{
							Value: pointers.String("value"),
						},
						"config": api.EnvVarValue{
							Config: pointers.String("config"),
						},
					},
					Base: buildtypes.BuildBaseSlim,
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
				Resources:          map[string]string{},
			},
		},
		{
			name: "shell task",
			task: api.Task{
				Name:      "Shell Task",
				Slug:      "shell_task",
				Arguments: []string{},
				Kind:      buildtypes.TaskKindShell,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.sh",
				},
				Env: api.TaskEnv{
					"value": api.EnvVarValue{
						Value: pointers.String("value"),
					},
					"config": api.EnvVarValue{
						Config: pointers.String("config"),
					},
				},
			},
			definition: Definition{
				Name:       "Shell Task",
				Slug:       "shell_task",
				Parameters: []ParameterDefinition{},
				Shell: &ShellDefinition{
					Entrypoint: "main.sh",
					EnvVars: api.TaskEnv{
						"value": api.EnvVarValue{
							Value: pointers.String("value"),
						},
						"config": api.EnvVarValue{
							Config: pointers.String("config"),
						},
					},
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
				Resources:          map[string]string{},
			},
		},
		{
			name: "image task",
			task: api.Task{
				Name:        "Image Task",
				Slug:        "image_task",
				Command:     []string{"bash"},
				Arguments:   []string{"-c", `echo "foobar"`},
				Kind:        buildtypes.TaskKindImage,
				KindOptions: buildtypes.KindOptions{},
				Image:       pointers.String("ubuntu:latest"),
				Env: api.TaskEnv{
					"value": api.EnvVarValue{
						Value: pointers.String("value"),
					},
					"config": api.EnvVarValue{
						Config: pointers.String("config"),
					},
				},
			},
			definition: Definition{
				Name:       "Image Task",
				Slug:       "image_task",
				Parameters: []ParameterDefinition{},
				Image: &ImageDefinition{
					Image:      "ubuntu:latest",
					Entrypoint: "bash",
					Command:    `-c 'echo "foobar"'`,
					EnvVars: api.TaskEnv{
						"value": api.EnvVarValue{
							Value: pointers.String("value"),
						},
						"config": api.EnvVarValue{
							Config: pointers.String("config"),
						},
					},
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
				Resources:          map[string]string{},
			},
		},
		{
			name: "rest task",
			resources: []api.ResourceMetadata{
				{
					ID:   "res20220111foobarx",
					Slug: "httpbin",
					DefaultEnvResource: &api.Resource{
						Name: "httpbin",
					},
				},
			},
			task: api.Task{
				Name:      "REST Task",
				Slug:      "rest_task",
				Arguments: []string{"{{__stdAPIRequest}}"},
				Kind:      buildtypes.TaskKindREST,
				KindOptions: buildtypes.KindOptions{
					"method": "GET",
					"path":   "/get",
					"urlParams": map[string]interface{}{
						"foo": "bar",
					},
					"headers": map[string]interface{}{
						"bar": "foo",
					},
					"bodyType": "json",
					"body":     "",
					"formData": map[string]interface{}{},
				},
				Resources: map[string]string{
					"rest": "res20220111foobarx",
				},
			},
			definition: Definition{
				Name:       "REST Task",
				Slug:       "rest_task",
				Parameters: []ParameterDefinition{},
				Resources:  map[string]string{},
				REST: &RESTDefinition{
					Resource: "httpbin",
					Method:   "GET",
					Path:     "/get",
					URLParams: map[string]interface{}{
						"foo": "bar",
					},
					Headers: map[string]interface{}{
						"bar": "foo",
					},
					BodyType: "json",
					Body:     "",
					FormData: map[string]interface{}{},
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
			},
		},
		{
			name: "check parameters",
			task: api.Task{
				Name: "Test Task",
				Slug: "test_task",
				Parameters: []api.Parameter{
					{
						Name: "Required boolean",
						Slug: "required_boolean",
						Type: api.TypeBoolean,
						Desc: "A required boolean.",
					},
					{
						Name:    "Short text",
						Slug:    "short_text",
						Type:    api.TypeString,
						Default: "foobar",
					},
					{
						Name:      "SQL",
						Slug:      "sql",
						Type:      api.TypeString,
						Component: api.ComponentEditorSQL,
					},
					{
						Name:      "Optional long text",
						Slug:      "optional_long_text",
						Type:      api.TypeString,
						Component: api.ComponentTextarea,
						Constraints: api.Constraints{
							Optional: true,
						},
					},
					{
						Name: "Options",
						Slug: "options",
						Type: api.TypeString,
						Constraints: api.Constraints{
							Options: []api.ConstraintOption{
								{
									Label: "one",
									Value: 1,
								},
								{
									Label: "two",
									Value: 2,
								},
								{
									Label: "three",
									Value: 3,
								},
							},
						},
					},
					{
						Name: "Regex",
						Slug: "regex",
						Type: api.TypeString,
						Constraints: api.Constraints{
							Regex: "foo.*",
						},
					},
					{
						Name: "Config var",
						Slug: "config_var",
						Type: api.TypeConfigVar,
						Default: map[string]interface{}{
							"__airplaneType": "configvar",
							"name":           "API_KEY",
						},
						Constraints: api.Constraints{
							Options: []api.ConstraintOption{
								{
									Label: "API key",
									Value: map[string]interface{}{
										"__airplaneType": "configvar",
										"name":           "API_KEY",
									},
								},
								{
									Label: "Other API key",
									Value: map[string]interface{}{
										"__airplaneType": "configvar",
										"name":           "OTHER_API_KEY",
									},
								},
							},
						},
					},
				},
				Arguments: []string{"{{JSON.stringify(params)}}"},
				Kind:      buildtypes.TaskKindPython,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.py",
				},
			},
			definition: Definition{
				Name: "Test Task",
				Slug: "test_task",
				Parameters: []ParameterDefinition{
					{
						Name:        "Required boolean",
						Slug:        "required_boolean",
						Type:        "boolean",
						Description: "A required boolean.",
						Required:    DefaultTrueDefinition{pointers.Bool(true)},
					},
					{
						Name:     "Short text",
						Slug:     "short_text",
						Type:     "shorttext",
						Default:  "foobar",
						Required: DefaultTrueDefinition{pointers.Bool(true)},
					},
					{
						Name:     "SQL",
						Slug:     "sql",
						Type:     "sql",
						Required: DefaultTrueDefinition{pointers.Bool(true)},
					},
					{
						Name:     "Optional long text",
						Slug:     "optional_long_text",
						Type:     "longtext",
						Required: DefaultTrueDefinition{pointers.Bool(false)},
					},
					{
						Name: "Options",
						Slug: "options",
						Type: "shorttext",
						Options: []OptionDefinition{
							{
								Label: "one",
								Value: 1,
							},
							{
								Label: "two",
								Value: 2,
							},
							{
								Label: "three",
								Value: 3,
							},
						},
						Required: DefaultTrueDefinition{pointers.Bool(true)},
					},
					{
						Name:     "Regex",
						Slug:     "regex",
						Type:     "shorttext",
						Regex:    "foo.*",
						Required: DefaultTrueDefinition{pointers.Bool(true)},
					},
					{
						Name:    "Config var",
						Slug:    "config_var",
						Type:    "configvar",
						Default: "API_KEY",
						Options: []OptionDefinition{
							{
								Label: "API key",
								Value: "API_KEY",
							},
							{
								Label: "Other API key",
								Value: "OTHER_API_KEY",
							},
						},
						Required: DefaultTrueDefinition{pointers.Bool(true)},
					},
				},
				Python: &PythonDefinition{
					Entrypoint: "main.py",
					EnvVars:    api.TaskEnv{},
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
				Resources:          map[string]string{},
			},
		},
		{
			name: "check execute rules",
			task: api.Task{
				Name:      "Test Task",
				Slug:      "test_task",
				Arguments: []string{"{{JSON.stringify(params)}}"},
				Kind:      buildtypes.TaskKindPython,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.py",
				},
				ExecuteRules: api.ExecuteRules{
					DisallowSelfApprove: true,
					RequireRequests:     true,
				},
			},
			definition: Definition{
				Name:       "Test Task",
				Slug:       "test_task",
				Parameters: []ParameterDefinition{},
				Python: &PythonDefinition{
					Entrypoint: "main.py",
					EnvVars:    api.TaskEnv{},
				},
				RequireRequests:    true,
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(false)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
				Resources:          map[string]string{},
			},
		},
		{
			name: "check default execute rules",
			task: api.Task{
				Name:      "Test Task",
				Slug:      "test_task",
				Arguments: []string{"{{JSON.stringify(params)}}"},
				Kind:      buildtypes.TaskKindPython,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.py",
				},
				ExecuteRules: api.ExecuteRules{
					DisallowSelfApprove: false,
					RequireRequests:     false,
				},
			},
			definition: Definition{
				Name:       "Test Task",
				Slug:       "test_task",
				Parameters: []ParameterDefinition{},
				Python: &PythonDefinition{
					Entrypoint: "main.py",
					EnvVars:    api.TaskEnv{},
				},
				RequireRequests:    false,
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
				Resources:          map[string]string{},
			},
		},
		{
			name: "check configs",
			resources: []api.ResourceMetadata{
				{
					ID:   "res20220111foobarx",
					Slug: "httpbin",
					DefaultEnvResource: &api.Resource{
						Name: "httpbin",
					},
				},
			},
			task: api.Task{
				Name:      "REST Task",
				Slug:      "rest_task",
				Arguments: []string{"{{__stdAPIRequest}}"},
				Configs: []api.ConfigAttachment{
					{
						NameTag: "CONFIG_NAME_1",
					},
					{
						NameTag: "CONFIG_NAME_2",
					},
				},
				Kind: buildtypes.TaskKindREST,
				KindOptions: buildtypes.KindOptions{
					"method": "GET",
					"path":   "/get",
					"urlParams": map[string]interface{}{
						"foo": "bar",
					},
					"headers": map[string]interface{}{
						"bar": "foo",
					},
					"bodyType": "json",
					"body":     "",
					"formData": map[string]interface{}{},
				},
				Resources: map[string]string{
					"rest": "res20220111foobarx",
				},
			},
			definition: Definition{
				Name:       "REST Task",
				Slug:       "rest_task",
				Parameters: []ParameterDefinition{},
				Resources:  map[string]string{},
				REST: &RESTDefinition{
					Resource: "httpbin",
					Method:   "GET",
					Path:     "/get",
					URLParams: map[string]interface{}{
						"foo": "bar",
					},
					Headers: map[string]interface{}{
						"bar": "foo",
					},
					BodyType: "json",
					Body:     "",
					FormData: map[string]interface{}{},
				},
				Configs:            []string{"CONFIG_NAME_1", "CONFIG_NAME_2"},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
			},
		},
		{
			name: "python task",
			task: api.Task{
				Name:        "Python Task",
				Slug:        "python_task",
				Description: "A task for testing",
				Arguments:   []string{"{{JSON.stringify(params)}}"},
				Kind:        buildtypes.TaskKindPython,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.py",
				},
				Env: api.TaskEnv{
					"value": api.EnvVarValue{
						Value: pointers.String("value"),
					},
					"config": api.EnvVarValue{
						Config: pointers.String("config"),
					},
				},
				Triggers: []api.Trigger{
					{
						Name:        "disabled trigger",
						Description: "disabled trigger",
						Slug:        pointers.String("disabled_trigger"),
						Kind:        api.TriggerKindSchedule,
						KindConfig: api.TriggerKindConfig{
							Schedule: &api.TriggerKindConfigSchedule{
								CronExpr: exampleCron,
							},
						},
						DisabledAt: &exampleTime,
					},
					{
						Name:        "archived trigger",
						Description: "archived trigger",
						Slug:        pointers.String("archived_trigger"),
						Kind:        api.TriggerKindSchedule,
						KindConfig: api.TriggerKindConfig{
							Schedule: &api.TriggerKindConfigSchedule{
								CronExpr: exampleCron,
							},
						},
						ArchivedAt: &exampleTime,
					},
					{
						Name:        "form trigger",
						Description: "form trigger",
						Kind:        api.TriggerKindForm,
						KindConfig: api.TriggerKindConfig{
							Form: &api.TriggerKindConfigForm{},
						},
					},
					{
						Name:        "no slug",
						Description: "no slug",
						Kind:        api.TriggerKindSchedule,
						KindConfig: api.TriggerKindConfig{
							Schedule: &api.TriggerKindConfigSchedule{
								CronExpr: exampleCron,
							},
						},
					},
					{
						Name:        "good schedule",
						Description: "good schedule",
						Slug:        pointers.String("good_schedule"),
						Kind:        api.TriggerKindSchedule,
						KindConfig: api.TriggerKindConfig{
							Schedule: &api.TriggerKindConfigSchedule{
								ParamValues: map[string]interface{}{
									"example_param": "hello",
								},
								CronExpr: exampleCron,
							},
						},
					},
				},
			},
			definition: Definition{
				Name:        "Python Task",
				Slug:        "python_task",
				Description: "A task for testing",
				Parameters:  []ParameterDefinition{},
				Python: &PythonDefinition{
					Entrypoint: "main.py",
					EnvVars: api.TaskEnv{
						"value": api.EnvVarValue{
							Value: pointers.String("value"),
						},
						"config": api.EnvVarValue{
							Config: pointers.String("config"),
						},
					},
				},
				Schedules: map[string]ScheduleDefinition{
					"good_schedule": {
						Name:        "good schedule",
						Description: "good schedule",
						CronExpr:    exampleCron.String(),
						ParamValues: map[string]interface{}{
							"example_param": "hello",
						},
					},
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Resources:          map[string]string{},
			},
		},
		{
			name: "python task slim image",
			task: api.Task{
				Name:        "Python Task",
				Slug:        "python_task",
				Description: "A task for testing",
				Arguments:   []string{"{{JSON.stringify(params)}}"},
				Kind:        buildtypes.TaskKindPython,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.py",
					"base":       buildtypes.BuildBaseSlim,
				},
				Env: api.TaskEnv{
					"value": api.EnvVarValue{
						Value: pointers.String("value"),
					},
					"config": api.EnvVarValue{
						Config: pointers.String("config"),
					},
				},
			},
			definition: Definition{
				Name:        "Python Task",
				Slug:        "python_task",
				Description: "A task for testing",
				Parameters:  []ParameterDefinition{},
				Python: &PythonDefinition{
					Entrypoint: "main.py",
					EnvVars: api.TaskEnv{
						"value": api.EnvVarValue{
							Value: pointers.String("value"),
						},
						"config": api.EnvVarValue{
							Config: pointers.String("config"),
						},
					},
					Base: buildtypes.BuildBaseSlim,
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
				Resources:          map[string]string{},
			},
		},
		{
			name: "simple resources",
			resources: []api.ResourceMetadata{
				{
					ID:   "res20220613localdb",
					Slug: "local_db",
					DefaultEnvResource: &api.Resource{
						Name: "Local DB",
					},
				},
			},
			task: api.Task{
				Name:        "Image Task",
				Slug:        "image_task",
				Command:     []string{"bash"},
				Arguments:   []string{"-c", `echo "foobar"`},
				Kind:        buildtypes.TaskKindImage,
				KindOptions: buildtypes.KindOptions{},
				Image:       pointers.String("ubuntu:latest"),
				Env: api.TaskEnv{
					"value": api.EnvVarValue{
						Value: pointers.String("value"),
					},
					"config": api.EnvVarValue{
						Config: pointers.String("config"),
					},
				},
				Resources: map[string]string{
					"db": "res20220613localdb",
				},
			},
			definition: Definition{
				Name:       "Image Task",
				Slug:       "image_task",
				Parameters: []ParameterDefinition{},
				Resources: map[string]string{
					"db": "local_db",
				},
				Image: &ImageDefinition{
					Image:      "ubuntu:latest",
					Entrypoint: "bash",
					Command:    `-c 'echo "foobar"'`,
					EnvVars: api.TaskEnv{
						"value": api.EnvVarValue{
							Value: pointers.String("value"),
						},
						"config": api.EnvVarValue{
							Config: pointers.String("config"),
						},
					},
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
				RestrictCallers:    []string{},
				Configs:            []string{},
				Constraints:        map[string]string{},
				Schedules:          map[string]ScheduleDefinition{},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert := require.New(t)
			d, err := NewDefinitionFromTask(test.task, test.resources)
			assert.NoError(err)
			assert.Equal(test.definition, d)
		})
	}
}

func TestDefinitionToUpdateTaskRequest(t *testing.T) {
	for _, test := range []struct {
		name       string
		definition Definition
		request    api.UpdateTaskRequest
		resources  []api.ResourceMetadata
		isBundle   bool
	}{
		{
			name: "python task",
			definition: Definition{
				Name:        "Test Task",
				Slug:        "test_task",
				Description: "A task for testing",
				Python: &PythonDefinition{
					Entrypoint: "main.py",
				},
			},
			request: api.UpdateTaskRequest{
				Name:        "Test Task",
				Slug:        "test_task",
				Description: "A task for testing",
				Configs:     &[]api.ConfigAttachment{},
				Parameters:  []api.Parameter{},
				Resources:   map[string]string{},
				Kind:        buildtypes.TaskKindPython,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.py",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				Timeout: 0,
				Env:     api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
			},
		},
		{
			name:     "python task from bundle",
			isBundle: true,
			definition: Definition{
				Name:        "Test Task",
				Slug:        "test_task",
				Description: "A task for testing",
				Python: &PythonDefinition{
					Entrypoint: "main.py",
				},
				buildConfig: buildtypes.BuildConfig{
					"entrypointFunc": "my_func",
					"entrypoint":     "main.py",
				},
			},
			request: api.UpdateTaskRequest{
				Name:    "Test Task",
				Slug:    "test_task",
				Command: []string{"python"},
				Arguments: []string{
					"/airplane/.airplane/shim.py",
					"/airplane/main.py",
					"my_func",
					"{{JSON.stringify(params)}}",
				},
				Description: "A task for testing",
				Configs:     &[]api.ConfigAttachment{},
				Parameters:  []api.Parameter{},
				Resources:   map[string]string{},
				Kind:        buildtypes.TaskKindPython,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.py",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				Timeout: 0,
				Env:     api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
			},
		},
		{
			name: "node task",
			definition: Definition{
				Name: "Node Task",
				Slug: "node_task",
				Node: &NodeDefinition{
					Entrypoint:  "main.ts",
					NodeVersion: "14",
				},
			},
			request: api.UpdateTaskRequest{
				Name:       "Node Task",
				Slug:       "node_task",
				Parameters: []api.Parameter{},
				Resources:  map[string]string{},
				Configs:    &[]api.ConfigAttachment{},
				Kind:       buildtypes.TaskKindNode,
				KindOptions: buildtypes.KindOptions{
					"entrypoint":  "main.ts",
					"nodeVersion": "14",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				Timeout: 0,
				Env:     api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
			},
		},
		{
			name:     "node task from bundle",
			isBundle: true,
			definition: Definition{
				Name: "Node Task",
				Slug: "node_task",
				Node: &NodeDefinition{
					Entrypoint:  "main.ts",
					NodeVersion: "14",
				},
				buildConfig: buildtypes.BuildConfig{
					"entrypointFunc": "default",
					"entrypoint":     "main.ts",
				},
			},
			request: api.UpdateTaskRequest{
				Name:    "Node Task",
				Slug:    "node_task",
				Command: []string{"node"},
				Arguments: []string{
					"/airplane/.airplane/dist/universal-shim.js",
					"/airplane/.airplane/main.js",
					"default",
					"{{JSON.stringify(params)}}",
				},
				Parameters: []api.Parameter{},
				Resources:  map[string]string{},
				Configs:    &[]api.ConfigAttachment{},
				Kind:       buildtypes.TaskKindNode,
				KindOptions: buildtypes.KindOptions{
					"entrypoint":  "main.ts",
					"nodeVersion": "14",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				Timeout: 0,
				Env:     api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
			},
		},
		{
			name: "shell task",
			definition: Definition{
				Name: "Shell Task",
				Slug: "shell_task",
				Shell: &ShellDefinition{
					Entrypoint: "main.sh",
				},
			},
			request: api.UpdateTaskRequest{
				Name:       "Shell Task",
				Slug:       "shell_task",
				Parameters: []api.Parameter{},
				Resources:  map[string]string{},
				Configs:    &[]api.ConfigAttachment{},
				Kind:       buildtypes.TaskKindShell,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.sh",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				Timeout: 0,
				Env:     api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
			},
		},
		{
			name:     "shell task from bundle",
			isBundle: true,
			definition: Definition{
				Name: "Shell Task",
				Slug: "shell_task",
				Shell: &ShellDefinition{
					Entrypoint: "main.sh",
				},
				Parameters: []ParameterDefinition{
					{Slug: "one", Name: "One", Type: "shorttext"},
					{Slug: "two", Name: "Two", Type: "boolean"},
				},
				buildConfig: buildtypes.BuildConfig{
					"entrypoint": "main.sh",
				},
			},
			request: api.UpdateTaskRequest{
				Name:    "Shell Task",
				Slug:    "shell_task",
				Command: []string{"bash"},
				Arguments: []string{
					".airplane/shim.sh",
					"./main.sh",
					"one={{params.one}}",
					"two={{params.two}}",
				},
				Parameters: []api.Parameter{
					{Slug: "one", Name: "One", Type: "string"},
					{Slug: "two", Name: "Two", Type: "boolean"},
				},
				Resources: map[string]string{},
				Configs:   &[]api.ConfigAttachment{},
				Kind:      buildtypes.TaskKindShell,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.sh",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				InterpolationMode: pointers.String("jst"),
				Timeout:           0,
				Env:               api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
			},
		},
		{
			name: "image task",
			definition: Definition{
				Name: "Image Task",
				Slug: "image_task",
				Image: &ImageDefinition{
					Image:      "ubuntu:latest",
					Entrypoint: "bash",
					Command:    `-c 'echo "foobar"'`,
				},
			},
			request: api.UpdateTaskRequest{
				Name:       "Image Task",
				Slug:       "image_task",
				Parameters: []api.Parameter{},
				Resources:  map[string]string{},
				Configs:    &[]api.ConfigAttachment{},
				Command:    []string{"bash"},
				Arguments:  []string{"-c", `echo "foobar"`},
				Kind:       buildtypes.TaskKindImage,
				Image:      pointers.String("ubuntu:latest"),
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				Timeout: 0,
				Env:     api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
				KindOptions: buildtypes.KindOptions{},
			},
		},
		{
			name: "rest task",
			definition: Definition{
				Name: "REST Task",
				Slug: "rest_task",
				REST: &RESTDefinition{
					Resource: "rest",
					Method:   "POST",
					Path:     "/post",
					BodyType: "json",
					Body:     `{"foo": "bar"}`,
				},
			},
			request: api.UpdateTaskRequest{
				Name:       "REST Task",
				Slug:       "rest_task",
				Parameters: []api.Parameter{},
				Configs:    &[]api.ConfigAttachment{},
				Kind:       buildtypes.TaskKindREST,
				KindOptions: buildtypes.KindOptions{
					"method":        "POST",
					"path":          "/post",
					"urlParams":     map[string]interface{}{},
					"headers":       map[string]interface{}{},
					"bodyType":      "json",
					"body":          `{"foo": "bar"}`,
					"formData":      map[string]interface{}{},
					"retryFailures": nil,
				},
				Resources: map[string]string{
					"rest": "rest_id",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				Timeout: 0,
				Env:     api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
			},
			resources: []api.ResourceMetadata{
				{
					ID: "rest_id",
					DefaultEnvResource: &api.Resource{
						Name: "rest",
					},
				},
			},
		},
		{
			name: "test update execute rules",
			definition: Definition{
				Name:        "Test Task",
				Slug:        "test_task",
				Description: "A task for testing",
				Python: &PythonDefinition{
					Entrypoint: "main.py",
				},
				RequireRequests:    true,
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(false)},
			},
			request: api.UpdateTaskRequest{
				Name:        "Test Task",
				Slug:        "test_task",
				Parameters:  []api.Parameter{},
				Resources:   map[string]string{},
				Configs:     &[]api.ConfigAttachment{},
				Description: "A task for testing",
				Kind:        buildtypes.TaskKindPython,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.py",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(true),
					RequireRequests:     pointers.Bool(true),
					RestrictCallers:     []string{},
				},
				Timeout: 0,
				Env:     api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
			},
		},
		{
			name: "test update default execute rules",
			definition: Definition{
				Name:        "Test Task",
				Slug:        "test_task",
				Description: "A task for testing",
				Python: &PythonDefinition{
					Entrypoint: "main.py",
				},
				RequireRequests:    false,
				AllowSelfApprovals: DefaultTrueDefinition{nil},
			},
			request: api.UpdateTaskRequest{
				Name:        "Test Task",
				Slug:        "test_task",
				Parameters:  []api.Parameter{},
				Resources:   map[string]string{},
				Configs:     &[]api.ConfigAttachment{},
				Description: "A task for testing",
				Kind:        buildtypes.TaskKindPython,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.py",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				Timeout: 0,
				Env:     api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
			},
		},
		{
			name: "check parameters",
			definition: Definition{
				Name: "Test Task",
				Slug: "test_task",
				Parameters: []ParameterDefinition{
					{
						Name:        "Required boolean",
						Slug:        "required_boolean",
						Type:        "boolean",
						Description: "A required boolean.",
					},
					{
						Name:    "Short text",
						Slug:    "short_text",
						Type:    "shorttext",
						Default: "foobar",
					},
					{
						Name: "SQL",
						Slug: "sql",
						Type: "sql",
					},
					{
						Name:     "Optional long text",
						Slug:     "optional_long_text",
						Type:     "longtext",
						Required: DefaultTrueDefinition{pointers.Bool(false)},
					},
					{
						Name: "Options",
						Slug: "options",
						Type: "shorttext",
						Options: []OptionDefinition{
							{
								Label: "one",
								Value: 1,
							},
							{
								Label: "two",
								Value: 2,
							},
							{
								Label: "three",
								Value: 3,
							},
							{
								Label:  "config",
								Config: pointers.String("config_name"),
							},
						},
					},
					{
						Name:  "Regex",
						Slug:  "regex",
						Type:  "shorttext",
						Regex: "foo.*",
					},
					{
						Name: "Config var",
						Slug: "config_var",
						Type: "configvar",
						Default: map[string]interface{}{
							"config": "API_KEY",
						},
						Options: []OptionDefinition{
							{
								Label:  "API key",
								Config: pointers.String("API_KEY"),
							},
							{
								Label:  "Other API key",
								Config: pointers.String("OTHER_API_KEY"),
							},
						},
					},
					{
						// With string values
						Name:    "Config var",
						Slug:    "config_var2",
						Type:    "configvar",
						Default: "API_KEY",
						Options: []OptionDefinition{
							{
								Label: "API key",
								Value: "API_KEY",
							},
							{
								Label: "Other API key",
								Value: "OTHER_API_KEY",
							},
						},
					},
				},
				Python: &PythonDefinition{
					Entrypoint: "main.py",
				},
			},
			request: api.UpdateTaskRequest{
				Name: "Test Task",
				Slug: "test_task",
				Parameters: []api.Parameter{
					{
						Name: "Required boolean",
						Slug: "required_boolean",
						Type: api.TypeBoolean,
						Desc: "A required boolean.",
					},
					{
						Name:    "Short text",
						Slug:    "short_text",
						Type:    api.TypeString,
						Default: "foobar",
					},
					{
						Name:      "SQL",
						Slug:      "sql",
						Type:      api.TypeString,
						Component: api.ComponentEditorSQL,
					},
					{
						Name:      "Optional long text",
						Slug:      "optional_long_text",
						Type:      api.TypeString,
						Component: api.ComponentTextarea,
						Constraints: api.Constraints{
							Optional: true,
						},
					},
					{
						Name: "Options",
						Slug: "options",
						Type: api.TypeString,
						Constraints: api.Constraints{
							Options: []api.ConstraintOption{
								{
									Label: "one",
									Value: 1,
								},
								{
									Label: "two",
									Value: 2,
								},
								{
									Label: "three",
									Value: 3,
								},
								{
									Label: "config",
									Value: map[string]interface{}{
										"__airplaneType": "configvar",
										"name":           "config_name",
									},
								},
							},
						},
					},
					{
						Name: "Regex",
						Slug: "regex",
						Type: api.TypeString,
						Constraints: api.Constraints{
							Regex: "foo.*",
						},
					},
					{
						Name: "Config var",
						Slug: "config_var",
						Type: api.TypeConfigVar,
						Default: map[string]interface{}{
							"__airplaneType": "configvar",
							"name":           "API_KEY",
						},
						Constraints: api.Constraints{
							Options: []api.ConstraintOption{
								{
									Label: "API key",
									Value: map[string]interface{}{
										"__airplaneType": "configvar",
										"name":           "API_KEY",
									},
								},
								{
									Label: "Other API key",
									Value: map[string]interface{}{
										"__airplaneType": "configvar",
										"name":           "OTHER_API_KEY",
									},
								},
							},
						},
					},
					{
						Name: "Config var",
						Slug: "config_var2",
						Type: api.TypeConfigVar,
						Default: map[string]interface{}{
							"__airplaneType": "configvar",
							"name":           "API_KEY",
						},
						Constraints: api.Constraints{
							Options: []api.ConstraintOption{
								{
									Label: "API key",
									Value: map[string]interface{}{
										"__airplaneType": "configvar",
										"name":           "API_KEY",
									},
								},
								{
									Label: "Other API key",
									Value: map[string]interface{}{
										"__airplaneType": "configvar",
										"name":           "OTHER_API_KEY",
									},
								},
							},
						},
					},
				},
				Resources: map[string]string{},
				Configs:   &[]api.ConfigAttachment{},
				Kind:      buildtypes.TaskKindPython,
				KindOptions: buildtypes.KindOptions{
					"entrypoint": "main.py",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				Timeout: 0,
				Env:     api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
			},
		},
		{
			name: "check configs",
			definition: Definition{
				Name: "REST Task",
				Slug: "rest_task",
				REST: &RESTDefinition{
					Resource: "rest",
					Method:   "POST",
					Path:     "/post",
					BodyType: "json",
					Body:     `{"foo": "bar"}`,
					Configs:  []string{"CONFIG_VARIABLE_1", "CONFIG_VARIABLE_2"},
				},
			},
			request: api.UpdateTaskRequest{
				Name:       "REST Task",
				Slug:       "rest_task",
				Parameters: []api.Parameter{},
				Configs: &[]api.ConfigAttachment{
					{
						NameTag: "CONFIG_VARIABLE_1",
					},
					{
						NameTag: "CONFIG_VARIABLE_2",
					},
				},
				Kind: buildtypes.TaskKindREST,
				KindOptions: buildtypes.KindOptions{
					"method":        "POST",
					"path":          "/post",
					"urlParams":     map[string]interface{}{},
					"headers":       map[string]interface{}{},
					"bodyType":      "json",
					"body":          `{"foo": "bar"}`,
					"formData":      map[string]interface{}{},
					"retryFailures": nil,
				},
				Resources: map[string]string{
					"rest": "rest_id",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				Timeout: 0,
				Env:     api.TaskEnv{},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
			},
			resources: []api.ResourceMetadata{
				{
					ID: "rest_id",
					DefaultEnvResource: &api.Resource{
						Name: "rest",
					},
				},
			},
		},
		{
			name: "simple resources",
			definition: Definition{
				Name: "Image Task",
				Slug: "image_task",
				Resources: map[string]string{
					"db": "local_db",
				},
				Image: &ImageDefinition{
					Image:      "ubuntu:latest",
					Entrypoint: "bash",
					Command:    `-c 'echo "foobar"'`,
					EnvVars: api.TaskEnv{
						"value": api.EnvVarValue{
							Value: pointers.String("value"),
						},
						"config": api.EnvVarValue{
							Config: pointers.String("config"),
						},
					},
				},
				AllowSelfApprovals: DefaultTrueDefinition{pointers.Bool(true)},
			},
			request: api.UpdateTaskRequest{
				Name:       "Image Task",
				Slug:       "image_task",
				Parameters: []api.Parameter{},
				Configs:    &[]api.ConfigAttachment{},
				Command:    []string{"bash"},
				Arguments:  []string{"-c", `echo "foobar"`},
				Kind:       buildtypes.TaskKindImage,
				Image:      pointers.String("ubuntu:latest"),
				Env: api.TaskEnv{
					"value": api.EnvVarValue{
						Value: pointers.String("value"),
					},
					"config": api.EnvVarValue{
						Config: pointers.String("config"),
					},
				},
				Resources: map[string]string{
					"db": "res20220613localdb",
				},
				ExecuteRules: api.UpdateExecuteRulesRequest{
					DisallowSelfApprove: pointers.Bool(false),
					RequireRequests:     pointers.Bool(false),
					RestrictCallers:     []string{},
				},
				Constraints: api.RunConstraints{
					Labels: []api.AgentLabel{},
				},
				KindOptions: buildtypes.KindOptions{},
				Timeout:     0,
			},
			resources: []api.ResourceMetadata{
				{
					ID:   "res20220613localdb",
					Slug: "local_db",
					DefaultEnvResource: &api.Resource{
						Name: "Local DB",
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert := require.New(t)
			task, err := test.definition.GetTask(GetTaskOpts{
				AvailableResources: test.resources,
				Bundle:             test.isBundle,
			})
			assert.NoError(err)
			assert.Equal(test.request, task.AsUpdateTaskRequest())
		})
	}
}

func TestDefinitionGetSchedules(t *testing.T) {
	require := require.New(t)

	def := Definition{
		Schedules: map[string]ScheduleDefinition{
			"foo": {
				Name:        "Foo",
				Description: "Does foo",
				CronExpr:    "0 0 * * *",
				ParamValues: map[string]interface{}{
					"param_one": 5.5,
				},
			},
		},
	}

	schedules := def.GetSchedules()
	require.Len(schedules, 1)
	require.Contains(schedules, "foo")

	scheduleDef := schedules["foo"]
	require.Equal(scheduleDef.Name, "Foo")
	require.Equal(scheduleDef.Description, "Does foo")
	require.Equal(scheduleDef.CronExpr, "0 0 * * *")
	require.Len(scheduleDef.ParamValues, 1)
	require.Contains(scheduleDef.ParamValues, "param_one")
	require.Equal(scheduleDef.ParamValues["param_one"], 5.5)
}

func TestDefinitionGetConfigAttachments(t *testing.T) {
	require := require.New(t)

	def := Definition{
		SQL: &SQLDefinition{
			Configs: []string{"CONFIG-VAR-NESTED"},
		},
	}

	ca, err := def.GetConfigAttachments()
	require.NoError(err)
	require.Equal(ca, []api.ConfigAttachment{
		{
			NameTag: "CONFIG-VAR-NESTED",
		},
	})

	def = Definition{
		SQL:     &SQLDefinition{},
		Configs: []string{"CONFIG-VAR-TOP"},
	}
	ca, err = def.GetConfigAttachments()
	require.NoError(err)
	require.Equal(ca, []api.ConfigAttachment{
		{
			NameTag: "CONFIG-VAR-TOP",
		},
	})
}
