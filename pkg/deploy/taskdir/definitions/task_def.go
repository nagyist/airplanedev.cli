package definitions

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"strings"
	"text/template"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/utils/pointers"
	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

type Definition struct {
	Name        string                `json:"name"`
	Slug        string                `json:"slug"`
	Description string                `json:"description,omitempty"`
	Parameters  []ParameterDefinition `json:"parameters,omitempty"`
	Resources   ResourceDefinition    `json:"resources,omitempty"`

	Image  *ImageDefinition  `json:"docker,omitempty"`
	Node   *NodeDefinition   `json:"node,omitempty"`
	Python *PythonDefinition `json:"python,omitempty"`
	Shell  *ShellDefinition  `json:"shell,omitempty"`

	SQL     *SQLDefinition        `json:"sql,omitempty"`
	REST    *RESTDefinition       `json:"rest,omitempty"`
	Builtin *BuiltinTaskContainer `json:",inline,omitempty"`

	Configs            []string                 `json:"configs,omitempty"`
	Constraints        map[string]string        `json:"constraints,omitempty"`
	RequireRequests    bool                     `json:"requireRequests,omitempty"`
	AllowSelfApprovals DefaultTrueDefinition    `json:"allowSelfApprovals,omitempty"`
	RestrictCallers    []string                 `json:"restrictCallers,omitempty"`
	Timeout            DefaultTimeoutDefinition `json:"timeout,omitempty"`
	Runtime            build.TaskRuntime        `json:"runtime,omitempty"`

	Schedules map[string]ScheduleDefinition `json:"schedules,omitempty"`

	buildConfig  build.BuildConfig
	defnFilePath string
}

type taskKind interface {
	copyToTask(*api.Task, build.BuildConfig, GetTaskOpts) error
	hydrateFromTask(api.Task, []api.ResourceMetadata) error
	setEntrypoint(string) error
	setAbsoluteEntrypoint(string) error
	getAbsoluteEntrypoint() (string, error)
	getKindOptions() (build.KindOptions, error)
	getEntrypoint() (string, error)
	getEnv() (api.TaskEnv, error)
	setEnv(api.TaskEnv) error
	getConfigAttachments() []api.ConfigAttachment
	getResourceAttachments() map[string]string
	getBuildType() (build.BuildType, build.BuildTypeVersion, build.BuildBase)
	SetBuildVersionBase(build.BuildTypeVersion, build.BuildBase)
}

type ParameterDefinition struct {
	Name        string                `json:"name"`
	Slug        string                `json:"slug"`
	Type        string                `json:"type"`
	Description string                `json:"description,omitempty"`
	Default     interface{}           `json:"default,omitempty"`
	Required    DefaultTrueDefinition `json:"required,omitempty"`
	Options     []OptionDefinition    `json:"options,omitempty"`
	Regex       string                `json:"regex,omitempty"`
}

type OptionDefinition struct {
	Label  string      `json:"label"`
	Value  interface{} `json:"value,omitempty"`
	Config *string     `json:"config,omitempty"`
}

var _ json.Unmarshaler = &OptionDefinition{}

func (o *OptionDefinition) UnmarshalJSON(b []byte) error {
	// If it's just a string, dump it in the value field.
	var value string
	if err := json.Unmarshal(b, &value); err == nil {
		o.Value = value
		return nil
	}

	// Otherwise, perform a normal unmarshal operation.
	// Note we need a new type, otherwise we recursively call this
	// method and end up stack overflowing.
	type option OptionDefinition
	var opt option
	if err := json.Unmarshal(b, &opt); err != nil {
		return err
	}
	*o = OptionDefinition(opt)

	return nil
}

type ScheduleDefinition struct {
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	CronExpr    string                 `json:"cron"`
	ParamValues map[string]interface{} `json:"paramValues,omitempty"`
}

