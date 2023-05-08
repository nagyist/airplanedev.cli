package api

import (
	"context"
	"encoding/json"
	"time"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/resources"
	"github.com/airplanedev/cli/pkg/utils/pointers"
	"github.com/airplanedev/ojson"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type IAPIClient interface {
	// GetTask fetches a task by slug. If the slug does not match a task, a *TaskMissingError is returned.
	GetTask(ctx context.Context, req GetTaskRequest) (res Task, err error)
	// GetTaskMetadata fetches a task's metadata by slug. If the slug does not match a task, a *TaskMissingError is returned.
	GetTaskMetadata(ctx context.Context, slug string) (res TaskMetadata, err error)
	GetView(ctx context.Context, req GetViewRequest) (res View, err error)
	ListResources(ctx context.Context, envSlug string) (res ListResourcesResponse, err error)
	ListResourceMetadata(ctx context.Context) (res ListResourceMetadataResponse, err error)
	CreateBuildUpload(ctx context.Context, req CreateBuildUploadRequest) (res CreateBuildUploadResponse, err error)
}

// Task represents a task.
type Task struct {
	URL                        string                 `json:"-" yaml:"-"`
	ID                         string                 `json:"taskID" yaml:"id"`
	Name                       string                 `json:"name" yaml:"name"`
	Slug                       string                 `json:"slug" yaml:"slug"`
	Description                string                 `json:"description" yaml:"description"`
	Image                      *string                `json:"image" yaml:"image"`
	Command                    []string               `json:"command" yaml:"command"`
	Arguments                  []string               `json:"arguments" yaml:"arguments"`
	Parameters                 Parameters             `json:"parameters" yaml:"parameters"`
	Configs                    []ConfigAttachment     `json:"configs" yaml:"configs"`
	Constraints                RunConstraints         `json:"constraints" yaml:"constraints"`
	Env                        EnvVars                `json:"env" yaml:"env"`
	ResourceRequests           ResourceRequests       `json:"resourceRequests" yaml:"resourceRequests"`
	Resources                  Resources              `json:"resources" yaml:"resources"`
	Kind                       buildtypes.TaskKind    `json:"kind" yaml:"kind"`
	KindOptions                buildtypes.KindOptions `json:"kindOptions" yaml:"kindOptions"`
	Runtime                    buildtypes.TaskRuntime `json:"runtime" yaml:"runtime"`
	Repo                       string                 `json:"repo" yaml:"repo"`
	RequireExplicitPermissions bool                   `json:"requireExplicitPermissions" yaml:"-"`
	Permissions                Permissions            `json:"permissions" yaml:"-"`
	DefaultRunPermissions      DefaultRunPermissions  `json:"defaultRunPermissions" yaml:"defaultRunPermissions"`
	ExecuteRules               ExecuteRules           `json:"executeRules" yaml:"-"`
	Timeout                    int                    `json:"timeout" yaml:"timeout"`
	IsArchived                 bool                   `json:"isArchived" yaml:"isArchived"`
	InterpolationMode          string                 `json:"interpolationMode" yaml:"-"`
	Triggers                   []Trigger              `json:"triggers" yaml:"-"`
	SDKVersion                 *string                `json:"sdkVersion" yaml:"-"`

	CreatedAt time.Time `json:"createdAt" yaml:"-"`
	// Computed based on the task's revision.
	UpdatedAt time.Time `json:"updatedAt" yaml:"-"`
}

// AsUpdateTaskRequest converts a Task into an UpdateTaskRequest.
//
// Keep in mind that fields that are not managed as code are not included:
// - RequireExplicitPermissions
// - Permissions
// - BuildID
// - EnvSlug
// - Repo
// - ResourceRequests
// - InterpolationMode
func (t Task) AsUpdateTaskRequest() UpdateTaskRequest {
	req := UpdateTaskRequest{
		Slug:        t.Slug,
		Name:        t.Name,
		Description: t.Description,
		Image:       t.Image,
		Command:     t.Command,
		Arguments:   t.Arguments,
		Parameters:  t.Parameters,
		Configs:     &t.Configs,
		Constraints: t.Constraints,
		Env:         t.Env,
		Resources:   t.Resources,
		Kind:        t.Kind,
		KindOptions: t.KindOptions,
		Runtime:     t.Runtime,
		ExecuteRules: UpdateExecuteRulesRequest{
			DisallowSelfApprove: &t.ExecuteRules.DisallowSelfApprove,
			RequireRequests:     &t.ExecuteRules.RequireRequests,
			RestrictCallers:     t.ExecuteRules.RestrictCallers,
			ConcurrencyKey:      &t.ExecuteRules.ConcurrencyKey,
			ConcurrencyLimit:    t.ExecuteRules.ConcurrencyLimit,
		},
		Timeout:               t.Timeout,
		DefaultRunPermissions: (*DefaultRunPermissions)(pointers.String(string(t.DefaultRunPermissions))),
		SDKVersion:            t.SDKVersion,
	}

	// Ensure all nullable fields are initialized since UpdateTaskRequest uses patch semantics.
	if req.Kind == buildtypes.TaskKindImage {
		if req.Command == nil {
			req.Command = []string{}
		}
		if req.Arguments == nil {
			req.Arguments = []string{}
		}
	}
	if req.Parameters == nil {
		req.Parameters = []Parameter{}
	}
	if req.Configs == nil || *req.Configs == nil {
		req.Configs = &[]ConfigAttachment{}
	}
	if req.Constraints.Labels == nil {
		req.Constraints.Labels = []AgentLabel{}
	}
	if req.Env == nil {
		req.Env = EnvVars{}
	}
	if req.Resources == nil {
		req.Resources = map[string]string{}
	}
	if req.KindOptions == nil {
		req.KindOptions = map[string]any{}
	}
	if req.ExecuteRules.DisallowSelfApprove == nil {
		req.ExecuteRules.DisallowSelfApprove = pointers.Bool(false)
	}
	if req.ExecuteRules.RequireRequests == nil {
		req.ExecuteRules.RequireRequests = pointers.Bool(false)
	}
	if req.ExecuteRules.RestrictCallers == nil {
		req.ExecuteRules.RestrictCallers = []string{}
	}
	if req.ExecuteRules.ConcurrencyKey == nil {
		req.ExecuteRules.ConcurrencyKey = pointers.String("")
	}
	if req.ExecuteRules.ConcurrencyLimit == nil {
		req.ExecuteRules.ConcurrencyLimit = pointers.Int64(1)
	}
	if req.DefaultRunPermissions == nil {
		req.DefaultRunPermissions = (*DefaultRunPermissions)(pointers.String(string(DefaultRunPermissionTaskViewers)))
	}

	if t.InterpolationMode != "" {
		req.InterpolationMode = &t.InterpolationMode
	}

	return req
}

type GetTaskRequest struct {
	Slug    string
	EnvSlug string
}

type TaskMetadata struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	IsArchived bool   `json:"isArchived"`
	// IsLocal is true if the task is local in the editor, false if it's a
	// task that's already deployed.
	IsLocal bool `json:"isLocal"`
}

