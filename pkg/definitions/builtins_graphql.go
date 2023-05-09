package definitions

import (
	"log"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/airplanedev/cli/pkg/cli/builtins"
	"github.com/pkg/errors"
)

func init() {
	plugins := []TaskBuiltinPlugin{
		newTaskBuiltinPlugin(
			[]builtins.FunctionSpecification{
				{
					Namespace: "graphql",
					Name:      "request",
				},
			},
			"graphql",
			func() BuiltinTaskDef { return &GraphQLDefinition{} },
		),
	}

	for _, plugin := range plugins {
		if err := registerBuiltinTaskPlugin(plugin); err != nil {
			log.Fatal(err)
		}
	}
}

type GraphQLDefinition struct {
	Resource      string                 `json:"resource"`
	Operation     string                 `json:"operation"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	URLParams     map[string]interface{} `json:"urlParams,omitempty"`
	Headers       map[string]interface{} `json:"headers,omitempty"`
	RetryFailures interface{}            `json:"retryFailures,omitempty"`
}

var _ taskKind = &GraphQLDefinition{}

func (d GraphQLDefinition) getFunctionSpecification() (builtins.FunctionSpecification, error) {
	return builtins.FunctionSpecification{
		Namespace: "graphql",
		Name:      "request",
	}, nil
}

func (d GraphQLDefinition) copyToTask(task *api.Task, bc buildtypes.BuildConfig, opts GetTaskOpts) error {
	if resource := getResourceBySlug(opts.AvailableResources, d.Resource); resource != nil {
		task.Resources["api"] = resource.ID
	} else if !opts.IgnoreInvalid {
		return api.ResourceMissingError{Slug: d.Resource}
	}
	return nil
}

func (d *GraphQLDefinition) update(t api.UpdateTaskRequest, availableResources []api.ResourceMetadata) error {
	if resID, ok := t.Resources["api"]; ok {
		if resource := getResourceByID(availableResources, resID); resource != nil {
			d.Resource = resource.Slug
		}
	}
	req, ok := t.KindOptions["request"]
	if !ok {
		return errors.New("missing request from GraphQL kind options")
	}
	request, ok := req.(map[string]interface{})
	if !ok {
		return errors.Errorf("expected map request, got %T instead", req)
	}
	if v, ok := request["operation"]; ok {
		if sv, ok := v.(string); ok {
			d.Operation = sv
		} else {
			return errors.Errorf("expected string operation, got %T instead", v)
		}
	}
	if v, ok := request["variables"]; ok {
		if mv, ok := v.(map[string]interface{}); ok {
			d.Variables = mv
		} else {
			return errors.Errorf("expected map variables, got %T instead", v)
		}
	}
	if v, ok := request["urlParams"]; ok {
		if mv, ok := v.(map[string]interface{}); ok {
			d.URLParams = mv
		} else {
			return errors.Errorf("expected map urlParams, got %T instead", v)
		}
	}
	if v, ok := request["headers"]; ok {
		if mv, ok := v.(map[string]interface{}); ok {
			d.Headers = mv
		} else {
			return errors.Errorf("expected map headers, got %T instead", v)
		}
	}
	if v, ok := request["retryFailures"]; ok {
		d.RetryFailures = v
	}
	return nil
}

func (d GraphQLDefinition) setEntrypoint(entrypoint string) error {
	return ErrNoEntrypoint
}

func (d GraphQLDefinition) setAbsoluteEntrypoint(entrypoint string) error {
	return ErrNoEntrypoint
}

func (d GraphQLDefinition) getAbsoluteEntrypoint() (string, error) {
	return "", ErrNoEntrypoint
}

func (d GraphQLDefinition) getKindOptions() (buildtypes.KindOptions, error) {
	variables := d.Variables
	if variables == nil {
		variables = map[string]interface{}{}
	}
	urlParams := d.URLParams
	if urlParams == nil {
		urlParams = map[string]interface{}{}
	}
	headers := d.Headers
	if headers == nil {
		headers = map[string]interface{}{}
	}
	return buildtypes.KindOptions{
		"functionSpecification": map[string]interface{}{
			"namespace": "graphql",
			"name":      "request",
		},
		"request": map[string]interface{}{
			"operation":     d.Operation,
			"variables":     variables,
			"urlParams":     urlParams,
			"headers":       headers,
			"retryFailures": d.RetryFailures,
		},
	}, nil
}

func (d GraphQLDefinition) getEntrypoint() (string, error) {
	return "", ErrNoEntrypoint
}

func (d GraphQLDefinition) getEnv() (api.EnvVars, error) {
	return nil, nil
}
func (d GraphQLDefinition) setEnv(e api.EnvVars) error {
	return nil
}

func (d GraphQLDefinition) getConfigAttachments() []api.ConfigAttachment {
	return nil
}

func (d GraphQLDefinition) getResourceAttachments() map[string]string {
	return map[string]string{"api": d.Resource}
}

func (d GraphQLDefinition) getBuildType() (buildtypes.BuildType, buildtypes.BuildTypeVersion, buildtypes.BuildBase) {
	return buildtypes.NoneBuildType, buildtypes.BuildTypeVersionUnspecified, buildtypes.BuildBaseNone
}

func (d GraphQLDefinition) SetBuildVersionBase(v buildtypes.BuildTypeVersion, b buildtypes.BuildBase) {
}