// SchemaStore must be updated if this file is moved or renamed.
// https://github.com/SchemaStore/schemastore/blob/b4eccc2fb5ad76fd9c0a70fa67228e5d65e2b562/src/api/json/catalog.json#L84
//
//go:embed schema_0_3.json
var schemaStr string

func NewDefinition(name string, slug string, kind build.TaskKind, entrypoint string) (Definition, error) {
	def := Definition{
		Name: name,
		Slug: slug,
	}

	switch kind {
	case build.TaskKindImage:
		def.Image = &ImageDefinition{
			Image:   "alpine:3",
			Command: `echo "hello world"`,
		}
	case build.TaskKindNode:
		def.Node = &NodeDefinition{
			Entrypoint:  entrypoint,
			NodeVersion: string(build.DefaultNodeVersion),
		}
	case build.TaskKindPython:
		def.Python = &PythonDefinition{
			Entrypoint: entrypoint,
		}
	case build.TaskKindShell:
		def.Shell = &ShellDefinition{
			Entrypoint: entrypoint,
		}
	case build.TaskKindSQL:
		def.SQL = &SQLDefinition{
			Entrypoint: entrypoint,
		}
	case build.TaskKindREST:
		def.REST = &RESTDefinition{
			Method:   "POST",
			Path:     "/",
			BodyType: "json",
			Body:     "{}",
		}
	case build.TaskKindBuiltin:
		return Definition{}, errors.New("use NewBuiltinDefinition instead")
	default:
		return Definition{}, errors.Errorf("unknown kind: %s", kind)
	}

	return def, nil
}

func NewBuiltinDefinition(name string, slug string, builtin BuiltinTaskDef) (Definition, error) {
	return Definition{
		Name:    name,
		Slug:    slug,
		Builtin: &BuiltinTaskContainer{def: builtin},
	}, nil
}

// Customize the UnmarshalJSON to pull out the builtin, if there is any. The MarshalJSON
// customization is done on the BuiltinTaskContainer (as this field is inlined).
func (d *Definition) UnmarshalJSON(b []byte) error {
	// Perform a normal unmarshal operation.
	// Note we need a new type, otherwise we recursively call this
	// method and end up stack overflowing.
	type definition Definition
	var def definition
	if err := json.Unmarshal(b, &def); err != nil {
		return err
	}
	*d = Definition(def)

	// Unmarshal it into a map.
	var serialized map[string]interface{}
	if err := json.Unmarshal(b, &serialized); err != nil {
		return err
	}

	// Is there a builtin somewhere?
	for key, plugin := range builtinTaskPluginsByDefinitionKey {
		defMap, ok := serialized[key]
		if !ok {
			continue
		}
		defBytes, err := json.Marshal(defMap)
		if err != nil {
			return err
		}
		kind := plugin.GetTaskKindDefinition()
		if err := json.Unmarshal(defBytes, &kind); err != nil {
			return err
		}
		d.Builtin = &BuiltinTaskContainer{def: kind}
		break
	}

	return nil
}

// Marshal returns a serialized version of the definition in the given format.
func (d Definition) Marshal(format DefFormat) ([]byte, error) {
	switch format {
	case DefFormatYAML:
		// Use the JSON marshaler so we use MarshalJSON methods.
		buf, err := yaml.MarshalWithOptions(d,
			yaml.UseJSONMarshaler(),
			yaml.UseLiteralStyleIfMultiline(true))
		if err != nil {
			return nil, err
		}
		return buf, nil

	case DefFormatJSON:
		// Use the YAML marshaler so we can take advantage of the yaml.IsZeroer check on omitempty.
		// But make it use the JSON marshaler so we use MarshalJSON methods.
		buf, err := yaml.MarshalWithOptions(d,
			yaml.UseJSONMarshaler(),
			yaml.JSON())
		if err != nil {
			return nil, err
		}
		// `yaml.Marshal` doesn't allow configuring JSON indentation, so do it after the fact.
		var out bytes.Buffer
		if err := json.Indent(&out, buf, "", "\t"); err != nil {
			return nil, err
		}
		return out.Bytes(), nil

	default:
		return nil, errors.Errorf("unknown format: %s", format)
	}
}