type CreateBuildUploadRequest struct {
	SizeBytes int `json:"sizeBytes"`
}

type CreateBuildUploadResponse struct {
	Upload       Upload `json:"upload"`
	WriteOnlyURL string `json:"writeOnlyURL"`
}

type Upload struct {
	ID               string    `json:"id"`
	FileName         string    `json:"fileName"`
	URL              string    `json:"url"`
	SizeBytes        int       `json:"sizeBytes"`
	CreatedAt        time.Time `json:"createdAt"`
	TeamID           string    `json:"teamID"`
	CreatorUserID    *string   `json:"creatorUserID"`
	RunID            *string   `json:"runID"`
	TriggerRequestID *string   `json:"triggerRequestID"`
	SessionID        *string   `json:"sessionID"`
}

type CreateUploadRequest struct {
	FileName  string `json:"fileName"`
	SizeBytes int    `json:"sizeBytes"`
}

type CreateUploadResponse struct {
	Upload       Upload `json:"upload"`
	ReadOnlyURL  string `json:"readOnlyURL"`
	WriteOnlyURL string `json:"writeOnlyURL"`
}

type GetUploadRequest struct {
	UploadID string `json:"uploadID"`
}

type GetUploadResponse struct {
	Upload      Upload `json:"upload"`
	ReadOnlyURL string `json:"readOnlyURL"`
}

