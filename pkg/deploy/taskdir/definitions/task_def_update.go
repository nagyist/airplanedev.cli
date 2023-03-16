package definitions

import (
	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/utils/pointers"
	"github.com/pkg/errors"
)

type UpdateOptions struct {
	// Triggers are the list of triggers attached to this task. Only schedules are managed
	// as code, so all other triggers are ignored. Schedules that do not have a slug set
	// are also ignored (this indicates they were created from the app).
	//
	// If nil, the definition's schedules are left as-is.
	Triggers []api.Trigger
	// AvailableResources are the resources that this task can attach. This is used for
	// translating between resource slugs and resource IDs.
	AvailableResources []api.ResourceMetadata
}

// Update updates a definition by applying the UpdateTaskRequest using patch semantics.
func (d *Definition) Update(req api.UpdateTaskRequest, opts UpdateOptions) error {
	d.Slug = req.Slug
	d.Name = req.Name
	d.Description = req.Description
	d.Runtime = req.Runtime
	d.Timeout.value = req.Timeout

	if err := d.updateKindSpecific(req, opts.AvailableResources); err != nil {
		return err
	}

	if req.Parameters != nil {
		parameters, err := convertParametersAPIToDef(req.Parameters)
		if err != nil {
			return err
		}
		d.Parameters = parameters
	}

	if req.Configs != nil {
		d.Configs = []string{}
		for _, config := range *req.Configs {
			d.Configs = append(d.Configs, config.NameTag)
		}
	}

	if req.Constraints.Labels != nil {
		d.Constraints = map[string]string{}
		for _, label := range req.Constraints.Labels {
			d.Constraints[label.Key] = label.Value
		}
	}

	if req.Resources != nil {
		d.Resources = ResourceDefinition{Attachments: map[string]string{}}
		if err := d.updateResources(req.Resources, req.Kind, opts.AvailableResources); err != nil {
			return err
		}
	}

	if req.ExecuteRules.RequireRequests != nil {
		d.RequireRequests = *req.ExecuteRules.RequireRequests
	}
	if req.ExecuteRules.DisallowSelfApprove != nil {
		d.AllowSelfApprovals.value = pointers.Bool(!*req.ExecuteRules.DisallowSelfApprove)
	}
	d.RestrictCallers = req.ExecuteRules.RestrictCallers
	if d.RestrictCallers == nil {
		d.RestrictCallers = []string{}
	}

	if opts.Triggers != nil {
		d.Schedules = map[string]ScheduleDefinition{}
		for _, trigger := range opts.Triggers {
			if trigger.Kind != api.TriggerKindSchedule || trigger.Slug == nil {
				// This trigger is not a schedule deployed via code.
				continue
			}
			if trigger.ArchivedAt != nil || trigger.DisabledAt != nil {
				// Trigger is archived or disabled, so don't add it to the definition.
				continue
			}

			d.Schedules[*trigger.Slug] = ScheduleDefinition{
				Name:        trigger.Name,
				Description: trigger.Description,
				CronExpr:    trigger.KindConfig.Schedule.CronExpr.String(),
				ParamValues: trigger.KindConfig.Schedule.ParamValues,
			}
		}
	}

	return nil
}

func (d *Definition) updateResources(resources api.Resources, kind build.TaskKind, availableResources []api.ResourceMetadata) error {
	if len(resources) == 0 {
		return nil
	}

	d.Resources.Attachments = map[string]string{}
	for alias, id := range resources {
		// Ignore SQL/REST resources; they get routed elsewhere.
		if (kind == build.TaskKindSQL && alias == "db") ||
			(kind == build.TaskKindREST && alias == "rest") ||
			(kind == build.TaskKindBuiltin) {
			continue
		}
		if resource := getResourceByID(availableResources, id); resource != nil {
			d.Resources.Attachments[alias] = resource.Slug
		}
	}

	return nil
}

func (d *Definition) updateKindSpecific(t api.UpdateTaskRequest, availableResources []api.ResourceMetadata) error {
	switch t.Kind {
	case build.TaskKindImage:
		d.Image = &ImageDefinition{}
		return d.Image.update(t, availableResources)
	case build.TaskKindNode:
		d.Node = &NodeDefinition{}
		return d.Node.update(t, availableResources)
	case build.TaskKindPython:
		d.Python = &PythonDefinition{}
		return d.Python.update(t, availableResources)
	case build.TaskKindShell:
		d.Shell = &ShellDefinition{}
		return d.Shell.update(t, availableResources)
	case build.TaskKindSQL:
		d.SQL = &SQLDefinition{}
		return d.SQL.update(t, availableResources)
	case build.TaskKindREST:
		d.REST = &RESTDefinition{}
		return d.REST.update(t, availableResources)
	case build.TaskKindBuiltin:
		return updateBuiltin(d, t, availableResources)
	default:
		return errors.Errorf("unknown task kind: %s", t.Kind)
	}
}