// GenerateCommentedFile generates a commented YAML file under certain circumstances. If the format
// requested isn't YAML, or if the definition has other things filled in, this method defaults to
// calling Marshal(format).
func (d Definition) GenerateCommentedFile(format DefFormat) ([]byte, error) {
	// If it's not YAML, or you have other things defined on your task def, bail.
	if format != DefFormatYAML ||
		d.Description != "" ||
		len(d.Parameters) > 0 ||
		len(d.Resources.Attachments) > 0 ||
		len(d.Constraints) > 0 ||
		d.RequireRequests ||
		!d.AllowSelfApprovals.IsZero() ||
		!d.Timeout.IsZero() ||
		d.Builtin != nil {
		return d.Marshal(format)
	}

	kind, err := d.Kind()
	if err != nil {
		return nil, err
	}

	taskDefinition := new(bytes.Buffer)
	var paramsExtraInfo string
	switch kind {
	case build.TaskKindImage:
		if d.Image.Entrypoint != "" || len(d.Image.EnvVars) > 0 {
			return d.Marshal(format)
		}
		tmpl, err := template.New("image").Parse(imageTemplate)
		if err != nil {
			return nil, errors.Wrap(err, "parsing image template")
		}
		if err := tmpl.Execute(taskDefinition, d.Image); err != nil {
			return nil, errors.Wrap(err, "executing image template")
		}
		paramsExtraInfo = imageParamsExtraDescription
	case build.TaskKindNode:
		if len(d.Node.EnvVars) > 0 {
			return d.Marshal(format)
		}
		tmpl, err := template.New("node").Parse(nodeTemplate)
		if err != nil {
			return nil, errors.Wrap(err, "parsing node template")
		}
		if err := tmpl.Execute(taskDefinition, d.Node); err != nil {
			return nil, errors.Wrap(err, "executing node template")
		}
	case build.TaskKindPython:
		if len(d.Python.EnvVars) > 0 {
			return d.Marshal(format)
		}
		tmpl, err := template.New("python").Parse(pythonTemplate)
		if err != nil {
			return nil, errors.Wrap(err, "parsing python template")
		}
		if err := tmpl.Execute(taskDefinition, d.Python); err != nil {
			return nil, errors.Wrap(err, "executing python template")
		}
	case build.TaskKindShell:
		if len(d.Shell.EnvVars) > 0 {
			return d.Marshal(format)
		}
		tmpl, err := template.New("shell").Parse(shellTemplate)
		if err != nil {
			return nil, errors.Wrap(err, "parsing shell template")
		}
		if err := tmpl.Execute(taskDefinition, d.Shell); err != nil {
			return nil, errors.Wrap(err, "executing shell template")
		}
		paramsExtraInfo = shellParamsExtraDescription
	case build.TaskKindSQL:
		if d.SQL.Resource != "" || len(d.SQL.QueryArgs) > 0 {
			return d.Marshal(format)
		}
		tmpl, err := template.New("sql").Parse(sqlTemplate)
		if err != nil {
			return nil, errors.Wrap(err, "parsing SQL template")
		}
		if err := tmpl.Execute(taskDefinition, d.SQL); err != nil {
			return nil, errors.Wrap(err, "executing sql template")
		}
	case build.TaskKindREST:
		if d.REST.Resource != "" ||
			len(d.REST.URLParams) > 0 ||
			len(d.REST.Headers) > 0 ||
			len(d.REST.FormData) > 0 {
			return d.Marshal(format)
		}
		tmpl, err := template.New("rest").Parse(restTemplate)
		if err != nil {
			return nil, errors.Wrap(err, "parsing REST template")
		}
		if err := tmpl.Execute(taskDefinition, d.REST); err != nil {
			return nil, errors.Wrap(err, "executing rest template")
		}
	default:
		return d.Marshal(format)
	}

	// Remove any newlines from the name & run yaml.Marshal to take care of any weird characters.
	nameBuf, err := yaml.Marshal(strings.ReplaceAll(d.Name, "\n", ""))
	if err != nil {
		return nil, errors.Wrap(err, "marshalling name")
	}
	// yaml.Marshal always appends a newline, trim it.
	name := strings.TrimSuffix(string(nameBuf), "\n")

	tmpl, err := template.New("definition").Parse(definitionTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "parsing definition template")
	}
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, map[string]interface{}{
		"slug":                   d.Slug,
		"name":                   name,
		"taskDefinition":         taskDefinition.String(),
		"paramsExtraDescription": paramsExtraInfo,
	}); err != nil {
		return nil, errors.Wrap(err, "executing definition template")
	}
	return buf.Bytes(), nil
}