// CreateTaskRequest creates a new task.
type CreateTaskRequest struct {
	Slug             string                 `json:"slug"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Image            *string                `json:"image"`
	Command          []string               `json:"command"`
	Arguments        []string               `json:"arguments"`
	Parameters       Parameters             `json:"parameters"`
	Configs          []ConfigAttachment     `json:"configs"`
	Constraints      RunConstraints         `json:"constraints"`
	EnvVars          EnvVars                `json:"env"`
	ResourceRequests map[string]string      `json:"resourceRequests"`
	Resources        map[string]string      `json:"resources"`
	Kind             buildtypes.TaskKind    `json:"kind"`
	KindOptions      buildtypes.KindOptions `json:"kindOptions"`
	Runtime          buildtypes.TaskRuntime `json:"runtime"`
	Repo             string                 `json:"repo"`
	Timeout          int                    `json:"timeout"`
	EnvSlug          string                 `json:"envSlug"`
}

// CreateTaskResponse represents a create task response.
type CreateTaskResponse struct {
	TaskID         string `json:"taskID"`
	Slug           string `json:"slug"`
	TaskRevisionID string `json:"taskRevisionID"`
}

// UpdateTaskRequest updates a task.
type UpdateTaskRequest struct {
	Slug                       string                    `json:"slug"`
	Name                       string                    `json:"name"`
	Description                string                    `json:"description"`
	Image                      *string                   `json:"image"`
	Command                    []string                  `json:"command"`
	Arguments                  []string                  `json:"arguments"`
	Parameters                 Parameters                `json:"parameters"`
	Configs                    *[]ConfigAttachment       `json:"configs"`
	Constraints                RunConstraints            `json:"constraints"`
	Env                        EnvVars                   `json:"env"`
	ResourceRequests           map[string]string         `json:"resourceRequests"`
	Resources                  map[string]string         `json:"resources"`
	Kind                       buildtypes.TaskKind       `json:"kind"`
	KindOptions                buildtypes.KindOptions    `json:"kindOptions"`
	Runtime                    buildtypes.TaskRuntime    `json:"runtime"`
	Repo                       string                    `json:"repo"`
	RequireExplicitPermissions *bool                     `json:"requireExplicitPermissions"`
	Permissions                *Permissions              `json:"permissions"`
	ExecuteRules               UpdateExecuteRulesRequest `json:"executeRules"`
	DefaultRunPermissions      *DefaultRunPermissions    `json:"defaultRunPermissions"`
	SDKVersion                 *string                   `json:"sdkVersion"`
	Timeout                    int                       `json:"timeout"`
	BuildID                    *string                   `json:"buildID"`
	InterpolationMode          *string                   `json:"interpolationMode"`
	EnvSlug                    string                    `json:"envSlug"`
}

type UpdateViewRequest struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	EnvVars     EnvVars `json:"envVars"`
}

type UpdateExecuteRulesRequest struct {
	DisallowSelfApprove *bool    `json:"disallowSelfApprove"`
	RequireRequests     *bool    `json:"requireRequests"`
	RestrictCallers     []string `json:"restrictCallers"`
	ConcurrencyKey      *string  `json:"concurrencyKey"`
	ConcurrencyLimit    *int64   `json:"concurrencyLimit"`
}

type ListResourcesResponse struct {
	Resources []Resource `json:"resources"`
}

type GetResourceResponse struct {
	Resource
}

type Resource struct {
	ID             string             `json:"id"`
	Slug           string             `json:"slug"`
	TeamID         string             `json:"teamID"`
	Name           string             `json:"name"`
	Kind           ResourceKind       `json:"kind"`
	ExportResource resources.Resource `json:"resource"`

	CreatedAt time.Time `json:"createdAt"`
	CreatedBy string    `json:"createdBy"`
	UpdatedAt time.Time `json:"updatedAt"`
	UpdatedBy string    `json:"updatedBy"`

	IsPrivate bool `json:"isPrivate"`

	CanUseResource    bool `json:"canUseResource"`
	CanUpdateResource bool `json:"canUpdateResource"`
}

func (r *Resource) UnmarshalJSON(buf []byte) error {
	var raw struct {
		ID             string                 `json:"id"`
		Slug           string                 `json:"slug"`
		TeamID         string                 `json:"teamID"`
		Name           string                 `json:"name"`
		Kind           ResourceKind           `json:"kind"`
		ExportResource map[string]interface{} `json:"resource"`

		CreatedAt time.Time `json:"createdAt"`
		CreatedBy string    `json:"createdBy"`
		UpdatedAt time.Time `json:"updatedAt"`
		UpdatedBy string    `json:"updatedBy"`

		IsPrivate bool `json:"isPrivate"`

		CanUseResource    bool `json:"canUseResource"`
		CanUpdateResource bool `json:"canUpdateResource"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}

	var export resources.Resource
	var err error
	if raw.ExportResource != nil {
		export, err = resources.GetResource(resources.ResourceKind(raw.Kind), raw.ExportResource)
		if err != nil {
			return err
		}
	}

	r.ID = raw.ID
	r.Slug = raw.Slug
	r.TeamID = raw.TeamID
	r.Name = raw.Name
	r.Kind = raw.Kind
	r.ExportResource = export
	r.CreatedAt = raw.CreatedAt
	r.CreatedBy = raw.CreatedBy
	r.UpdatedAt = raw.UpdatedAt
	r.UpdatedBy = raw.UpdatedBy
	r.IsPrivate = raw.IsPrivate
	r.CanUseResource = raw.CanUseResource
	r.CanUpdateResource = raw.CanUpdateResource

	return nil
}

