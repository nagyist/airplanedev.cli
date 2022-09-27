package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/ojson"
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
	URL                        string             `json:"-" yaml:"-"`
	ID                         string             `json:"taskID" yaml:"id"`
	Name                       string             `json:"name" yaml:"name"`
	Slug                       string             `json:"slug" yaml:"slug"`
	Description                string             `json:"description" yaml:"description"`
	Image                      *string            `json:"image" yaml:"image"`
	Command                    []string           `json:"command" yaml:"command"`
	Arguments                  []string           `json:"arguments" yaml:"arguments"`
	Parameters                 Parameters         `json:"parameters" yaml:"parameters"`
	Configs                    []ConfigAttachment `json:"configs" yaml:"configs"`
	Constraints                RunConstraints     `json:"constraints" yaml:"constraints"`
	Env                        TaskEnv            `json:"env" yaml:"env"`
	ResourceRequests           ResourceRequests   `json:"resourceRequests" yaml:"resourceRequests"`
	Resources                  Resources          `json:"resources" yaml:"resources"`
	Kind                       build.TaskKind     `json:"kind" yaml:"kind"`
	KindOptions                build.KindOptions  `json:"kindOptions" yaml:"kindOptions"`
	Runtime                    build.TaskRuntime  `json:"runtime" yaml:"runtime"`
	Repo                       string             `json:"repo" yaml:"repo"`
	RequireExplicitPermissions bool               `json:"requireExplicitPermissions" yaml:"-"`
	Permissions                Permissions        `json:"permissions" yaml:"-"`
	ExecuteRules               ExecuteRules       `json:"executeRules" yaml:"-"`
	Timeout                    int                `json:"timeout" yaml:"timeout"`
	IsArchived                 bool               `json:"isArchived" yaml:"isArchived"`
	InterpolationMode          string             `json:"interpolationMode" yaml:"-"`
	Triggers                   []Trigger          `json:"triggers" yaml:"-"`
}

type GetTaskRequest struct {
	Slug    string
	EnvSlug string
}

type TaskMetadata struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	IsArchived bool   `json:"isArchived"`
}

type CreateBuildUploadRequest struct {
	SizeBytes int `json:"sizeBytes"`
}

type CreateBuildUploadResponse struct {
	Upload       Upload `json:"upload"`
	WriteOnlyURL string `json:"writeOnlyURL"`
}

type Upload struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// CreateTaskRequest creates a new task.
type CreateTaskRequest struct {
	Slug             string             `json:"slug"`
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	Image            *string            `json:"image"`
	Command          []string           `json:"command"`
	Arguments        []string           `json:"arguments"`
	Parameters       Parameters         `json:"parameters"`
	Configs          []ConfigAttachment `json:"configs"`
	Constraints      RunConstraints     `json:"constraints"`
	EnvVars          TaskEnv            `json:"env"`
	ResourceRequests map[string]string  `json:"resourceRequests"`
	Resources        map[string]string  `json:"resources"`
	Kind             build.TaskKind     `json:"kind"`
	KindOptions      build.KindOptions  `json:"kindOptions"`
	Runtime          build.TaskRuntime  `json:"runtime"`
	Repo             string             `json:"repo"`
	Timeout          int                `json:"timeout"`
	EnvSlug          string             `json:"envSlug"`
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
	Env                        TaskEnv                   `json:"env"`
	ResourceRequests           map[string]string         `json:"resourceRequests"`
	Resources                  map[string]string         `json:"resources"`
	Kind                       build.TaskKind            `json:"kind"`
	KindOptions                build.KindOptions         `json:"kindOptions"`
	Runtime                    build.TaskRuntime         `json:"runtime"`
	Repo                       string                    `json:"repo"`
	RequireExplicitPermissions *bool                     `json:"requireExplicitPermissions"`
	Permissions                *Permissions              `json:"permissions"`
	ExecuteRules               UpdateExecuteRulesRequest `json:"executeRules"`
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
	DisallowSelfApprove *bool `json:"disallowSelfApprove"`
	RequireRequests     *bool `json:"requireRequests"`
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

type TaskEnv map[string]EnvVarValue

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
	DisallowSelfApprove bool `json:"disallowSelfApprove"`
	RequireRequests     bool `json:"requireRequests"`
}

type View struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	ArchivedAt  *time.Time `json:"archivedAt"`
	ArchivedBy  *string    `json:"archivedBy"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	CreatedBy   string     `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
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
}

type GetEnvResponse struct {
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
