package api

import (
	"encoding/json"
	"time"

	// Some types are imported from lib. Eventually we might want all of these types to live in lib. For now,
	// we can move tasks from here -> lib on an as-needed basis.
	libapi "github.com/airplanedev/lib/pkg/api"
	buildtypes "github.com/airplanedev/lib/pkg/build/types"
	"github.com/airplanedev/ojson"
)

type GetTaskRequest struct {
	Slug    string
	EnvSlug string
}

type ReviewerID struct {
	UserID  string `json:"userID,omitempty"`
	GroupID string `json:"groupID,omitempty"`
}

type GetTaskReviewersResponse struct {
	Task      *libapi.Task `json:"task"`
	Reviewers []ReviewerID `json:"reviewers"`
}

// CreateTaskRequest creates a new task.
type CreateTaskRequest struct {
	Slug             string                    `json:"slug"`
	Name             string                    `json:"name"`
	Description      string                    `json:"description"`
	Image            *string                   `json:"image"`
	Command          []string                  `json:"command"`
	Arguments        []string                  `json:"arguments"`
	Parameters       libapi.Parameters         `json:"parameters"`
	Configs          []libapi.ConfigAttachment `json:"configs"`
	Constraints      libapi.RunConstraints     `json:"constraints"`
	EnvVars          libapi.TaskEnv            `json:"env"`
	ResourceRequests map[string]string         `json:"resourceRequests"`
	Resources        map[string]string         `json:"resources"`
	Kind             buildtypes.TaskKind       `json:"kind"`
	KindOptions      buildtypes.KindOptions    `json:"kindOptions"`
	Runtime          buildtypes.TaskRuntime    `json:"runtime"`
	Repo             string                    `json:"repo"`
	Timeout          int                       `json:"timeout"`
	EnvSlug          string                    `json:"envSlug"`
}

type UpdateTaskResponse struct {
	TaskRevisionID string `json:"taskRevisionID"`
}

// GetLogsResponse represents a get logs response.
type GetLogsResponse struct {
	RunID         string    `json:"runID"`
	Logs          []LogItem `json:"logs"`
	NextPageToken string    `json:"next_token"`
	PrevPageToken string    `json:"prev_token"`
}

// GetDeploymentLogsResponse represents a get deploy logs response.
type GetDeploymentLogsResponse struct {
	Logs          []LogItem `json:"logs"`
	NextPageToken string    `json:"nextToken"`
	PrevPageToken string    `json:"prevToken"`
}

// Outputs represents outputs.
//
// It has custom UnmarshalJSON/MarshalJSON methods in order to proxy to the underlying
// ojson.Value methods.
type Outputs ojson.Value

func (o *Outputs) UnmarshalJSON(buf []byte) error {
	var v ojson.Value
	if err := json.Unmarshal(buf, &v); err != nil {
		return err
	}

	*o = Outputs(v)
	return nil
}

func (o Outputs) MarshalJSON() ([]byte, error) {
	return json.Marshal(ojson.Value(o))
}

// Represents a line of the output
type OutputRow struct {
	OutputName string      `json:"name" yaml:"name"`
	Value      interface{} `json:"value" yaml:"value"`
}

// GetOutputsResponse represents a get outputs response.
type GetOutputsResponse struct {
	Outputs Outputs `json:"outputs"`
}

type GetRunbookResponse struct {
	Runbook Runbook `json:"runbook"`
}

type Runbook struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Slug            string            `json:"slug"`
	Parameters      libapi.Parameters `json:"parameters"`
	TemplateSession TemplateSession   `json:"templateSession"`
}

type TemplateSession struct {
	ID          string                    `json:"id"`
	Configs     []libapi.ConfigAttachment `json:"configs"`
	Constraints libapi.RunConstraints     `json:"constraints"`
}

type ListSessionBlocksResponse struct {
	Blocks []SessionBlock `json:"blocks"`
}

type BlockKind string

const (
	BlockKindUnknown BlockKind = ""
	BlockKindStdAPI  BlockKind = "stdapi"
	BlockKindTask    BlockKind = "task"
	BlockKindNote    BlockKind = "note"
	BlockKindForm    BlockKind = "form"
)

type SessionBlock struct {
	ID              string          `json:"id"`
	BlockKind       string          `json:"kind"`
	BlockKindConfig BlockKindConfig `json:"kindConfig"`
	StartCondition  string          `json:"startCondition"`
	Slug            string          `json:"slug"`
}

