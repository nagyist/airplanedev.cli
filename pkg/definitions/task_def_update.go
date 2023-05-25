package definitions

import (
	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	api "github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/airplanedev/cli/pkg/utils/pointers"
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
	Users              []api.User
	Groups             []api.Group
}

// Update updates a definition by applying the UpdateTaskRequest using patch semantics.
func (d *Definition) Update(req api.UpdateTaskRequest, opts UpdateOptions) error {
	d.Slug = req.Slug
	d.Name = req.Name
	d.Description = req.Description
	d.Runtime = req.Runtime
	d.Timeout = req.Timeout

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
		d.Resources = map[string]string{}
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
	if req.ExecuteRules.ConcurrencyKey != nil {
		d.ConcurrencyKey = *req.ExecuteRules.ConcurrencyKey
	}
	if req.ExecuteRules.ConcurrencyLimit != nil {
		d.ConcurrencyLimit = NewDefaultOneDefinition(int(*req.ExecuteRules.ConcurrencyLimit))
	}
	if req.DefaultRunPermissions != nil {
		d.DefaultRunPermissions = NewDefaultTaskViewersDefinition(*req.DefaultRunPermissions)
	}
	if req.SDKVersion != nil {
		d.SDKVersion = req.SDKVersion
	}
	if req.Permissions != nil {
		if err := d.updatePermissions(*req.Permissions, opts.Users, opts.Groups); err != nil {
			return err
		}
	}
	if req.RequireExplicitPermissions != nil {
		if !*req.RequireExplicitPermissions {
			d.Permissions = nil
		} else {
			if d.Permissions == nil {
				d.Permissions = &PermissionsDefinition{}
			}
			d.Permissions.RequireExplicitPermissions = true
		}
	}
	if d.Permissions == nil && req.PermissionsSource != nil && *req.PermissionsSource == api.PermissionsSourceCode {
		// Update did not have permissions set but indicated that permissions should be set via code only.
		// Set permissions to a non-nil value to disambiguate from the case where permissions don't have to be managed via code.
		d.Permissions = &PermissionsDefinition{}
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

func (d *Definition) updateResources(resources api.Resources, kind buildtypes.TaskKind, availableResources []api.ResourceMetadata) error {
	if len(resources) == 0 {
		return nil
	}

	d.Resources = map[string]string{}
	for alias, id := range resources {
		// Ignore SQL/REST resources; they get routed elsewhere.
		if (kind == buildtypes.TaskKindSQL && alias == "db") ||
			(kind == buildtypes.TaskKindREST && alias == "rest") ||
			(kind == buildtypes.TaskKindBuiltin) {
			continue
		}
		if resource := getResourceByID(availableResources, id); resource != nil {
			d.Resources[alias] = resource.Slug
		}
	}

	return nil
}

func (d *Definition) updatePermissions(permissions api.Permissions, users []api.User, groups []api.Group) error {
	d.Permissions = &PermissionsDefinition{}
	for _, p := range permissions {
		var userEmail string
		var groupSlug string
		if p.SubUserID != nil {
			if user := getUserByID(users, *p.SubUserID); user != nil {
				userEmail = user.Email
			}
		}
		if p.SubGroupID != nil {
			if group := getGroupByID(groups, *p.SubGroupID); group != nil {
				groupSlug = group.Slug
			}
		}
		switch p.RoleID {
		case api.RoleTaskViewer:
			if userEmail != "" {
				d.Permissions.Viewers.Users = append(d.Permissions.Viewers.Users, userEmail)
			} else if groupSlug != "" {
				d.Permissions.Viewers.Groups = append(d.Permissions.Viewers.Groups, groupSlug)
			}
		case api.RoleTaskRequester:
			if userEmail != "" {
				d.Permissions.Requesters.Users = append(d.Permissions.Requesters.Users, userEmail)
			} else if groupSlug != "" {
				d.Permissions.Requesters.Groups = append(d.Permissions.Requesters.Groups, groupSlug)
			}
		case api.RoleTaskExecuter:
			if userEmail != "" {
				d.Permissions.Executers.Users = append(d.Permissions.Executers.Users, userEmail)
			} else if groupSlug != "" {
				d.Permissions.Executers.Groups = append(d.Permissions.Executers.Groups, groupSlug)
			}
		case api.RoleTaskAdmin:
			if userEmail != "" {
				d.Permissions.Admins.Users = append(d.Permissions.Admins.Users, userEmail)
			} else if groupSlug != "" {
				d.Permissions.Admins.Groups = append(d.Permissions.Admins.Groups, groupSlug)
			}
		}
	}
	return nil
}

func (d *Definition) updateKindSpecific(t api.UpdateTaskRequest, availableResources []api.ResourceMetadata) error {
	switch t.Kind {
	case buildtypes.TaskKindImage:
		if d.Image == nil {
			d.Image = &ImageDefinition{}
		}
		return d.Image.update(t, availableResources)
	case buildtypes.TaskKindNode:
		if d.Node == nil {
			d.Node = &NodeDefinition{}
		}
		return d.Node.update(t, availableResources)
	case buildtypes.TaskKindPython:
		if d.Python == nil {
			d.Python = &PythonDefinition{}
		}
		return d.Python.update(t, availableResources)
	case buildtypes.TaskKindShell:
		if d.Shell == nil {
			d.Shell = &ShellDefinition{}
		}
		return d.Shell.update(t, availableResources)
	case buildtypes.TaskKindSQL:
		if d.SQL == nil {
			d.SQL = &SQLDefinition{}
		}
		return d.SQL.update(t, availableResources)
	case buildtypes.TaskKindREST:
		if d.REST == nil {
			d.REST = &RESTDefinition{}
		}
		return d.REST.update(t, availableResources)
	case buildtypes.TaskKindBuiltin:
		return updateBuiltin(d, t, availableResources)
	default:
		return errors.Errorf("unknown task kind: %s", t.Kind)
	}
}
