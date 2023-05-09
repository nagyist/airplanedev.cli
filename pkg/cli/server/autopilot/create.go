package autopilot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	libhttp "github.com/airplanedev/cli/pkg/cli/apiclient/http"
	"github.com/airplanedev/cli/pkg/cli/server/state"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/pkg/errors"
)

func create(ctx context.Context, s *state.State, prompt string, generateContext GenerateContext) error {
	switch generateContext.Subject {
	case Task:
		return createTask(ctx, s, prompt, generateContext)
	default:
		return libhttp.NewErrBadRequest("unsupported subject")
	}
}

func createTask(ctx context.Context, s *state.State, prompt string, generateContext GenerateContext) error {
	switch generateContext.SubjectKind {
	case SQL:
		return createSQLTask(ctx, s, prompt, generateContext.GenerateSQLContext)
	default:
		return libhttp.NewErrBadRequest("unsupported subject kind")
	}
}

func createSQLTask(ctx context.Context, s *state.State, prompt string, context *GenerateSQLContext) error {
	if context == nil {
		return libhttp.NewErrBadRequest("missing sql context")
	}

	res, err := s.RemoteClient.AutopilotComplete(ctx, api.AutopilotCompleteRequest{
		Type:   api.SQLCompletionType,
		Prompt: prompt,
		Context: &api.CompleteContext{
			CompleteSQLContext: &api.CompleteSQLContext{
				ResourceID: context.ResourceID,
			},
		},
	})
	if err != nil {
		return err
	}

	// TODO: Have our API return a file name for the entrypoint and/or task definition YAML.
	// Generate random file name
	rand := utils.RandomString(3, utils.CharsetAlphaLowercase)
	slug := fmt.Sprintf("my_sql_task_%s", rand)
	sqlEntrypoint := fmt.Sprintf("%s.sql", slug)

	// Write content to file
	if err := os.WriteFile(filepath.Join(s.Dir, sqlEntrypoint), []byte(res.Content), 0644); err != nil {
		return errors.Wrap(err, "writing sql entrypoint")
	}

	res, err = s.RemoteClient.AutopilotComplete(ctx, api.AutopilotCompleteRequest{
		Type: api.TaskYAMLCompletionType,
		Prompt: fmt.Sprintf(
			"Generate a task definition YAML with slug %s, entrypoint %s, and resource slug %s",
			slug,
			sqlEntrypoint,
			context.ResourceSlug,
		),
		Context: &api.CompleteContext{
			CompleteTaskYAMLContext: &api.CompleteTaskYAMLContext{
				Kind: buildtypes.TaskKindSQL,
			},
		},
	})
	if err != nil {
		return err
	}

	sqlTaskYamlPath := fmt.Sprintf("%s.task.yaml", slug)
	if err := os.WriteFile(filepath.Join(s.Dir, sqlTaskYamlPath), []byte(res.Content), 0644); err != nil {
		return errors.Wrap(err, "writing sql task yaml")
	}

	return nil
}