type BlockKindConfig struct {
	StdAPI *BlockKindConfigStdAPI `json:"stdapi,omitempty"`
	Task   *BlockKindConfigTask   `json:"task,omitempty"`
	Note   *BlockKindConfigNote   `json:"note,omitempty"`
	Form   *BlockKindConfigForm   `json:"form,omitempty"`
}

type BlockKindConfigNote struct {
	// Content is only allowed to be either a `string` or `expressions.Template`
	Content interface{} `json:"content"`
}

type BlockKindConfigStdAPI struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Request   interface{}       `json:"request"`
	Resources map[string]string `json:"resources"`
}

type BlockKindConfigTask struct {
	TaskID      string                 `json:"taskID"`
	ParamValues map[string]interface{} `json:"paramValues"`
}

type BlockKindConfigForm struct {
	Parameters libapi.Parameters      `json:"parameters"`
	ParamValue map[string]interface{} `json:"paramValues"`
}

// LogItem represents a log item.
type LogItem struct {
	Timestamp time.Time `json:"timestamp"`
	InsertID  string    `json:"insertID"`
	Text      string    `json:"text"`
	Level     LogLevel  `json:"level"`
	TaskSlug  string    `json:"taskSlug"`
}

type LogLevel string

const (
	LogLevelInfo  LogLevel = "info"
	LogLevelDebug LogLevel = "debug"
)

// RegistryTokenResponse represents a registry token response.
type RegistryTokenResponse struct {
	Token      string `json:"token"`
	Expiration string `json:"expiration"`
	Repo       string `json:"repo"`
}

// AuthInfoResponse represents info about authenticated user.
type AuthInfoResponse struct {
	User *UserInfo `json:"user"`
	Team *TeamInfo `json:"team"`
}

type UserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type TeamInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreateTaskResponse represents a create task response.
type CreateTaskResponse struct {
	TaskID         string `json:"taskID"`
	Slug           string `json:"slug"`
	TaskRevisionID string `json:"taskRevisionID"`
}

// ListTasksResponse represents a list tasks response.
type ListTasksResponse struct {
	Tasks []libapi.Task `json:"tasks"`
}

// Values represent parameters values.
//
// An alias is used because we want the type
// to be `map[string]interface{}` and not a custom one.
//
// They're keyed by the parameter "slug".
type Values = map[string]interface{}

// RunTaskRequest represents a run task request.
type RunTaskRequest struct {
	TaskID      *string `json:"taskID"`
	TaskSlug    *string `json:"slug"`
	ParamValues Values  `json:"paramValues"`
	EnvSlug     string  `json:"envSlug"`
}

// RunTaskResponse represents a run task response.
type RunTaskResponse struct {
	RunID string `json:"runID"`
}

// GetRunResponse represents a get task response.
type GetRunResponse struct {
	Run Run `json:"run"`
}

// RunStatus enumerates run status.
type RunStatus string

// All RunStatus types.
const (
	RunNotStarted RunStatus = "NotStarted"
	RunQueued     RunStatus = "Queued"
	RunActive     RunStatus = "Active"
	RunSucceeded  RunStatus = "Succeeded"
	RunFailed     RunStatus = "Failed"
	RunCancelled  RunStatus = "Cancelled"
)

func (r RunStatus) IsTerminal() bool {
	return r == RunSucceeded || r == RunFailed || r == RunCancelled
}

// Run represents a run.
type Run struct {
	RunID       string             `json:"runID"`
	TaskID      string             `json:"taskID"`
	TaskName    string             `json:"taskName"`
	TeamID      string             `json:"teamID"`
	Status      RunStatus          `json:"status"`
	ParamValues Values             `json:"paramValues"`
	Parameters  *libapi.Parameters `json:"parameters"`
	CreatedAt   time.Time          `json:"createdAt"`
	CreatorID   string             `json:"creatorID"`
	QueuedAt    *time.Time         `json:"queuedAt"`
	ActiveAt    *time.Time         `json:"activeAt"`
	SucceededAt *time.Time         `json:"succeededAt"`
	FailedAt    *time.Time         `json:"failedAt"`
	CancelledAt *time.Time         `json:"cancelledAt"`
	CancelledBy *string            `json:"cancelledBy"`
	EnvSlug     string             `json:"envSlug"`
}