type ResourceKind string

const (
	KindUnknown  ResourceKind = ""
	KindPostgres ResourceKind = "postgres"
	KindMySQL    ResourceKind = "mysql"
	KindREST     ResourceKind = "rest"
)

type ListResourceMetadataResponse struct {
	Resources []ResourceMetadata `json:"resources"`
}

type ResourceMetadata struct {
	ID                 string    `json:"id"`
	Slug               string    `json:"slug"`
	DefaultEnvResource *Resource `json:"defaultEnvResource"`
}

type DefaultRunPermissions string

const (
	DefaultRunPermissionTaskViewers      DefaultRunPermissions = "task-viewers"
	DefaultRunPermissionTaskParticipants DefaultRunPermissions = "task-participants"
)

type Permissions []Permission

type Permission struct {
	Action     Action  `json:"action,omitempty"`
	RoleID     RoleID  `json:"roleID,omitempty"`
	SubUserID  *string `json:"subUserID"`
	SubGroupID *string `json:"subGroupID"`
}

type Action string

type RoleID string

const (
	RoleTeamAdmin        RoleID = "team_admin"
	RoleTeamDeveloper    RoleID = "team_developer"
	RoleTaskViewer       RoleID = "task_viewer"
	RoleTaskRequester    RoleID = "task_requester"
	RoleTaskExecuter     RoleID = "task_executer"
	RoleTaskAdmin        RoleID = "task_admin"
	RoleRunViewer        RoleID = "run_viewer"
	RoleRunbookViewer    RoleID = "runbook_viewer"
	RoleRunbookRequester RoleID = "runbook_requester"
	RoleRunbookExecuter  RoleID = "runbook_executer"
	RoleRunbookAdmin     RoleID = "runbook_admin"
	RoleSessionViewer    RoleID = "session_viewer"
	RoleSessionExecuter  RoleID = "session_executer"
	RoleSessionAdmin     RoleID = "session_admin"
	RoleResourceUser     RoleID = "resource_user"
)

