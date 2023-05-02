package apiext

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	libapi "github.com/airplanedev/cli/pkg/api"
	api "github.com/airplanedev/cli/pkg/api/cliapi"
	libhttp "github.com/airplanedev/cli/pkg/api/http"
	"github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/builtins"
	"github.com/airplanedev/cli/pkg/configs"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/parameters"
	resources "github.com/airplanedev/cli/pkg/resources/cliresources"
	"github.com/airplanedev/cli/pkg/server/state"
	serverutils "github.com/airplanedev/cli/pkg/server/utils"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/airplanedev/ojson"
	"github.com/pkg/errors"
)

const (
	maxWorkflowChildRuns = 1000
)

type ExecuteTaskRequest struct {
	Slug        string            `json:"slug"`
	ParamValues api.Values        `json:"paramValues"`
	Resources   map[string]string `json:"resources"`
}

// ExecuteTaskHandler handles requests to the /v0/tasks/execute endpoint
func ExecuteTaskHandler(ctx context.Context, state *state.State, r *http.Request, req ExecuteTaskRequest) (api.RunTaskResponse, error) {
	run := *dev.NewLocalRun()
	parentID, err := getRunIDFromToken(r)
	if err != nil {
		return api.RunTaskResponse{}, err
	}
	run.ParentID = parentID

	runID := dev.GenerateRunID()
	run.ID = runID
	run.RunID = runID

	var envSlug *string
	if parentID != "" {
		// Pull env slug from the parent run.
		parentRun, err := state.GetRunInternal(ctx, parentID)
		if err != nil {
			return api.RunTaskResponse{}, err
		}
		if parentRun.FallbackEnvSlug != "" {
			envSlug = &parentRun.FallbackEnvSlug
		}

		// Check if this run has exceeded the max child runs limit.
		if parentRun.TaskRevision.Def.Runtime == types.TaskRuntimeWorkflow {
			childRuns, err := state.GetRunDescendants(ctx, parentID)
			if err != nil {
				return api.RunTaskResponse{}, err
			}
			if len(childRuns)+1 >= maxWorkflowChildRuns {
				return api.RunTaskResponse{}, libhttp.NewErrBadRequest("Parent run has exceeded the maximum limit of %d child runs", maxWorkflowChildRuns)
			}
		}
	} else {
		envSlug = serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	}

	localTaskConfig, ok := state.TaskConfigs.Get(req.Slug)
	isBuiltin := builtins.IsBuiltinTaskSlug(req.Slug)
	hasErrors := false
	if ok {
		// If it's registered locally, only execute it if it doesn't have errors.
		taskErrors, err := state.GetTaskErrors(ctx, req.Slug, pointers.ToString(envSlug))
		if err != nil {
			return api.RunTaskResponse{}, err
		}
		hasErrors = len(taskErrors.Errors) > 0
	}
	if hasErrors || (!isBuiltin && !ok) {
		if envSlug == nil {
			message := fmt.Sprintf("task with slug %q is not registered locally", req.Slug)
			if hasErrors {
				message = fmt.Sprintf("task with slug %q cannot be executed locally", req.Slug)
			}
			return api.RunTaskResponse{}, libhttp.NewErrNotFound(message)
		}

		resp, err := state.RemoteClient.RunTask(ctx, api.RunTaskRequest{
			TaskSlug:    &req.Slug,
			ParamValues: req.ParamValues,
			EnvSlug:     *envSlug,
		})
		if err != nil {
			var taskMissingError *libapi.TaskMissingError
			if errors.As(err, &taskMissingError) {
				message := fmt.Sprintf("task with slug %q is not registered locally or remotely in environment %q", req.Slug, pointers.ToString(envSlug))
				if hasErrors {
					message = fmt.Sprintf("task with slug %q cannot be executed locally and is not registered remotely in environment %q", req.Slug, pointers.ToString(envSlug))
				}
				return api.RunTaskResponse{}, libhttp.NewErrNotFound(message)
			} else {
				return api.RunTaskResponse{}, err
			}
		}

		run.Remote = true
		run.ID = resp.RunID
		run.RunID = resp.RunID
		run.EnvSlug = pointers.ToString(envSlug)
		run.FallbackEnvSlug = pointers.ToString(envSlug)
		state.AddRun(req.Slug, resp.RunID, run)
		return api.RunTaskResponse{RunID: resp.RunID}, nil
	}

	runConfig := dev.LocalRunConfig{
		ID:              runID,
		ParamValues:     req.ParamValues,
		LocalClient:     state.LocalClient,
		RemoteClient:    state.RemoteClient,
		TunnelToken:     state.DevToken,
		FallbackEnvSlug: pointers.ToString(envSlug),
		Slug:            req.Slug,
		ParentRunID:     pointers.String(parentID),
		IsBuiltin:       isBuiltin,
		AuthInfo:        state.AuthInfo,
		LogBroker:       run.LogBroker,
		WorkingDir:      state.Dir,
		StudioURL:       state.StudioURL,
		EnvVars:         state.DevConfig.EnvVars,
	}
	params := libapi.Parameters{}
	resourceAttachments := map[string]string{}
	mergedResources, err := resources.MergeRemoteResources(ctx, state.RemoteClient, state.DevConfig, envSlug)
	if err != nil {
		return api.RunTaskResponse{}, errors.Wrap(err, "merging local and remote resources")
	}
	// Builtins have a specific alias in the form of "rest", "db", etc. that is required by the builtins binary,
	// and so we need to manually generate resource attachments.
	if isBuiltin {
		// The SDK should provide us with exactly one resource for builtins.
		if len(req.Resources) != 1 {
			return api.RunTaskResponse{}, libhttp.NewErrBadRequest("unable to determine resource required by builtin, there is not exactly one resource in request: %+v", req.Resources)
		}

		// Get the only entry in the request resource map.
		var builtinAlias, resourceID string
		for builtinAlias, resourceID = range req.Resources {
		}

		var foundResource bool
		for slug, res := range mergedResources {
			if res.Resource.GetID() == resourceID {
				resourceAttachments[builtinAlias] = slug
				foundResource = true
				break
			}
		}

		if !foundResource {
			message := fmt.Sprintf("resource with id %q not found in dev config file", resourceID)
			if envSlug != nil {
				message += fmt.Sprintf("or remotely in env %q", pointers.ToString(envSlug))
			}
			if resourceID == resources.SlackID {
				message = "your team has not configured Slack. Please visit https://docs.airplane.dev/platform/slack-integration#connect-to-slack to authorize Slack to perform actions in your workspace."
			}
			return api.RunTaskResponse{}, libhttp.NewErrNotFound(message)
		}
		run.IsStdAPI = true
		stdapiReq, err := builtins.Request(req.Slug, req.ParamValues)
		if err != nil {
			return api.RunTaskResponse{}, err
		}
		run.StdAPIRequest = stdapiReq
		run.TaskName = req.Slug
		run.TaskSlug = req.Slug
		run.ParamValues = req.ParamValues
	} else {
		kind, kindOptions, err := dev.GetKindAndOptions(localTaskConfig)
		if err != nil {
			return api.RunTaskResponse{}, err
		}
		runConfig.Kind = kind
		runConfig.KindOptions = kindOptions
		runConfig.Name = localTaskConfig.Def.GetName()
		runConfig.File = localTaskConfig.TaskEntrypoint
		resourceAttachments, err = localTaskConfig.Def.GetResourceAttachments()
		if err != nil {
			return api.RunTaskResponse{}, errors.Wrap(err, "getting resource attachments")
		}
		if runConfig.TaskEnvVars, err = localTaskConfig.Def.GetEnv(); err != nil {
			return api.RunTaskResponse{}, errors.Wrap(err, "getting task env vars")
		}
		if runConfig.ConfigAttachments, err = localTaskConfig.Def.GetConfigAttachments(); err != nil {
			return api.RunTaskResponse{}, errors.Wrap(err, "getting attached configs")
		}
		params, err = localTaskConfig.Def.GetParameters()
		if err != nil {
			return api.RunTaskResponse{}, errors.Wrap(err, "getting parameters")
		}
		run.TaskID = req.Slug
		run.TaskSlug = req.Slug
		run.TaskName = localTaskConfig.Def.GetName()
		runConfig.ConfigVars, err = configs.MergeRemoteConfigs(ctx, state, envSlug)
		if err != nil {
			return api.RunTaskResponse{}, errors.Wrap(err, "merging local and remote configs")
		}
		run.TaskRevision = localTaskConfig
		paramValuesWithDefaults := parameters.ApplyDefaults(params, req.ParamValues)
		run.ParamValues = paramValuesWithDefaults
		runConfig.ParamValues, err = parameters.StandardizeParamValues(ctx, state.RemoteClient, params, paramValuesWithDefaults)
		if err != nil {
			return api.RunTaskResponse{}, err
		}
	}
	aliasToResourceMap, err := resources.GenerateAliasToResourceMap(
		ctx,
		resourceAttachments,
		mergedResources,
		envSlug,
		state.RemoteClient,
	)
	if err != nil {
		return api.RunTaskResponse{}, err
	}
	runConfig.AliasToResource = aliasToResourceMap
	run.Resources = resources.GenerateResourceAliasToID(aliasToResourceMap)
	run.CreatedAt = time.Now().UTC()

	run.Parameters = &params
	run.FallbackEnvSlug = pointers.ToString(envSlug)

	run.Status = api.RunActive
	runCtx, fn := context.WithCancel(context.Background()) // Context used for cancelling a run.
	run.CancelFn = fn
	// if the user is authenticated in CLI, use their ID
	if state.AuthInfo.User != nil {
		run.CreatorID = state.AuthInfo.User.ID
	}
	state.AddRun(req.Slug, runID, run)

	// Use a new context while executing so the handler context doesn't cancel task execution
	go func() {
		outputs, err := state.Executor.Execute(runCtx, runConfig)
		completedAt := time.Now()

		status := api.RunSucceeded
		var succeededAt *time.Time
		var failedAt *time.Time

		if err == nil {
			succeededAt = &completedAt
		} else {
			runState, _ := state.GetRunInternal(ctx, runID)
			if runState.Status == api.RunCancelled {
				status = api.RunCancelled
			} else {
				status = api.RunFailed
				failedAt = &completedAt
				// If an error output isn't already set, set it here.
				if outputs.V == nil {
					outputs = api.Outputs{
						V: ojson.NewObject().SetAndReturn("error", err.Error()),
					}
				}
			}

			// If the process was killed by a signal, the builtins binary is likely corrupt. Manually trigger a
			// re-download of the builtins binary.
			exitErr := &exec.ExitError{}
			if runState.Status != api.RunCancelled && errors.As(err, &exitErr) && exitErr.ExitCode() == -1 { // -1 is the exit code for killed processes
				if err := state.Executor.Refresh(); err != nil {
					logger.Debug("refreshing executor: %+v", err)
				}

				outputs = api.Outputs{
					V: ojson.NewObject().SetAndReturn(
						"error",
						fmt.Sprintf(
							"We detected some corrupted files in your %s directory. We've reinitialized this directory for you, please try executing the task again.",
							filepath.Join(filepath.Base(state.Dir), ".airplane"),
						),
					),
				}
			}
		}

		if _, err = state.UpdateRun(runID, func(run *dev.LocalRun) error {
			run.Outputs = outputs
			run.Status = status
			run.SucceededAt = succeededAt
			run.FailedAt = failedAt
			return nil
		}); err != nil {
			logger.Error("updating run with status: %+v", err)
		}
	}()

	return api.RunTaskResponse{RunID: runID}, nil
}