// ListRunsRequest represents a list runs request.
type ListRunsRequest struct {
	TaskID  string    `json:"taskID"`
	Since   time.Time `json:"since"`
	Until   time.Time `json:"until"`
	Page    int       `json:"page"`
	Limit   int       `json:"limit"`
	EnvSlug string    `json:"envSlug"`
}

// ListRunsResponse represents a list runs response.
type ListRunsResponse struct {
	Runs []Run `json:"runs"`
}

// GetConfigRequest represents a get config request
type GetConfigRequest struct {
	Name       string `json:"name"`
	Tag        string `json:"tag"`
	ShowSecret bool   `json:"showSecret"`
	EnvSlug    string `json:"envSlug"`
}

// SetConfigRequest represents a set config request.
type SetConfigRequest struct {
	Name     string `json:"name"`
	Tag      string `json:"tag"`
	Value    string `json:"value"`
	IsSecret bool   `json:"isSecret"`
	EnvSlug  string `json:"envSlug"`
}

// ListConfigsRequest represents a list configs request
type ListConfigsRequest struct {
	Names       []string `json:"names"`
	ShowSecrets bool     `json:"showSecrets"`
	EnvSlug     string   `json:"envSlug"`
}

// ListConfigsResponse represents a list configs response.
type ListConfigsResponse struct {
	Configs []Config `json:"configs"`
}

// Config represents a config var.
type Config struct {
	ID       string `json:"configID"`
	Name     string `json:"name" yaml:"name"`
	Tag      string `json:"tag" yaml:"tag"`
	Value    string `json:"value" yaml:"value"`
	IsSecret bool   `json:"isSecret" yaml:"isSecret"`
}

// GetConfigResponse represents a get config response.
type GetConfigResponse struct {
	Config Config `json:"config"`
}

type CreateAPIKeyRequest struct {
	Name string `json:"name"`
}

type CreateAPIKeyResponse struct {
	APIKey APIKey `json:"apiKey"`
}

type ListAPIKeysResponse struct {
	APIKeys []APIKey `json:"apiKeys"`
}

type DeleteAPIKeyRequest struct {
	KeyID string `json:"keyID"`
}