func (d *Definition) Unmarshal(format DefFormat, buf []byte) error {
	var err error
	switch format {
	case DefFormatYAML:
		buf, err = yaml.YAMLToJSON(buf)
		if err != nil {
			return err
		}
	case DefFormatJSON:
		// nothing
	default:
		return errors.Errorf("unknown format: %s", format)
	}

	schemaLoader := gojsonschema.NewStringLoader(schemaStr)
	docLoader := gojsonschema.NewBytesLoader(buf)

	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return errors.Wrap(err, "validating schema")
	}

	if !result.Valid() {
		return errors.WithStack(ErrSchemaValidation{Errors: result.Errors()})
	}

	if err = json.Unmarshal(buf, &d); err != nil {
		return err
	}
	return nil
}

// Normalize is a chance to rewrite the definition to account for changes in formatting after
// being unmarshalled. This can result in multiple API calls & is not always needed & so is not
// lumped in with Unmarshal.
func (d *Definition) Normalize(availableResources []api.ResourceMetadata) error {
	// Rewrites Resource to be a slug rather than a name.
	if d.SQL != nil {
		return d.SQL.normalize(availableResources)
	} else if d.REST != nil {
		return d.REST.normalize(availableResources)
	}
	return nil
}

// SetAbsoluteEntrypoint sets the absolute entrypoint for this definition. Does not change the
// result of calling Entrypoint(). Returns ErrNoEntrypoint if the task kind definition requires
// no entrypoint.
func (d *Definition) SetAbsoluteEntrypoint(entrypoint string) error {
	taskKind, err := d.taskKind()
	if err != nil {
		return err
	}

	return taskKind.setAbsoluteEntrypoint(entrypoint)
}

// GetAbsoluteEntrypoint gets the absolute entrypoint for this definition. Returns
// ErrNoEntrypoint if the task kind definition requires no entrypoint. If SetAbsoluteEntrypoint
// has not been set, returns ErrNoAbsoluteEntrypoint.
func (d *Definition) GetAbsoluteEntrypoint() (string, error) {
	taskKind, err := d.taskKind()
	if err != nil {
		return "", err
	}

	return taskKind.getAbsoluteEntrypoint()
}

func (d Definition) Kind() (build.TaskKind, error) {
	if d.Image != nil {
		return build.TaskKindImage, nil
	} else if d.Node != nil {
		return build.TaskKindNode, nil
	} else if d.Python != nil {
		return build.TaskKindPython, nil
	} else if d.Shell != nil {
		return build.TaskKindShell, nil
	} else if d.SQL != nil {
		return build.TaskKindSQL, nil
	} else if d.REST != nil {
		return build.TaskKindREST, nil
	} else if d.Builtin != nil {
		return build.TaskKindBuiltin, nil
	} else {
		return "", errors.New("incomplete task definition")
	}
}

func (d Definition) taskKind() (taskKind, error) {
	if d.Image != nil {
		return d.Image, nil
	} else if d.Node != nil {
		return d.Node, nil
	} else if d.Python != nil {
		return d.Python, nil
	} else if d.Shell != nil {
		return d.Shell, nil
	} else if d.SQL != nil {
		return d.SQL, nil
	} else if d.REST != nil {
		return d.REST, nil
	} else if d.Builtin != nil {
		return d.Builtin.def, nil
	} else {
		return nil, errors.New("incomplete task definition")
	}
}

