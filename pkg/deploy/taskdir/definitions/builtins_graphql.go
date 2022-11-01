package definitions

import (
	"context"
	"log"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/builtins"
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
			func() BuiltinTaskDef { return &GraphQLDefinition_0_3{} },
		),
	}

	for _, plugin := range plugins {
		if err := registerBuiltinTaskPlugin(plugin); err != nil {
			log.Fatal(err)
		}
	}
}

type GraphQLDefinition_0_3 struct {
	Resource      string                 `json:"resource"`
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	URLParams     map[string]interface{} `json:"urlParams,omitempty"`
	Headers       map[string]interface{} `json:"headers,omitempty"`
}

var _ taskKind_0_3 = &GraphQLDefinition_0_3{}

func (d GraphQLDefinition_0_3) getFunctionSpecification() (builtins.FunctionSpecification, error) {
	return builtins.FunctionSpecification{
		Namespace: "graphql",
		Name:      "request",
	}, nil
}

func (d GraphQLDefinition_0_3) fillInUpdateTaskRequest(ctx context.Context, client api.IAPIClient, req *api.UpdateTaskRequest) error {
	collection, err := getResourceIDsBySlugAndName(ctx, client)
	if err != nil {
		return err
	}

	if id, ok := collection.bySlug[d.Resource]; ok {
		req.Resources["api"] = id
	} else {
		return errors.Errorf("unknown resource: %s", d.Resource)
	}
	return nil
}

func (d *GraphQLDefinition_0_3) hydrateFromTask(ctx context.Context, client api.IAPIClient, t *api.Task) error {
	if resID, ok := t.Resources["api"]; ok {
		resourceSlugsByID, err := getResourceSlugsByID(ctx, client)
		if err != nil {
			return err
		}
		if slug, ok := resourceSlugsByID[resID]; ok {
			d.Resource = slug
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
	if v, ok := request["query"]; ok {
		if sv, ok := v.(string); ok {
			d.Query = sv
		} else {
			return errors.Errorf("expected string query, got %T instead", v)
		}
	}
	if v, ok := request["operationName"]; ok {
		if sv, ok := v.(string); ok {
			d.OperationName = sv
		} else {
			return errors.Errorf("expected string operationName, got %T instead", v)
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
	return nil
}

func (d GraphQLDefinition_0_3) setEntrypoint(entrypoint string) error {
	return ErrNoEntrypoint
}

func (d GraphQLDefinition_0_3) setAbsoluteEntrypoint(entrypoint string) error {
	return ErrNoEntrypoint
}

func (d GraphQLDefinition_0_3) getAbsoluteEntrypoint() (string, error) {
	return "", ErrNoEntrypoint
}

func (d GraphQLDefinition_0_3) getKindOptions() (build.KindOptions, error) {
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
	return build.KindOptions{
		"functionSpecification": map[string]interface{}{
			"namespace": "graphql",
			"name":      "request",
		},
		"request": map[string]interface{}{
			"query":         d.Query,
			"operationName": d.OperationName,
			"variables":     variables,
			"urlParams":     urlParams,
			"headers":       headers,
		},
	}, nil
}

func (d GraphQLDefinition_0_3) getEntrypoint() (string, error) {
	return "", ErrNoEntrypoint
}

func (d GraphQLDefinition_0_3) getEnv() (api.TaskEnv, error) {
	return nil, nil
}

func (d GraphQLDefinition_0_3) getConfigAttachments() []api.ConfigAttachment {
	return nil
}

func (d GraphQLDefinition_0_3) getResourceAttachments() map[string]string {
	return map[string]string{"api": d.Resource}
}

func (d GraphQLDefinition_0_3) getBuildType() (build.BuildType, build.BuildTypeVersion) {
	return build.NoneBuildType, build.BuildTypeVersionUnspecified
}

func (d GraphQLDefinition_0_3) setBuildVersion(v build.BuildTypeVersion) {
}