type APIKey struct {
	ID        string    `json:"id" yaml:"id"`
	TeamID    string    `json:"teamID" yaml:"teamID"`
	Name      string    `json:"name" yaml:"name"`
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`
	Key       string    `json:"key" yaml:"key"`
}

type GetUniqueSlugResponse struct {
	Slug string `json:"slug"`
}

type DeployTask struct {
	TaskID            string                     `json:"taskID"`
	Kind              buildtypes.TaskKind        `json:"kind"`
	BuildConfig       buildtypes.BuildConfig     `json:"buildConfig"`
	UploadID          string                     `json:"uploadID"`
	UpdateTaskRequest libapi.UpdateTaskRequest   `json:"updateTaskRequest"`
	EnvVars           libapi.TaskEnv             `json:"envVars"`
	GitFilePath       string                     `json:"gitFilePath"`
	Schedules         map[string]libapi.Schedule `json:"schedules"`
}

type DeployView struct {
	ID                string                   `json:"id"`
	UploadID          string                   `json:"uploadID"`
	UpdateViewRequest libapi.UpdateViewRequest `json:"updateViewRequest"`
	BuildConfig       buildtypes.BuildConfig   `json:"buildConfig"`
	// Path from the git root to the entrypoint of the app if the app was deployed
	// from a git repository.
	GitFilePath string `json:"gitFilePath"`
}

type BuildContext = buildtypes.BuildContext

type DeployBundle struct {
	UploadID     string       `json:"uploadID"`
	Name         string       `json:"name"`
	TargetFiles  []string     `json:"targetFiles"`
	BuildContext BuildContext `json:"buildContext"`
	GitFilePath  string       `json:"gitFilePath"`
}

type CreateDeploymentRequest struct {
	Tasks       []DeployTask   `json:"tasks"`
	Views       []DeployView   `json:"views"`
	Bundles     []DeployBundle `json:"bundles"`
	GitMetadata GitMetadata    `json:"gitMetadata"`
	EnvSlug     string         `json:"envSlug"`
}

type GenerateSignedURLsResponse struct {
	SignedURLs []string `json:"signedURLs"`
}

type CancelDeploymentRequest struct {
	ID string `json:"id"`
}

type GitMetadata struct {
	CommitHash          string    `json:"commitHash"`
	Ref                 string    `json:"ref"`
	User                string    `json:"user"`
	RepositoryOwnerName string    `json:"repositoryOwnerName"`
	RepositoryName      string    `json:"repositoryName"`
	CommitMessage       string    `json:"commitMessage"`
	Vendor              GitVendor `json:"vendor"`
	IsDirty             bool      `json:"isDirty"`
}

type GitVendor string

const (
	GitVendorGitHub GitVendor = "GitHub"
)

type CreateDeploymentResponse struct {
	Deployment       Deployment `json:"deployment"`
	NumTasksUpdated  int        `json:"numTasksUpdated"`
	NumAppsUpdated   int        `json:"numAppsUpdated"`
	NumBuildsCreated int        `json:"numBuildsCreated"`
}

type Deployment struct {
	ID           string     `json:"id"`
	TeamID       string     `json:"teamID"`
	CreatedAt    time.Time  `json:"createdAt"`
	CreatedBy    string     `json:"createdBy"`
	SucceededAt  *time.Time `json:"succeededAt,omitempty"`
	CancelledAt  *time.Time `json:"cancelledAt,omitempty"`
	FailedAt     *time.Time `json:"failedAt,omitempty"`
	FailedReason string     `json:"failedReason,omitempty"`
}

type App struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	ArchivedAt  *time.Time `json:"archivedAt"`
	ArchivedBy  *string    `json:"archivedBy"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	CreatedBy   string     `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type CreateDemoDBRequest struct {
	Name string `json:"name"`
}

type GetResourceRequest struct {
	ID                   string `json:"id"`
	Slug                 string `json:"slug"`
	EnvSlug              string `json:"envSlug"`
	IncludeSensitiveData bool   `json:"includeSensitiveData"`
}

// TODO: shift to lib?
type GetPermissionsResponse struct {
	// One true/false value for each requested action.
	Outputs map[string]bool `json:"resource"`
}

type ListFlagsResponse struct {
	Flags map[string]string `json:"flags"`
}

type User struct {
	ID        string  `json:"userID" db:"id"`
	Email     string  `json:"email" db:"email"`
	Name      string  `json:"name" db:"name"`
	AvatarURL *string `json:"avatarURL" db:"avatar_url"`
}

type GetUserResponse struct {
	User User `json:"user"`
}

type GetTunnelTokenResponse struct {
	Token string `json:"token"`
}

type SetDevSecretRequest struct {
	Token string `json:"token"`
}

type CreateSandboxRequest struct {
	Namespace *string `json:"namespace"`
	Key       *string `json:"key"`
}

type CreateSandboxResponse struct {
	Token string `json:"token"`
}

type ListEnvsResponse struct {
	Envs []libapi.Env `json:"envs"`
}

// TODO: Move autocomplete types to lib

type CompletionType string

const (
	SQLCompletionType           CompletionType = "sql"
	TaskYAMLCompletionType      CompletionType = "task-yaml"
	TaskInlineCompletionType    CompletionType = "task-inline"
	ViewComponentCompletionType CompletionType = "view-component"
)

type AutopilotCompleteRequest struct {
	Type    CompletionType   `json:"type"`
	Prompt  string           `json:"prompt"`
	Context *CompleteContext `json:"context"`
}

type AutopilotCompleteResponse struct {
	Content string `json:"content"`
}

type CompleteContext struct {
	CompleteSQLContext           *CompleteSQLContext           `json:"sql"`
	CompleteTaskYAMLContext      *CompleteTaskYAMLContext      `json:"taskYAML"`
	CompleteTaskInlineContext    *CompleteTaskInlineContext    `json:"taskInline"`
	CompleteViewComponentContext *CompleteViewComponentContext `json:"viewComponent"`
}

type CompleteSQLContext struct {
	ResourceID string `json:"resourceID"`
}

type CompleteTaskYAMLContext struct {
	Kind buildtypes.TaskKind `json:"kind"`
}

type CompleteTaskInlineContext struct {
	Kind buildtypes.TaskKind `json:"kind"`
}

type ViewComponentKind string

const (
	ViewComponentKindChart ViewComponentKind = "chart"
	ViewComponentKindForm  ViewComponentKind = "form"
	ViewComponentKindTable ViewComponentKind = "table"
)

type CompleteViewComponentContext struct {
	Kind ViewComponentKind `json:"kind"`
}