type GetTaskOpts struct {
	// List of resources that this task can attach. This is used for converting
	// resource slugs to IDs.
	AvailableResources []api.ResourceMetadata
	// Set to `true` if this task is using bundle builds.
	Bundle bool
	// Set to `true` to silently ignore invalid definition fields.
	IgnoreInvalid bool
}

// GetTask converts a task definition into a Task struct.
//
// Note that certain fields are not supported "as-code", e.g. permissions. Those fields
// will not be set on the task.
func (d Definition) GetTask(opts GetTaskOpts) (api.Task, error) {
	task := api.Task{
		Slug:        d.Slug,
		Name:        d.Name,
		Description: d.Description,
		Timeout:     d.Timeout.Value(),
		Runtime:     d.Runtime,
		ExecuteRules: api.ExecuteRules{
			RequireRequests:     d.RequireRequests,
			DisallowSelfApprove: !d.AllowSelfApprovals.Value(),
			RestrictCallers:     d.RestrictCallers,
		},
		Resources: api.Resources{},
		Configs:   []api.ConfigAttachment{},
		Constraints: api.RunConstraints{
			Labels: []api.AgentLabel{},
		},
	}

	params, err := d.GetParameters()
	if err != nil {
		return api.Task{}, err
	}
	task.Parameters = params

	if err := d.addResourcesToTask(&task, opts); err != nil {
		return api.Task{}, err
	}

	for _, configName := range d.Configs {
		task.Configs = append(task.Configs, api.ConfigAttachment{NameTag: configName})
	}

	for key, val := range d.Constraints {
		task.Constraints.Labels = append(task.Constraints.Labels, api.AgentLabel{
			Key:   key,
			Value: val,
		})
	}

	bc, err := d.GetBuildConfig()
	if err != nil {
		return api.Task{}, err
	}
	if err := d.addKindSpecificsToTask(&task, bc, opts); err != nil {
		return api.Task{}, err
	}

	return task, nil
}

func (d Definition) addResourcesToTask(task *api.Task, opts GetTaskOpts) error {
	if len(d.Resources.Attachments) == 0 {
		return nil
	}

	for alias, slug := range d.Resources.Attachments {
		if resource := getResourceBySlug(opts.AvailableResources, slug); resource != nil {
			task.Resources[alias] = resource.ID
		} else if !opts.IgnoreInvalid {
			return api.ResourceMissingError{Slug: slug}
		}
	}

	return nil
}

func (d Definition) addKindSpecificsToTask(task *api.Task, bc build.BuildConfig, opts GetTaskOpts) error {
	kind, options, err := d.GetKindAndOptions()
	if err != nil {
		return err
	}
	task.Kind = kind
	task.KindOptions = options

	env, err := d.GetEnv()
	if err != nil {
		return err
	}
	task.Env = env

	task.Configs, err = d.GetConfigAttachments()
	if err != nil {
		return err
	}

	taskKind, err := d.taskKind()
	if err != nil {
		return err
	}
	if err := taskKind.copyToTask(task, bc, opts); err != nil {
		return err
	}
	return nil
}

// Entrypoint returns ErrNoEntrypoint if the task kind definition requires no entrypoint. May be
// empty. May be absolute or relative; if relative, it is relative to the defn file.
func (d Definition) Entrypoint() (string, error) {
	taskKind, err := d.taskKind()
	if err != nil {
		return "", err
	}
	return taskKind.getEntrypoint()
}

// GetDefnFilePath returns the absolute path to the file that configured this definition, if one exists.
func (d Definition) GetDefnFilePath() string {
	return d.defnFilePath
}

func (d Definition) GetDescription() string {
	return d.Description
}

