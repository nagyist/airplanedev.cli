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
}

type View struct {
	ID          string            `json:"id"`
	Slug        string            `json:"slug"`
	ArchivedAt  *time.Time        `json:"archivedAt"`
	ArchivedBy  *string           `json:"archivedBy"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	CreatedBy   string            `json:"createdBy"`
	CreatedAt   time.Time         `json:"createdAt"`
	EnvVars     map[string]string `json:"envVars"`
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
