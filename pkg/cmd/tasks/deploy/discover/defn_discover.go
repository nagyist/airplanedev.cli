package discover

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/taskdir"
	"github.com/airplanedev/cli/pkg/taskdir/definitions"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/pkg/errors"
)

type DefnDiscoverer struct {
	Client    api.APIClient
	AssumeYes bool
	AssumeNo  bool
	Logger    logger.Logger
}

var _ TaskDiscoverer = &DefnDiscoverer{}

func (dd *DefnDiscoverer) IsAirplaneTask(ctx context.Context, file string) (slug string, err error) {
	def, err := getDef(file)
	if err != nil {
		return "", err
	}

	return def.Slug, nil
}

func (dd *DefnDiscoverer) GetTaskConfig(ctx context.Context, task api.Task, file string) (TaskConfig, error) {
	dir, err := taskdir.Open(file, true)
	if err != nil {
		return TaskConfig{}, err
	}
	defer dir.Close()

	def, err := dir.ReadDefinition_0_3()
	if err != nil {
		return TaskConfig{}, err
	}

	utr, err := def.GetUpdateTaskRequest(ctx, dd.Client, &task)
	if err != nil {
		return TaskConfig{}, err
	}

	taskFilePath := ""
	entrypoint, err := def.Entrypoint()
	if err == definitions.ErrNoEntrypoint {
		// nothing
	} else if err != nil {
		return TaskConfig{}, err
	} else {
		taskFilePath, err = filepath.Abs(entrypoint)
		if err != nil {
			return TaskConfig{}, err
		}
	}

	return TaskConfig{
		TaskRoot:     dir.DefinitionRootPath(),
		TaskFilePath: taskFilePath,
		Task:         task,
		Def:          &def,
		Kind:         utr.Kind,
		KindOptions:  utr.KindOptions,
	}, nil
}

func (dd *DefnDiscoverer) TaskConfigSource() TaskConfigSource {
	return TaskConfigSourceDefn
}

func (dd *DefnDiscoverer) HandleMissingTask(ctx context.Context, file string) (*api.Task, error) {
	def, err := getDef(file)
	if err != nil {
		return nil, err
	}
	if !utils.CanPrompt() {
		return nil, nil
	}

	question := fmt.Sprintf("Task with slug %s does not exist. Would you like to create a new task?", def.Slug)
	if ok, err := utils.ConfirmWithAssumptions(question, dd.AssumeYes, dd.AssumeNo); err != nil {
		return nil, err
	} else if !ok {
		// User answered "no", so bail here.
		return nil, nil
	}

	dd.Logger.Log("Creating task...")
	utr, err := def.GetUpdateTaskRequest(ctx, dd.Client, nil)
	if err != nil {
		return nil, err
	}

	_, err = dd.Client.CreateTask(ctx, api.CreateTaskRequest{
		Slug:             utr.Slug,
		Name:             utr.Name,
		Description:      utr.Description,
		Image:            utr.Image,
		Command:          utr.Command,
		Arguments:        utr.Arguments,
		Parameters:       utr.Parameters,
		Constraints:      utr.Constraints,
		Env:              utr.Env,
		ResourceRequests: utr.ResourceRequests,
		Resources:        utr.Resources,
		Kind:             utr.Kind,
		KindOptions:      utr.KindOptions,
		Repo:             utr.Repo,
		Timeout:          utr.Timeout,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "creating task %s", def.Slug)
	}

	task, err := dd.Client.GetTask(ctx, def.Slug)
	if err != nil {
		return nil, errors.Wrap(err, "fetching created task")
	}
	return &task, nil
}

func getDef(file string) (definitions.Definition_0_3, error) {
	dir, err := taskdir.Open(file, true)
	if err != nil {
		return definitions.Definition_0_3{}, nil
	}
	defer dir.Close()

	return dir.ReadDefinition_0_3()
}