func (d Definition) GetParameters() (api.Parameters, error) {
	return convertParametersDefToAPI(d.Parameters)
}

func (d Definition) GetBuildType() (build.BuildType, build.BuildTypeVersion, build.BuildBase, error) {
	taskKind, err := d.taskKind()
	if err != nil {
		return "", "", "", err
	}
	t, v, b := taskKind.getBuildType()
	return t, v, b, nil
}

// SetBuildVersionBase sets the version and base that this definition should be built with. Does not
// override the version or base if it was already set.
func (d Definition) SetBuildVersionBase(v build.BuildTypeVersion, b build.BuildBase) error {
	taskKind, err := d.taskKind()
	if err != nil {
		return err
	}
	taskKind.SetBuildVersionBase(v, b)
	return nil
}

func (d *Definition) SetDefnFilePath(filePath string) {
	d.defnFilePath = filePath
}

func (d *Definition) UpgradeJST() error {
	return nil
}

func (d *Definition) GetKindAndOptions() (build.TaskKind, build.KindOptions, error) {
	kind, err := d.Kind()
	if err != nil {
		return "", nil, err
	}

	taskKind, err := d.taskKind()
	if err != nil {
		return "", nil, err
	}

	options, err := taskKind.getKindOptions()
	if err != nil {
		return "", nil, err
	}

	return kind, options, nil
}

func (d *Definition) GetEnv() (api.TaskEnv, error) {
	taskKind, err := d.taskKind()
	if err != nil {
		return nil, err
	}
	return taskKind.getEnv()
}

func (d *Definition) SetEnv(e api.TaskEnv) error {
	taskKind, err := d.taskKind()
	if err != nil {
		return err
	}
	return taskKind.setEnv(e)
}

func (d *Definition) GetConfigAttachments() ([]api.ConfigAttachment, error) {
	taskKind, err := d.taskKind()
	if err != nil {
		return nil, err
	}
	return taskKind.getConfigAttachments(), nil
}

func (d *Definition) GetResourceAttachments() (map[string]string, error) {
	taskKind, err := d.taskKind()
	if err != nil {
		return nil, err
	}

	taskKindResourceAttachments := taskKind.getResourceAttachments()
	resourceAttachments := make(map[string]string, len(d.Resources.Attachments)+len(taskKindResourceAttachments))
	// Append explicit resource attachments.
	for alias, id := range d.Resources.Attachments {
		resourceAttachments[alias] = id
	}

	// Append kind-specific resource attachments - these override any explicit resource attachments above
	for alias, id := range taskKindResourceAttachments {
		resourceAttachments[alias] = id
	}

	return resourceAttachments, nil
}

func (d *Definition) GetSlug() string {
	return d.Slug
}

func (d *Definition) GetName() string {
	return d.Name
}

func (d *Definition) GetRuntime() build.TaskRuntime {
	return d.Runtime
}

func (d *Definition) SetEntrypoint(entrypoint string) error {
	taskKind, err := d.taskKind()
	if err != nil {
		return err
	}

	return taskKind.setEntrypoint(entrypoint)
}

func (d *Definition) SetWorkdir(taskroot, workdir string) error {
	// TODO: currently only a concept on Node - should be generalized to all builders.
	if d.Node == nil {
		return nil
	}

	d.SetBuildConfig("workdir", strings.TrimPrefix(workdir, taskroot))

	return nil
}

func (d *Definition) GetSchedules() map[string]api.Schedule {
	if len(d.Schedules) == 0 {
		return nil
	}

	schedules := make(map[string]api.Schedule)
	for slug, def := range d.Schedules {
		schedules[slug] = api.Schedule{
			Name:        def.Name,
			Description: def.Description,
			CronExpr:    def.CronExpr,
			ParamValues: def.ParamValues,
		}
	}
	return schedules
}