type ResourceRequests map[string]string

type Resources map[string]string

type EnvVars map[string]EnvVarValue

type EnvVarValue struct {
	Value  *string `json:"value,omitempty" yaml:"value,omitempty"`
	Config *string `json:"config,omitempty" yaml:"config,omitempty"`
}

var _ yaml.Unmarshaler = &EnvVarValue{}

// UnmarshalJSON allows you set an env var's `value` using either
// of these notations:
//
//	AIRPLANE_DSN: "foobar"
//
//	AIRPLANE_DSN:
//	  value: "foobar"
func (ev *EnvVarValue) UnmarshalYAML(node *yaml.Node) error {
	// First, try to unmarshal as a string.
	// This would be the first case above.
	var value string
	if err := node.Decode(&value); err == nil {
		// Success!
		ev.Value = &value
		return nil
	}

	// Otherwise, perform a normal unmarshal operation.
	// This would be the second case above.
	//
	// Note we need a new type, otherwise we recursively call this
	// method and end up stack overflowing.
	type envVarValue EnvVarValue
	var v envVarValue
	if err := node.Decode(&v); err != nil {
		return err
	}
	*ev = EnvVarValue(v)

	return nil
}

var _ json.Unmarshaler = &EnvVarValue{}

func (ev *EnvVarValue) UnmarshalJSON(b []byte) error {
	// First, try to unmarshal as a string.
	var value string
	if err := json.Unmarshal(b, &value); err == nil {
		// Success!
		ev.Value = &value
		return nil
	}

	// Otherwise, perform a normal unmarshal operation.
	//
	// Note we need a new type, otherwise we recursively call this
	// method and end up stack overflowing.
	type envVarValue EnvVarValue
	var v envVarValue
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*ev = EnvVarValue(v)

	return nil
}

// Parameters represents a slice of task parameters.
type Parameters []Parameter

// Parameter represents a task parameter.
type Parameter struct {
	Name        string      `json:"name" yaml:"name"`
	Slug        string      `json:"slug" yaml:"slug"`
	Type        Type        `json:"type" yaml:"type"`
	Desc        string      `json:"desc" yaml:"desc,omitempty"`
	Component   Component   `json:"component" yaml:"component,omitempty"`
	Default     Value       `json:"default" yaml:"default,omitempty"`
	Constraints Constraints `json:"constraints" yaml:"constraints,omitempty"`
}

// ConfigAttachment represents a config attachment.
type ConfigAttachment struct {
	NameTag string `json:"nameTag" yaml:"nameTag"`
}

// UnmarshalJSON implementation.
func (p *Parameters) UnmarshalJSON(buf []byte) error {
	var tmp struct {
		Parameters []Parameter `json:"parameters"`
	}

	if err := json.Unmarshal(buf, &tmp); err != nil {
		return err
	}

	*p = tmp.Parameters
	return nil
}

// MarshalJSON implementation.
func (p Parameters) MarshalJSON() ([]byte, error) {
	type object struct {
		Parameters []Parameter `json:"parameters"`
	}
	return json.Marshal(object{p})
}

// Constraints represent constraints.
type Constraints struct {
	Optional bool               `json:"optional" yaml:"optional,omitempty"`
	Regex    string             `json:"regex" yaml:"regex,omitempty"`
	Options  []ConstraintOption `json:"options,omitempty" yaml:"options,omitempty"`
}

type ConstraintOption struct {
	Label string `json:"label"`
	Value Value  `json:"value"`
}

// Value represents a value.
type Value interface{}

// Type enumerates parameter types.
type Type string

