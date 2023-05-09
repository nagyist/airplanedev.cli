package rb2wf

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/stretchr/testify/require"
)

// TestConverter tests the runbook to workflow converter by running the converter and comparing
// the outputs to pre-generated fixtures. If you need to re-generate these fixtures (e.g., due to
// a legitimate code change), then run the tests with REGENERATE_FIXTURES=true in your environment.
func TestConverter(t *testing.T) {
	fixturesDir := "fixtures/end_to_end"
	regenerateFixtures := strings.ToLower(os.Getenv("REGENERATE_FIXTURES")) == "true"

	ctx := context.Background()

	client := &api.MockClient{
		Resources: []libapi.Resource{
			{
				ID:   "dbID",
				Slug: "db_slug",
				Name: "dbName",
				Kind: "mysql",
			},
			{
				ID:   "restID",
				Slug: "rest_slug",
				Name: "restName",
				Kind: "rest",
			},
			{
				ID:   "emailID",
				Slug: "email_slug",
				Name: "emailName",
				Kind: "smtp",
			},
		},
		Configs: []api.Config{
			{
				Name:     "db",
				Tag:      "dbdsn",
				Value:    "testValue",
				IsSecret: true,
			},
		},
		Runbooks: map[string]api.Runbook{
			"test_runbook": {
				ID:   "testID",
				Name: "testRunbook",
				Slug: "test_runbook_slug",
				TemplateSession: api.TemplateSession{
					ID: "testTemplateSession",
					Configs: []libapi.ConfigAttachment{
						{
							NameTag: "dbdsn",
						},
					},
				},
				Parameters: libapi.Parameters{
					{
						Slug:    "test_param_slug",
						Name:    "Test param",
						Type:    "string",
						Default: "512",
					},
					{
						Slug:    "an_integer_slug",
						Name:    "An integer",
						Type:    "integer",
						Default: 3,
					},
					{
						Slug:    "a_date_slug",
						Name:    "A date",
						Type:    "date",
						Default: "2022-11-18",
					},
					{
						Slug: "a_boolean_slug",
						Name: "A boolean",
						Type: "boolean",
					},
				},
			},
		},
		SessionBlocks: map[string][]api.SessionBlock{
			"testTemplateSession": {
				{
					ID:        "taskBlockID",
					Slug:      "task_block_slug",
					BlockKind: "task",
					BlockKindConfig: api.BlockKindConfig{
						Task: &api.BlockKindConfigTask{
							TaskID: "testTaskID",
							ParamValues: map[string]interface{}{
								"count": map[string]interface{}{
									"__airplaneType": "template",
									"raw":            "{{params.an_integer_slug}}",
								},
							},
						},
					},
				},
				{
					ID:        "noteBlockID",
					Slug:      "note_block_slug",
					BlockKind: "note",
					BlockKindConfig: api.BlockKindConfig{
						Note: &api.BlockKindConfigNote{
							Content: map[string]interface{}{
								"__airplaneType": "template",
								"raw":            "This is some content with a {{params.an_integer_slug}}",
							},
						},
					},
				},
				{
					ID:        "noteBlockGlobals",
					Slug:      "note_block_globals_slug",
					BlockKind: "note",
					BlockKindConfig: api.BlockKindConfig{
						Note: &api.BlockKindConfigNote{
							Content: map[string]interface{}{
								"__airplaneType": "template",
								"raw": "Tests that these templates get replaced: {{params.an_integer_slug}}." +
									" {{session.id}} {{session.url}} {{session.name}} {{session.creator.id}} {{session.creator.email}}" +
									"{{session.creator.name}} {{block.id}} {{block.slug}} {{runbook.id}} " +
									"{{runbook.url}} {{env.id}} {{env.slug}} {{env.name}} {{env.is_default}}",
							},
						},
					},
				},
				{
					ID:        "sqlBlockID",
					Slug:      "sql_block_slug",
					BlockKind: "stdapi",
					BlockKindConfig: api.BlockKindConfig{
						StdAPI: &api.BlockKindConfigStdAPI{
							Namespace: "sql",
							Name:      "query",
							Request: map[string]interface{}{
								"query": map[string]interface{}{
									"__airplaneType": "template",
									"raw":            "SELECT count(*) from users limit :user_count;",
								},
								"queryArgs": map[string]interface{}{
									"user_count": map[string]interface{}{
										"__airplaneType": "template",
										"raw":            "{{params.an_integer_slug}}",
									},
								},
							},
							Resources: map[string]string{
								"db_slug": "dbID",
							},
						},
					},
				},
				{
					ID:             "restBlockID",
					Slug:           "rest_block_slug",
					BlockKind:      "stdapi",
					StartCondition: "\"hello\" === params.test_param_slug",
					BlockKindConfig: api.BlockKindConfig{
						StdAPI: &api.BlockKindConfigStdAPI{
							Namespace: "rest",
							Name:      "request",
							Request: map[string]interface{}{
								"headers": map[string]interface{}{
									"header1": map[string]interface{}{
										"__airplaneType": "template",
										"raw":            "header2",
									},
								},
								"method": "GET",
								"path": map[string]interface{}{
									"__airplaneType": "template",
									"raw":            "/heathz",
								},
								"urlParams": map[string]interface{}{
									"test1": map[string]interface{}{
										"__airplaneType": "template",
										"raw":            "test2",
									},
								},
							},
							Resources: map[string]string{
								"rest_slug": "restID",
							},
						},
					},
				},
				{
					ID:        "emailBlockID",
					Slug:      "email_block_slug",
					BlockKind: "stdapi",
					BlockKindConfig: api.BlockKindConfig{
						StdAPI: &api.BlockKindConfigStdAPI{
							Namespace: "email",
							Name:      "message",
							Request: map[string]interface{}{
								"message": map[string]interface{}{
									"__airplaneType": "template",
									"raw":            "This is a message!",
								},
								"recipients": []map[string]interface{}{
									{
										"email": "bob@example.com",
										"name":  "Bob",
									},
								},
								"sender": map[string]interface{}{
									"email": map[string]interface{}{
										"__airplaneType": "template",
										"raw":            "yolken@airplane.dev",
									},
									"name": map[string]interface{}{
										"__airplaneType": "template",
										"raw":            "BHY",
									},
								},
								"subject": map[string]interface{}{
									"__airplaneType": "template",
									"raw":            "Hello",
								},
							},
							Resources: map[string]string{
								"email_slug": "emailID",
							},
						},
					},
				},
				{
					ID:        "slackBlockID",
					Slug:      "slack_block_slug",
					BlockKind: "stdapi",
					BlockKindConfig: api.BlockKindConfig{
						StdAPI: &api.BlockKindConfigStdAPI{
							Namespace: "slack",
							Name:      "message",
							Request: map[string]interface{}{
								"channelName": "notif-deploys-test",
								"message": map[string]interface{}{
									"__airplaneType": "template",
									"raw":            "Hello!",
								},
							},
						},
					},
				},
				{
					ID:        "formBlockID",
					Slug:      "form_block_slug",
					BlockKind: "form",
					BlockKindConfig: api.BlockKindConfig{
						Form: &api.BlockKindConfigForm{
							Parameters: libapi.Parameters{
								{
									Slug: "name",
									Name: "Name",
									Type: libapi.TypeString,
									Desc: "My description",
									Default: map[string]interface{}{
										"__airplaneType": "template",
										"raw":            "Hello",
									},
								},
								{
									Slug:        "optional_param",
									Name:        "optional param",
									Type:        libapi.TypeString,
									Constraints: libapi.Constraints{Optional: true},
								},
								{
									Slug: "number_param",
									Name: "number example",
									Type: libapi.TypeInteger,
								},
								{
									Slug: "float_param",
									Name: "float example",
									Type: libapi.TypeFloat,
								},
								{
									Slug: "bool_example",
									Name: "bool example",
									Type: libapi.TypeBoolean,
								},
								{
									Slug:        "option_dropdown",
									Type:        libapi.TypeString,
									Constraints: libapi.Constraints{Options: []libapi.ConstraintOption{{Label: "label1", Value: "value1"}}},
								},
								{
									Slug:      "long_text",
									Type:      libapi.TypeString,
									Component: libapi.ComponentTextarea,
								},
								{
									Slug: "date_example",
									Type: libapi.TypeDate,
								},
								{
									Slug: "datetime_example",
									Type: libapi.TypeDatetime,
								},
								{
									Slug:      "sql_param",
									Name:      "sql example",
									Type:      libapi.TypeString,
									Component: libapi.ComponentEditorSQL,
								},
								{
									Slug:        "regex_param",
									Name:        "regex example",
									Type:        libapi.TypeString,
									Constraints: libapi.Constraints{Regex: "^airplane$"},
								},
								{
									Slug: "file_is_ignored",
									Type: libapi.TypeUpload,
								},
								{
									Slug: "config_is_ignored",
									Type: libapi.TypeConfigVar,
								},
							},
						},
					},
				},
				{
					ID:        "noteBlockID2",
					Slug:      "note_block_slug2",
					BlockKind: "note",
					// Test rewriting JSTs from start conditions:
					StartCondition: "form_block_slug.output.bool_example && env.is_default",
					BlockKindConfig: api.BlockKindConfig{
						Note: &api.BlockKindConfigNote{
							Content: map[string]interface{}{
								"__airplaneType": "template",
								// Test rewriting JSTs that reference form outputs:
								"raw": "This is some content with a form value reference: {{form_block_slug.output.date_example}}",
							},
						},
					},
				},
			},
		},
		Tasks: map[string]libapi.Task{
			"testTaskID": {
				ID:   "testTaskID",
				Slug: "test_task_slug",
				Parameters: libapi.Parameters{
					{
						Name: "count",
						Slug: "count_slug",
						Type: "integer",
					},
				},
			},
		},
	}

	tempDir, err := os.MkdirTemp("", "rb2wf")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Copy the base files over to the temp directory and install the dependencies via yarn.
	// During a normal run, this is handled in the CLI entrypoint before the converter is called.
	err = filepath.Walk("fixtures/base", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel("fixtures/base", path)
		if err != nil {
			return err
		}

		fileContents, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(filepath.Join(tempDir, relPath), fileContents, 0644)
	})
	require.NoError(t, err)

	cmd := exec.Command("yarn")
	cmd.Dir = tempDir
	_, err = cmd.CombinedOutput()
	require.NoError(t, err)

	converterObj := NewRunbookConverter(client, tempDir, "test_entrypoint.ts")

	err = converterObj.Convert(ctx, "test_runbook", "prod")
	require.NoError(t, err)

	fileContents := map[string]string{}

	err = filepath.Walk(tempDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(tempDir, path)
		if err != nil {
			return err
		}

		// Skip over node modules
		if strings.HasPrefix(relPath, "node_modules/") {
			return nil
		}
		// Skip over yarn lock
		if relPath == "yarn.lock" {
			return nil
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		fileContents[relPath] = string(contents)
		return nil
	})
	require.NoError(t, err)

	for relPath, contents := range fileContents {
		fixturePath := filepath.Join(fixturesDir, relPath)

		if regenerateFixtures {
			require.NoError(t, os.WriteFile(fixturePath, []byte(contents), 0644))
		} else {
			fixtureContents, err := os.ReadFile(fixturePath)
			require.NoError(t, err)
			require.Equal(
				t,
				string(fixtureContents),
				contents,
				"contents of %s don't match; run with REGENERATE_FIXTURES=true to regenerate fixtures if needed",
				fixturePath,
			)
		}
	}
}