func NewDefinitionFromTask(t api.Task, availableResources []api.ResourceMetadata) (Definition, error) {
	d := Definition{
		Name:            t.Name,
		Slug:            t.Slug,
		Description:     t.Description,
		RequireRequests: t.ExecuteRules.RequireRequests,
		RestrictCallers: t.ExecuteRules.RestrictCallers,
		Runtime:         t.Runtime,
	}

	params, err := convertParametersAPIToDef(t.Parameters)
	if err != nil {
		return Definition{}, err
	}
	d.Parameters = params

	if err := d.convertResourcesFromTask(t, availableResources); err != nil {
		return Definition{}, err
	}

	if err := d.convertTaskKindFromTask(t, availableResources); err != nil {
		return Definition{}, err
	}

	if len(t.Configs) > 0 {
		d.Configs = make([]string, len(t.Configs))
		for idx, config := range t.Configs {
			d.Configs[idx] = config.NameTag
		}
	}

	if !t.Constraints.IsEmpty() {
		d.Constraints = map[string]string{}
		for _, label := range t.Constraints.Labels {
			d.Constraints[label.Key] = label.Value
		}
	}

	d.AllowSelfApprovals.value = pointers.Bool(!t.ExecuteRules.DisallowSelfApprove)
	d.Timeout.value = t.Timeout

	schedules := make(map[string]ScheduleDefinition)
	for _, trigger := range t.Triggers {
		if trigger.Kind != api.TriggerKindSchedule || trigger.Slug == nil {
			// Trigger is not a schedule deployed via code
			continue
		}
		if trigger.ArchivedAt != nil || trigger.DisabledAt != nil {
			// Trigger is archived or disabled, so don't add to task defn file
			continue
		}

		schedules[*trigger.Slug] = ScheduleDefinition{
			Name:        trigger.Name,
			Description: trigger.Description,
			CronExpr:    trigger.KindConfig.Schedule.CronExpr.String(),
			ParamValues: trigger.KindConfig.Schedule.ParamValues,
		}
	}
	if len(schedules) > 0 {
		d.Schedules = schedules
	}

	return d, nil
}

func (d *Definition) convertResourcesFromTask(t api.Task, availableResources []api.ResourceMetadata) error {
	if len(t.Resources) == 0 {
		return nil
	}

	d.Resources.Attachments = map[string]string{}
	for alias, id := range t.Resources {
		// Ignore SQL/REST resources; they get routed elsewhere.
		if (t.Kind == build.TaskKindSQL && alias == "db") ||
			(t.Kind == build.TaskKindREST && alias == "rest") ||
			(t.Kind == build.TaskKindBuiltin) {
			continue
		}
		if resource := getResourceByID(availableResources, id); resource != nil {
			d.Resources.Attachments[alias] = resource.Slug
		}
	}

	return nil
}

func (d *Definition) convertTaskKindFromTask(t api.Task, availableResources []api.ResourceMetadata) error {
	switch t.Kind {
	case build.TaskKindImage:
		d.Image = &ImageDefinition{}
		return d.Image.hydrateFromTask(t, availableResources)
	case build.TaskKindNode:
		d.Node = &NodeDefinition{}
		return d.Node.hydrateFromTask(t, availableResources)
	case build.TaskKindPython:
		d.Python = &PythonDefinition{}
		return d.Python.hydrateFromTask(t, availableResources)
	case build.TaskKindShell:
		d.Shell = &ShellDefinition{}
		return d.Shell.hydrateFromTask(t, availableResources)
	case build.TaskKindSQL:
		d.SQL = &SQLDefinition{}
		return d.SQL.hydrateFromTask(t, availableResources)
	case build.TaskKindREST:
		d.REST = &RESTDefinition{}
		return d.REST.hydrateFromTask(t, availableResources)
	case build.TaskKindBuiltin:
		return hydrateBuiltin(d, t, availableResources)
	default:
		return errors.Errorf("unknown task kind: %s", t.Kind)
	}
}

