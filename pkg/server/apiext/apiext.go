package apiext

import (
	"context"
	"fmt"
	"net/http"
	"time"

	libapi "github.com/airplanedev/cli/pkg/api"
	api "github.com/airplanedev/cli/pkg/api/cliapi"
	libhttp "github.com/airplanedev/cli/pkg/api/http"
	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/parameters"
	"github.com/airplanedev/cli/pkg/print"
	resources "github.com/airplanedev/cli/pkg/resources/cliresources"
	"github.com/airplanedev/cli/pkg/server/handlers"
	"github.com/airplanedev/cli/pkg/server/outputs"
	"github.com/airplanedev/cli/pkg/server/state"
	serverutils "github.com/airplanedev/cli/pkg/server/utils"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

// AttachExternalAPIRoutes attaches a minimal subset of the actual Airplane API endpoints that are necessary to locally develop
// a task. For example, a workflow task might call airplane.execute, which would normally make a request to the
// /v0/tasks/execute endpoint in production, but instead we have our own implementation below.
func AttachExternalAPIRoutes(r *mux.Router, state *state.State) {
	const basePath = "/v0/"
	r = r.NewRoute().PathPrefix(basePath).Subrouter()

	r.Handle("/tasks/execute", handlers.WithBody(state, ExecuteTaskHandler)).Methods("POST", "OPTIONS")
	r.Handle("/tasks/getMetadata", handlers.New(state, GetTaskMetadataHandler)).Methods("GET", "OPTIONS")
	r.Handle("/tasks/getTaskReviewers", handlers.New(state, GetTaskReviewersHandler)).Methods("GET", "OPTIONS")

	r.Handle("/runbooks/execute", handlers.WithBody(state, ExecuteRunbookHandler)).Methods("POST", "OPTIONS")
	r.Handle("/runbooks/get", handlers.New(state, GetRunbooksHandler)).Methods("GET", "OPTIONS")

	r.Handle("/entities/search", handlers.New(state, SearchEntitiesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runners/createScaleSignal", handlers.WithBody(state, CreateScaleSignalHandler)).Methods("POST", "OPTIONS")

	r.Handle("/runs/getOutputs", handlers.New(state, outputs.GetOutputsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/runs/get", handlers.New(state, GetRunHandler)).Methods("GET", "OPTIONS")

	r.Handle("/resources/list", handlers.New(state, ListResourcesHandler)).Methods("GET", "OPTIONS")
	r.Handle("/resources/listMetadata", handlers.New(state, ListResourceMetadataHandler)).Methods("GET", "OPTIONS")

	r.Handle("/views/get", handlers.New(state, GetViewHandler)).Methods("GET", "OPTIONS")

	r.Handle("/displays/create", handlers.WithBody(state, CreateDisplayHandler)).Methods("POST", "OPTIONS")

	r.Handle("/prompts/get", handlers.New(state, GetPromptHandler)).Methods("GET", "OPTIONS")
	r.Handle("/prompts/create", handlers.WithBody(state, CreatePromptHandler)).Methods("POST", "OPTIONS")

	// Run sleeps
	r.Handle("/sleeps/create", handlers.WithBody(state, CreateSleepHandler)).Methods("POST", "OPTIONS")
	r.Handle("/sleeps/list", handlers.New(state, ListSleepsHandler)).Methods("GET", "OPTIONS")
	r.Handle("/sleeps/get", handlers.New(state, GetSleepHandler)).Methods("GET", "OPTIONS")

	r.Handle("/permissions/get", handlers.New(state, GetPermissionsHandler)).Methods("GET", "OPTIONS")

	r.Handle("/hosts/web", handlers.New(state, WebHostHandler)).Methods("GET", "OPTIONS")

	r.Handle("/uploads/create", handlers.WithBody(state, CreateUploadHandler)).Methods("POST", "OPTIONS")
}

func getRunIDFromToken(r *http.Request) (string, error) {
	if token := r.Header.Get("X-Airplane-Token"); token != "" {
		claims, err := dev.ParseInsecureAirplaneToken(token)
		if err != nil {
			return "", err
		}
		return claims.RunID, nil
	}
	return "", nil
}

// GetRunHandler handles requests to the /v0/runs/get endpoint
func GetRunHandler(ctx context.Context, state *state.State, r *http.Request) (dev.LocalRun, error) {
	runID := r.URL.Query().Get("id")
	run, err := state.GetRun(ctx, runID)
	if err != nil {
		return dev.LocalRun{}, err
	}

	if run.Remote {
		resp, err := state.RemoteClient.GetRun(ctx, runID)
		if err != nil {
			return dev.LocalRun{}, errors.Wrap(err, "getting remote run")
		}
		return dev.FromRemoteRun(resp.Run), nil
	}

	return run, nil
}

// GetViewHandler handles requests to the /v0/views/get endpoint. It generates a deterministic view ID for each view
// found locally, and its primary purpose is to ensure that the view discoverer does not error.
// If a view is not local, it tries the fallback environment, so that local views
// can route correctly to the correct URL.
func GetViewHandler(ctx context.Context, state *state.State, r *http.Request) (libapi.View, error) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		return libapi.View{}, libhttp.NewErrBadRequest("slug cannot be empty")
	}

	_, ok := state.ViewConfigs.Get(slug)
	// Not local, we try using the fallback env first, but we default to returning a dummy view if it's not found.
	if !ok {
		if state.InitialRemoteEnvSlug != nil {
			// TODO: should probably pass env into GetView
			resp, err := state.RemoteClient.GetView(ctx, libapi.GetViewRequest{
				Slug: slug,
			})
			if err != nil {
				logger.Debug("Received error %s from remote view, falling back to default", err)
			} else {
				return resp, nil
			}
		}
	}

	return libapi.View{
		ID:      fmt.Sprintf("vew-%s", slug),
		Slug:    slug,
		IsLocal: true,
	}, nil
}

type CreateDisplayRequest struct {
	Display libapi.Display `json:"display"`
}

type CreateDisplayResponse struct {
	ID string `json:"id"`
}

func CreateDisplayHandler(ctx context.Context, state *state.State, r *http.Request, req CreateDisplayRequest) (CreateDisplayResponse, error) {
	runID, err := getRunIDFromToken(r)
	if err != nil {
		return CreateDisplayResponse{}, err
	}
	if runID == "" {
		return CreateDisplayResponse{}, libhttp.NewErrBadRequest("this endpoint can only be called from the task runtime")
	}

	now := time.Now()
	display := libapi.Display{
		ID:        utils.GenerateID("dsp"),
		RunID:     runID,
		Kind:      req.Display.Kind,
		CreatedAt: now,
		UpdatedAt: now,
	}
	switch req.Display.Kind {
	case "markdown":
		display.Content = req.Display.Content

		maxMarkdownLength := 100000
		if len(display.Content) > maxMarkdownLength {
			return CreateDisplayResponse{}, libhttp.NewErrBadRequest("content too long: expected at most %d characters, got %d", maxMarkdownLength, len(display.Content))
		}
	case "table":
		display.Rows = req.Display.Rows
		display.Columns = req.Display.Columns

		maxRows := 10000
		if len(display.Rows) > maxRows {
			return CreateDisplayResponse{}, libhttp.NewErrBadRequest("too many table rows: expected at most %d, got %d", maxRows, len(display.Rows))
		}

		maxColumns := 100
		if len(display.Columns) > maxColumns {
			return CreateDisplayResponse{}, libhttp.NewErrBadRequest("too many table columns: expected at most %d, got %d", maxColumns, len(display.Columns))
		}
	case "json":
		display.Value = req.Display.Value
	case "file":
		display.UploadID = req.Display.UploadID
	}

	run, err := state.UpdateRun(runID, func(run *dev.LocalRun) error {
		run.Displays = append(run.Displays, display)
		return nil
	})
	if err != nil {
		return CreateDisplayResponse{}, err
	}

	content := fmt.Sprintf("[kind=%s]\n\n%s", display.Kind, display.Content)
	prefix := "[" + logger.Gray(run.TaskID+" display") + "] "
	print.BoxPrintWithPrefix(content, prefix)

	return CreateDisplayResponse{
		ID: display.ID,
	}, nil
}

type PromptResponse struct {
	ID string `json:"id"`
}

func CreatePromptHandler(ctx context.Context, state *state.State, r *http.Request, req libapi.Prompt) (PromptResponse, error) {
	runID, err := getRunIDFromToken(r)
	if err != nil {
		return PromptResponse{}, err
	}
	if runID == "" {
		return PromptResponse{}, libhttp.NewErrBadRequest("this endpoint can only be called from the task runtime")
	}

	if req.Values == nil {
		req.Values = map[string]interface{}{}
	}

	req.Values = parameters.ApplyDefaults(req.Schema, req.Values)
	// NOTE: we don't standardize param values here since currently parameter values
	// are represented as map[string]interface{} whereas in Airport, they are a Values type.
	// The Airport Values type has a custom JSON marshaler that converts upload objects to
	// upload IDs. StandardizeParamValues converts upload IDs to objects which would differ
	// in behavior when returning the prompt values.

	reviewersWithDefaults := req.Reviewers
	if reviewersWithDefaults == nil {
		reviewersWithDefaults = &libapi.PromptReviewers{}
	}
	if reviewersWithDefaults.AllowSelfApprovals == nil {
		// Default self approvals to true
		reviewersWithDefaults.AllowSelfApprovals = pointers.Bool(true)
	}
	if reviewersWithDefaults.Groups == nil {
		reviewersWithDefaults.Groups = []string{}
	}
	if reviewersWithDefaults.Users == nil {
		reviewersWithDefaults.Users = []string{}
	}

	prompt := libapi.Prompt{
		ID:          utils.GenerateID("pmt"),
		RunID:       runID,
		Schema:      req.Schema,
		Values:      req.Values,
		Reviewers:   reviewersWithDefaults,
		ConfirmText: req.ConfirmText,
		CancelText:  req.CancelText,
		CreatedAt:   time.Now(),
		Description: req.Description,
	}

	if _, err := state.UpdateRun(runID, func(run *dev.LocalRun) error {
		run.Prompts = append(run.Prompts, prompt)
		run.IsWaitingForUser = true
		return nil
	}); err != nil {
		return PromptResponse{}, err
	}

	return PromptResponse{ID: prompt.ID}, nil
}

type GetPromptResponse struct {
	Prompt libapi.Prompt `json:"prompt"`
}

func GetPromptHandler(ctx context.Context, state *state.State, r *http.Request) (GetPromptResponse, error) {
	promptID := r.URL.Query().Get("id")
	if promptID == "" {
		return GetPromptResponse{}, libhttp.NewErrBadRequest("expected a prompt ID")
	}
	runID, err := getRunIDFromToken(r)
	if err != nil {
		return GetPromptResponse{}, err
	}
	if runID == "" {
		return GetPromptResponse{}, libhttp.NewErrBadRequest("this endpoint can only be called from the task runtime")
	}

	run, err := state.GetRunInternal(ctx, runID)
	if err != nil {
		return GetPromptResponse{}, err
	}

	for _, p := range run.Prompts {
		if p.ID == promptID {
			// Standardize the prompt values here to convert upload IDs to objects. We do it in the
			// prompts get handler (not list) to agree with Airport.
			p.Values, err = parameters.StandardizeParamValues(ctx, state.RemoteClient, p.Schema, p.Values)
			if err != nil {
				return GetPromptResponse{}, err
			}
			return GetPromptResponse{Prompt: p}, nil
		}
	}
	return GetPromptResponse{}, libhttp.NewErrNotFound("prompt not found")
}

// ListResourcesHandler handles requests to the /v0/resources/list endpoint
func ListResourcesHandler(ctx context.Context, state *state.State, r *http.Request) (libapi.ListResourcesResponse, error) {
	resources := make([]libapi.Resource, 0, len(state.DevConfig.RawResources))
	for slug, r := range state.DevConfig.Resources {
		resources = append(resources, libapi.Resource{
			ID:                r.Resource.GetID(),
			Slug:              slug,
			Kind:              libapi.ResourceKind(r.Resource.Kind()),
			ExportResource:    r.Resource,
			CanUseResource:    true,
			CanUpdateResource: true,
		})
	}

	slices.SortFunc(resources, func(a, b libapi.Resource) bool {
		return a.Slug < b.Slug
	})

	return libapi.ListResourcesResponse{
		Resources: resources,
	}, nil
}

// ListResourceMetadataHandler handles requests to the /v0/resources/listMetadata endpoint
func ListResourceMetadataHandler(ctx context.Context, state *state.State, r *http.Request) (libapi.ListResourceMetadataResponse, error) {
	envSlug := serverutils.GetEffectiveEnvSlugFromRequest(state, r)
	resources, err := resources.ListResourceMetadata(ctx, state.RemoteClient, state.DevConfig, envSlug)
	if err != nil {
		return libapi.ListResourceMetadataResponse{}, err
	}

	return libapi.ListResourceMetadataResponse{
		Resources: resources,
	}, nil
}

func GetPermissionsHandler(ctx context.Context, state *state.State, r *http.Request) (api.GetPermissionsResponse, error) {
	taskSlug := r.URL.Query().Get("task_slug")
	actions := r.URL.Query()["actions"]
	_, hasLocalTask := state.TaskConfigs.Get(taskSlug)

	outputs := map[string]bool{}
	if hasLocalTask {
		for _, action := range actions {
			outputs[action] = true
		}
		return api.GetPermissionsResponse{
			Outputs: outputs,
		}, nil
	}

	return state.RemoteClient.GetPermissions(ctx, taskSlug, actions)
}

func WebHostHandler(ctx context.Context, state *state.State, r *http.Request) (string, error) {
	return state.RemoteClient.GetWebHost(ctx)
}

func CreateUploadHandler(
	ctx context.Context,
	state *state.State,
	r *http.Request,
	req libapi.CreateUploadRequest,
) (libapi.CreateUploadResponse, error) {
	resp, err := state.RemoteClient.CreateUpload(ctx, req)
	if err != nil {
		return libapi.CreateUploadResponse{}, errors.Wrap(err, "creating upload")
	}

	return libapi.CreateUploadResponse{
		Upload:       resp.Upload,
		ReadOnlyURL:  resp.ReadOnlyURL,
		WriteOnlyURL: resp.WriteOnlyURL,
	}, nil
}

type CreateRunnerScaleSignalRequest struct {
	SignalKey                 string  `json:"signalKey"`
	ExpirationDurationSeconds int     `json:"expirationDurationSeconds"`
	TaskSlug                  *string `json:"taskSlug"`
	TaskID                    *string `json:"taskID"`
	IsStdAPI                  *bool   `json:"isStdAPI"`
	TaskRevisionID            *string `json:"taskRevisionID"`
}

func CreateScaleSignalHandler(
	ctx context.Context,
	state *state.State,
	r *http.Request,
	req CreateRunnerScaleSignalRequest,
) (StubResponse, error) {
	return StubResponse{}, nil
}

type SearchEntitiesResponse struct {
	Results []struct{} `json:"results"`
}

func SearchEntitiesHandler(
	ctx context.Context,
	state *state.State,
	r *http.Request,
) (SearchEntitiesResponse, error) {
	return SearchEntitiesResponse{
		Results: []struct{}{},
	}, nil
}