// All Parameter types.
const (
	TypeString    Type = "string"
	TypeBoolean   Type = "boolean"
	TypeUpload    Type = "upload"
	TypeInteger   Type = "integer"
	TypeFloat     Type = "float"
	TypeDate      Type = "date"
	TypeDatetime  Type = "datetime"
	TypeConfigVar Type = "configvar"
)

// Component enumerates components.
type Component string

// All Component types.
const (
	ComponentNone      Component = ""
	ComponentEditorSQL Component = "editor-sql"
	ComponentTextarea  Component = "textarea"
)

// RunConstraints represents run constraints.
type RunConstraints struct {
	Labels []AgentLabel `json:"labels" yaml:"labels"`
}

func (rc RunConstraints) IsEmpty() bool {
	return len(rc.Labels) == 0
}

// AgentLabel represents an agent label.
type AgentLabel struct {
	Key   string `json:"key" yaml:"key"`
	Value string `json:"value" yaml:"value"`
}

type ExecuteRules struct {
	DisallowSelfApprove bool     `json:"disallowSelfApprove"`
	RequireRequests     bool     `json:"requireRequests"`
	RestrictCallers     []string `json:"restrictCallers"`
	ConcurrencyKey      string   `json:"concurrencyKey"`
	ConcurrencyLimit    *int64   `json:"concurrencyLimit"`
}

type View struct {
	ID              string            `json:"id"`
	Slug            string            `json:"slug"`
	ArchivedAt      *time.Time        `json:"archivedAt"`
	ArchivedBy      *string           `json:"archivedBy"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	CreatedBy       string            `json:"createdBy"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedBy       string            `json:"updatedBy"`
	UpdatedAt       time.Time         `json:"updatedAt"`
	EnvVars         EnvVars           `json:"envVars"`
	ResolvedEnvVars map[string]string `json:"resolvedEnvVars"`
	// IsLocal is true if the view is local in the editor, false if it's a
	// view that's already deployed.
	IsLocal bool `json:"isLocal"`
}

type ViewMetadata struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	IsArchived bool   `json:"isArchived"`
	// IsLocal is true if the view is local in the editor, false if it's a
	// view that's already deployed.
	IsLocal bool `json:"isLocal"`
}

type GetViewRequest struct {
	ID   string
	Slug string
}

type CreateViewRequest struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	EnvVars     EnvVars `json:"envVars"`
}

type Schedule struct {
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	CronExpr    string                 `json:"cronExpr"`
	ParamValues map[string]interface{} `json:"paramValues,omitempty"`
}

type Display struct {
	ID        string    `json:"id"`
	RunID     string    `json:"runID"`
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// kind=markdown
	Content string `json:"content"`

	// kind=table
	Rows    []ojson.Value        `json:"rows"`
	Columns []DisplayTableColumn `json:"columns"`

	// kind=json
	Value any `json:"value"`

	// kind=file
	UploadID string `json:"uploadID"`
}

type DisplayTableColumn struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Prompt struct {
	ID          string                 `json:"id"`
	RunID       string                 `json:"runID"`
	Schema      Parameters             `json:"schema"`
	Values      map[string]interface{} `json:"values"`
	CreatedAt   time.Time              `json:"createdAt"`
	SubmittedAt *time.Time             `json:"submittedAt"`
	SubmittedBy *string                `json:"submittedBy"`
	CancelledAt *time.Time             `json:"cancelledAt"`
	CancelledBy *string                `json:"cancelledBy"`
	Reviewers   *PromptReviewers       `json:"reviewers"`
	ConfirmText string                 `json:"confirmText"`
	CancelText  string                 `json:"cancelText"`
	Description string                 `json:"description"`
}

type PromptReviewers struct {
	// Groups are identified by their slugs.
	Groups []string `json:"groups"`
	// Users are identified by their emails.
	Users              []string `json:"users"`
	AllowSelfApprovals *bool    `json:"allowSelfApprovals"`
}

