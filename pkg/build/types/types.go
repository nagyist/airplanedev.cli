package types

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/airplanedev/cli/pkg/build/utils"
	"golang.org/x/exp/slices"
)

// KindOptions are part of the task definition, supplied by the user.
type KindOptions map[string]interface{}

// BuildConfig is a collection of build-specific configuration options based on a task's
// KindOptions.
type BuildConfig map[string]interface{}

// Moving forward, opt to use BuildType instead
type TaskKind string

const (
	TaskKindImage  TaskKind = "image"
	TaskKindNode   TaskKind = "node"
	TaskKindPython TaskKind = "python"
	TaskKindShell  TaskKind = "shell"
	TaskKindApp    TaskKind = "app"

	TaskKindSQL     TaskKind = "sql"
	TaskKindREST    TaskKind = "rest"
	TaskKindBuiltin TaskKind = "builtin"
)

type UserFriendlyTaskKind string

const (
	UserFriendlyTaskKindImage  UserFriendlyTaskKind = "Docker"
	UserFriendlyTaskKindNode   UserFriendlyTaskKind = "JavaScript"
	UserFriendlyTaskKindPython UserFriendlyTaskKind = "Python"
	UserFriendlyTaskKindShell  UserFriendlyTaskKind = "Shell"

	UserFriendlyTaskKindSQL  UserFriendlyTaskKind = "SQL"
	UserFriendlyTaskKindREST UserFriendlyTaskKind = "REST"
)

// TODO: this should ideally also be able to convert TaskKindBuiltin
func (k TaskKind) UserFriendlyTaskKind() UserFriendlyTaskKind {
	switch k {
	case TaskKindImage:
		return UserFriendlyTaskKindImage
	case TaskKindNode:
		return UserFriendlyTaskKindNode
	case TaskKindPython:
		return UserFriendlyTaskKindPython
	case TaskKindShell:
		return UserFriendlyTaskKindShell
	case TaskKindSQL:
		return UserFriendlyTaskKindSQL
	case TaskKindREST:
		return UserFriendlyTaskKindREST
	}

	return ""
}

type BuildType string

const (
	NodeBuildType   BuildType = "node"
	ViewBuildType   BuildType = "view"
	PythonBuildType BuildType = "python"
	ShellBuildType  BuildType = "shell"
	// NoneBuildType indicates that the entity should not be built.
	NoneBuildType BuildType = "none"
)

func (b BuildType) Valid() bool {
	_, ok := AllBuildTypeVersions[b]
	return ok
}

type BuildTypeVersion string

const (
	BuildTypeVersionNode14 BuildTypeVersion = "14"
	BuildTypeVersionNode16 BuildTypeVersion = "16"
	BuildTypeVersionNode18 BuildTypeVersion = "18"

	BuildTypeVersionPython37  BuildTypeVersion = "3.7"
	BuildTypeVersionPython38  BuildTypeVersion = "3.8"
	BuildTypeVersionPython39  BuildTypeVersion = "3.9"
	BuildTypeVersionPython310 BuildTypeVersion = "3.10"
	BuildTypeVersionPython311 BuildTypeVersion = "3.11"

	BuildTypeVersionUnspecified BuildTypeVersion = ""
)

const (
	DefaultNodeVersion   = BuildTypeVersionNode18
	DefaultPythonVersion = BuildTypeVersionPython310
)

var AllBuildTypeVersions = map[BuildType][]BuildTypeVersion{
	NodeBuildType: {
		BuildTypeVersionNode14,
		BuildTypeVersionNode16,
		BuildTypeVersionNode18,
		BuildTypeVersionUnspecified,
	},
	ViewBuildType: {
		BuildTypeVersionNode14,
		BuildTypeVersionNode16,
		BuildTypeVersionNode18,
		BuildTypeVersionUnspecified,
	},
	PythonBuildType: {
		BuildTypeVersionPython37,
		BuildTypeVersionPython38,
		BuildTypeVersionPython39,
		BuildTypeVersionPython310,
		BuildTypeVersionPython311,
		BuildTypeVersionUnspecified,
	},
	ShellBuildType: {
		BuildTypeVersionUnspecified,
	},
	NoneBuildType: {
		BuildTypeVersionUnspecified,
	},
}

type BuildContext struct {
	Type    BuildType              `json:"type"`
	Version BuildTypeVersion       `json:"version"`
	Base    BuildBase              `json:"base"`
	EnvVars map[string]EnvVarValue `json:"envVars"`
}
type EnvVarValue struct {
	Value  *string `json:"value,omitempty"`
	Config *string `json:"config,omitempty"`
}

func (b BuildContext) Valid() bool {
	if !b.Type.Valid() {
		return false
	}
	return slices.Contains(AllBuildTypeVersions[b.Type], b.Version)
}

func (b BuildContext) VersionOrDefault() BuildTypeVersion {
	if b.Version == BuildTypeVersionUnspecified {
		return b.DefaultVersion()
	}
	return b.Version
}

func (b BuildContext) DefaultVersion() BuildTypeVersion {
	switch b.Type {
	case NodeBuildType, ViewBuildType:
		return DefaultNodeVersion
	case PythonBuildType:
		return DefaultPythonVersion
	default:
		return BuildTypeVersionUnspecified
	}
}

type BuildBase string

const (
	BuildBaseFull BuildBase = "full"
	BuildBaseSlim BuildBase = "slim"
	BuildBaseNone BuildBase = ""
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

type BuildInstructions struct {
	InstallInstructions []InstallInstruction
	BuildArgs           []string
}

func (i BuildInstructions) DockerfileString() (string, error) {
	return utils.ApplyTemplate(heredoc.Doc(`
		{{range .BuildArgs}}
		ARG {{.}}
		{{end}}
		{{range .InstallInstructions}}
		{{if .SrcPath}}
		COPY {{.SrcPath}} {{if .DstPath}}{{.DstPath}}{{else}}.{{end}}
		{{if .Executable}}
		RUN chmod +x {{if .DstPath}}{{.DstPath}}{{else}}{{.SrcPath}}{{end}}
		{{end}}
		{{end}}
		{{if .Cmd}}RUN {{.Cmd}}{{end}}
		{{end}}
	`), i)
}

type InstallInstruction struct {
	Cmd        string
	SrcPath    string
	DstPath    string
	Executable bool
}

type ErrUnsupportedBuilder struct {
	Type BuildType
}

var _ error = &ErrUnsupportedBuilder{}

func (e ErrUnsupportedBuilder) Error() string {
	return fmt.Sprintf("Unsupported builder type %q", e.Type)
}