// GetTaskMetadataHandler handles requests to the /v0/tasks/getMetadata endpoint. It generates a deterministic task ID
// for each task found locally, and its primary purpose is to ensure that the task discoverer does not error.
// If a task is not local, it tries the fallback environment, so that local views
// can route correctly to the correct URL.
func GetTaskMetadataHandler(ctx context.Context, state *state.State, r *http.Request) (libapi.TaskMetadata, error) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		return libapi.TaskMetadata{}, libhttp.NewErrBadRequest("expected a slug")
	}

	_, ok := state.TaskConfigs.Get(slug)
	isBuiltin := builtins.IsBuiltinTaskSlug(slug)
	// Neither builtin nor local, we try using the fallback env first, but we
	// default to returning a dummy task if it's not found.
	if !isBuiltin && !ok {
		if state.InitialRemoteEnvSlug != nil {
			resp, err := state.RemoteClient.GetTaskMetadata(ctx, slug)
			if err != nil {
				logger.Debug("Received error %s from remote task metadata, falling back to default", err)
			} else {
				return resp, nil
			}
		}
	}
	return libapi.TaskMetadata{
		ID:      fmt.Sprintf("tsk-%s", slug),
		Slug:    slug,
		IsLocal: true,
	}, nil
}

func GetTaskReviewersHandler(ctx context.Context, state *state.State, r *http.Request) (api.GetTaskReviewersResponse, error) {
	taskSlug := r.URL.Query().Get("taskSlug")
	if taskSlug == "" {
		return api.GetTaskReviewersResponse{}, libhttp.NewErrBadRequest("expected a slug")
	}

	localTaskConfig, ok := state.TaskConfigs.Get(taskSlug)
	if ok {
		parameters, err := localTaskConfig.Def.GetParameters()
		if err != nil {
			return api.GetTaskReviewersResponse{}, err
		}
		return api.GetTaskReviewersResponse{
			Task: &libapi.Task{
				Slug:       taskSlug,
				Parameters: parameters,
				Triggers:   []libapi.Trigger{{Kind: "form"}},
			},
			Reviewers: []api.ReviewerID{},
		}, nil
	}
	if state.InitialRemoteEnvSlug == nil {
		return api.GetTaskReviewersResponse{}, libhttp.NewErrNotFound("task with slug %q is not registered locally", taskSlug)
	}

	resp, err := state.RemoteClient.GetTaskReviewers(ctx, taskSlug)
	if err != nil {
		var taskMissingError *libapi.TaskMissingError
		if errors.As(err, &taskMissingError) {
			return api.GetTaskReviewersResponse{}, libhttp.NewErrNotFound("task with slug %q is not registered locally or remotely", taskSlug)
		} else {
			return api.GetTaskReviewersResponse{}, err
		}
	}
	return resp, nil
}
