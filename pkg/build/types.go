package build

// KindOptions are part of the task definition, supplied by the user.
type KindOptions map[string]interface{}

// BuildConfig is a collection of build-specific configuration options based on a task's
// KindOptions.
type BuildConfig map[string]interface{}

type TaskKind string

const (
	TaskKindImage  TaskKind = "image"
	TaskKindNode   TaskKind = "node"
	TaskKindPython TaskKind = "python"
	TaskKindShell  TaskKind = "shell"
	TaskKindApp    TaskKind = "app"

	TaskKindSQL  TaskKind = "sql"
	TaskKindREST TaskKind = "rest"
)

type BuildType string

const (
	NodeBuildType   BuildType = "node"
	PythonBuildType BuildType = "python"
	DockerBuildType BuildType = "docker"
	ShellBuildType  BuildType = "shell"
	// NoneBuildType indicates that the entity should not be built.
	NoneBuildType BuildType = "none"
)

type BuildTypeVersion string

const (
	BuildTypeVersionNode14 BuildTypeVersion = "14"
	BuildTypeVersionNode16 BuildTypeVersion = "16"
	BuildTypeVersionNode18 BuildTypeVersion = "18"

	BuildTypeVersionPython37  BuildTypeVersion = "3.7"
	BuildTypeVersionPython38  BuildTypeVersion = "3.8"
	BuildTypeVersionPython39  BuildTypeVersion = "3.9"
	BuildTypeVersionPython310 BuildTypeVersion = "3.10"
	// BuildTypeVersionUnspecified indicates either that a build type does not apply
	// or that the build type should be chosen by the consumer.
	BuildTypeVersionUnspecified BuildTypeVersion = ""
)

type TaskRuntime string

const (
	TaskRuntimeStandard TaskRuntime = ""
	TaskRuntimeWorkflow TaskRuntime = "workflow"
)

// Value represents a value.
type Value interface{}

// Values represent parameters values.
//
// An alias is used because we want the type
// to be `map[string]interface{}` and not a custom one.
//
// They're keyed by the parameter "slug".
type Values = map[string]interface{}
