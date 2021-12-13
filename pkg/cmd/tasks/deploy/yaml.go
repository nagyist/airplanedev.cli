package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/build"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/taskdir"
	libBuild "github.com/airplanedev/lib/pkg/build"
	"github.com/pkg/errors"
)

// DeployFromYaml deploys from a yaml file.
func deployFromYaml(ctx context.Context, cfg config) (rErr error) {
	client := cfg.client
	props := taskDeployedProps{
		from: "yaml",
	}
	start := time.Now()
	defer func() {
		analytics.Track(cfg.root, "Task Deployed", map[string]interface{}{
			"from":             props.from,
			"kind":             props.kind,
			"task_id":          props.taskID,
			"task_slug":        props.taskSlug,
			"task_name":        props.taskName,
			"build_id":         props.buildID,
			"errored":          rErr != nil,
			"duration_seconds": time.Since(start).Seconds(),
		})
	}()

	dir, err := taskdir.Open(cfg.paths[0], false)
	if err != nil {
		return err
	}
	defer dir.Close()

	def, err := dir.ReadDefinition()
	if err != nil {
		return err
	}

	def, err = def.Validate()
	if err != nil {
		return err
	}
	props.taskSlug = def.Slug

	err = ensureConfigVarsExist(ctx, client, &def)
	if err != nil {
		return err
	}

	kind, kindOptions, err := def.GetKindAndOptions()
	if err != nil {
		return err
	}
	props.kind = kind

	// Remap resources from ref -> name to ref -> id.
	resp, err := client.ListResources(ctx)
	if err != nil {
		return errors.Wrap(err, "fetching resources")
	}
	resourcesByName := map[string]api.Resource{}
	for _, resource := range resp.Resources {
		resourcesByName[resource.Name] = resource
	}
	resources := map[string]string{}
	for ref, name := range def.Resources {
		if res, ok := resourcesByName[name]; ok {
			resources[ref] = res.ID
		} else {
			return errors.Errorf("unknown resource: %s", name)
		}
	}

	var image *string
	var command []string
	if def.Image != nil {
		image = &def.Image.Image
		command = def.Image.Command
	}

	task, err := client.GetTask(ctx, def.Slug)
	if _, ok := err.(*api.TaskMissingError); ok {
		// A task with this slug does not exist, so we should create one.
		logger.Log("Creating task...")
		_, err = client.CreateTask(ctx, api.CreateTaskRequest{
			Slug:             def.Slug,
			Name:             def.Name,
			Description:      def.Description,
			Image:            image,
			Command:          command,
			Arguments:        def.Arguments,
			Parameters:       def.Parameters,
			Constraints:      def.Constraints,
			Env:              def.Env,
			ResourceRequests: def.ResourceRequests,
			Resources:        resources,
			Kind:             kind,
			KindOptions:      kindOptions,
			Repo:             def.Repo,
			Timeout:          def.Timeout,
		})
		if err != nil {
			return errors.Wrapf(err, "creating task %s", def.Slug)
		}

		task, err = client.GetTask(ctx, def.Slug)
		if err != nil {
			return errors.Wrap(err, "fetching created task")
		}
	} else if err != nil {
		return errors.Wrap(err, "getting task")
	}
	props.taskID = task.ID
	props.taskName = task.Name

	interpolationMode := task.InterpolationMode
	if interpolationMode != "jst" {
		if cfg.upgradeInterpolation {
			logger.Warning(`Your task is being migrated from handlebars to Airplane JS Templates.
More information: https://apn.sh/jst-upgrade`)
			interpolationMode = "jst"
			if err := def.UpgradeJST(); err != nil {
				return err
			}
		} else {
			logger.Warning(`Tasks are migrating from handlebars to Airplane JS Templates! Your task has not
been automatically upgraded because of potential backwards-compatibility issues
(e.g. uploads will be passed to your task as an object with a url field instead
of just the url string).

To upgrade, update your task to support the new format and re-deploy with --jst.
More information: https://apn.sh/jst-upgrade`)
		}
	}

	if ok, err := libBuild.NeedsBuilding(kind); err != nil {
		return err
	} else if ok {
		var bc build.BuildCreator
		if cfg.local {
			bc = build.NewLocalBuildCreator()
		} else {
			bc = build.NewRemoteBuildCreator()
		}
		resp, err := bc.CreateBuild(ctx, build.Request{
			Client: client,
			Root:   dir.DefinitionRootPath(),
			Def:    &def,
			TaskID: task.ID,
		})
		props.buildLocal = cfg.local
		if resp != nil {
			props.buildID = resp.BuildID
		}
		if err != nil {
			return err
		}
		image = &resp.ImageURL
	}

	_, err = client.UpdateTask(ctx, api.UpdateTaskRequest{
		Slug:                       def.Slug,
		Name:                       def.Name,
		Description:                def.Description,
		Image:                      image,
		Command:                    command,
		Arguments:                  def.Arguments,
		Parameters:                 def.Parameters,
		Constraints:                def.Constraints,
		Env:                        def.Env,
		ResourceRequests:           def.ResourceRequests,
		Resources:                  resources,
		Kind:                       kind,
		KindOptions:                kindOptions,
		Repo:                       def.Repo,
		RequireExplicitPermissions: task.RequireExplicitPermissions,
		Permissions:                task.Permissions,
		Timeout:                    def.Timeout,
		InterpolationMode:          interpolationMode,
	})
	if err != nil {
		return errors.Wrapf(err, "updating task %s", def.Slug)
	}

	// Leave off `-- [parameters]` for simplicity - user will get prompted.
	cmd := fmt.Sprintf("airplane exec %s", def.Slug)
	logger.Suggest(
		"⚡ To execute the task from the CLI:",
		cmd,
	)

	logger.Suggest(
		"⚡ To execute the task from the UI:",
		client.TaskURL(def.Slug),
	)
	return nil
}