// GetBuildConfig gets the full build config, synthesized from KindOptions and explicitly set
// BuildConfig. KindOptions are unioned with BuildConfig; non-nil values in BuildConfig take
// precedence, and a nil BuildConfig value removes the key from the final build config.
func (d *Definition) GetBuildConfig() (build.BuildConfig, error) {
	config := build.BuildConfig{}

	_, options, err := d.GetKindAndOptions()
	if err != nil {
		return nil, err
	}
	for key, val := range options {
		config[key] = val
	}

	// Pass runtime through to builder
	config["runtime"] = d.Runtime

	for key, val := range d.buildConfig {
		if val == nil { // Nil masks out the value.
			delete(config, key)
		} else {
			config[key] = val
		}
	}

	return config, nil
}

// SetBuildConfig sets a build config option. A value of nil means that the key will be
// excluded from GetBuildConfig; used to mask values that exist in KindOptions.
func (d *Definition) SetBuildConfig(key string, value interface{}) {
	if d.buildConfig == nil {
		d.buildConfig = map[string]interface{}{}
	}
	d.buildConfig[key] = value
}

type ResourceDefinition struct {
	Attachments map[string]string
}

func (r *ResourceDefinition) UnmarshalJSON(b []byte) error {
	// If it's just a map, dump it in the Attachments field.
	if err := json.Unmarshal(b, &r.Attachments); err == nil {
		return nil
	}

	// Otherwise, expect a list.
	var list []interface{}
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}

	r.Attachments = make(map[string]string)
	for _, item := range list {
		if s, ok := item.(string); ok {
			if _, exists := r.Attachments[s]; exists {
				return errors.New("aliases in resource list must be unique")
			}
			r.Attachments[s] = s
		} else {
			return errors.New("expected string in resource list")
		}
	}
	return nil
}

func (r ResourceDefinition) MarshalJSON() ([]byte, error) {
	// Return a list if we can.
	var slugs []string
	for alias, slug := range r.Attachments {
		// If we have a single case of alias != slug, just return the map.
		if alias != slug {
			return json.Marshal(r.Attachments)
		}
		slugs = append(slugs, slug)
	}
	return json.Marshal(slugs)
}

// MarshalYAML adds custom logic for marshaling a resource definition into YAML. There seems to be a bug with the
// go-yaml package and marshaling maps using MarshalJSON, which is why we need to include MarshalYAML as well
// (even though we useJSONMarshaler above). If we rely solely on MarshalJSON, it will marshal the resource attachments
// at the top level, e.g.
//
// resources:
// demo: db
//
// as opposed to the correct YAML:
//
// resources:
//
//	demo: db
func (r ResourceDefinition) MarshalYAML() (interface{}, error) {
	// Return a list if we can.
	var slugs []string
	for alias, slug := range r.Attachments {
		// If we have a single case of alias != slug, just return the map.
		if alias != slug {
			return r.Attachments, nil
		}
		slugs = append(slugs, slug)
	}
	return slugs, nil
}

func (r ResourceDefinition) IsZero() bool {
	return len(r.Attachments) == 0
}

// Looks up a resource from `resources`, matching on ID. If no match is found, nil is returned.
func getResourceByID(resources []api.ResourceMetadata, id string) *api.ResourceMetadata {
	for i, resource := range resources {
		if resource.ID == id {
			return &resources[i]
		}
	}
	return nil
}

// Looks up a resource from `resources`, matching on slug. If no match is found, nil is returned.
func getResourceBySlug(resources []api.ResourceMetadata, slug string) *api.ResourceMetadata {
	for i, resource := range resources {
		if resource.Slug == slug {
			return &resources[i]
		}
	}
	return nil
}

// Looks up a resource from `resources`, matching on name. If no match is found, nil is returned.
func getResourceByName(resources []api.ResourceMetadata, name string) *api.ResourceMetadata {
	for i, resource := range resources {
		if resource.DefaultEnvResource != nil && resource.DefaultEnvResource.Name == name {
			return &resources[i]
		}
	}
	return nil
}