type Sleep struct {
	// Unique ID of the sleep.
	ID string `json:"id"`
	// RunID identifies the run that is sleeping.
	RunID string `json:"runID"`
	// DurationMs is the length of the sleep in milliseconds, used for display purposes only.
	DurationMs int `json:"durationMs"`
	// CreatedAt is when this sleep was started. This is generated on the server side.
	CreatedAt time.Time `json:"createdAt"`
	// Until is the sleep end time.
	Until time.Time `json:"until"`

	// When the sleep was skipped. This value will be null if the sleep was not skipped.
	SkippedAt *time.Time `json:"skippedAt"`
	SkippedBy *string    `json:"skippedBy"`
}

type Env struct {
	ID         string     `json:"id"`
	Slug       string     `json:"slug"`
	Name       string     `json:"name"`
	TeamID     string     `json:"teamID"`
	Default    bool       `json:"default"`
	CreatedAt  time.Time  `json:"createdAt"`
	CreatedBy  string     `json:"createdBy"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	UpdatedBy  string     `json:"updatedBy"`
	IsArchived bool       `json:"isArchived"`
	ArchivedAt *time.Time `json:"archivedAt"`
}

type EvaluateTemplateRequest struct {
	// Value is an arbitrary value that can include one or more Template values.
	// Each Template will be evaluated, and if successful, will be replaced in
	// Value with its output. The updated Value will be returned in the response.
	// If any templates fail to evaluate, the Template will be left in Value
	// and a separate error will be returned in the response.
	Value interface{} `json:"value"`
	// TODO: Add Run struct to lib and use Run instead
	RunID       string                        `json:"runID"`
	Env         Env                           `json:"env"`
	Resources   map[string]resources.Resource `json:"resources"`
	Configs     map[string]string             `json:"configs"`
	ParamValues map[string]interface{}        `json:"paramValues"`
	ParentRunID string                        `json:"parentRunID"`
	TaskID      string                        `json:"taskID"`
	TaskSlug    string                        `json:"taskSlug"`
	// If strict mode is disabled, then the template engine will not throw an error for invalid templates.
	DisableStrictMode bool `json:"disableStrictMode"`
	// Lookup maps is a mapping of namespace to the lookup map for that namespace.
	// These are in addition to the lookup maps generated above. The user cannot
	// override a default lookup, i.e. a lookup map with namespace "params" would
	// always use ParamValues as the lookup map.
	// For example:
	// {
	//   "extra_lookups": {
	//     "field1": "hello",
	//     "field2": "hi"
	//   }
	// }
	// Any references to extra_lookups.field1 or extra_lookups.field2 in Value
	// will be replaced by the values above.
	LookupMaps map[string]interface{} `json:"lookupMaps"`
}

type EvaluateTemplateResponse struct {
	Value interface{} `json:"value"`
}

func (r *EvaluateTemplateRequest) UnmarshalJSON(buf []byte) error {
	var raw struct {
		Value       interface{}                       `json:"value"`
		RunID       string                            `json:"runID"`
		Env         Env                               `json:"env"`
		Resources   map[string]map[string]interface{} `json:"resources"`
		Configs     map[string]string                 `json:"configs"`
		ParamValues map[string]interface{}            `json:"paramValues"`
	}

	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}

	exportResources := make(map[string]resources.Resource, len(raw.Resources))
	for slug, res := range raw.Resources {
		kind, ok := res["kind"]
		if !ok {
			return errors.New("export resource does not have kind")
		}
		kindStr, ok := kind.(string)
		if !ok {
			return errors.Errorf("kind is unexpected type %T", kind)
		}

		export, err := resources.GetResource(resources.ResourceKind(kindStr), res)
		if err != nil {
			return err
		}
		exportResources[slug] = export
	}

	r.Value = raw.Value
	r.RunID = raw.RunID
	r.Env = raw.Env
	r.Resources = exportResources
	r.Configs = raw.Configs
	r.ParamValues = raw.ParamValues

	return nil
}
