package print

import (
	"time"

	"github.com/airplanedev/cli/pkg/api"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
)

// This struct mirrors api.Task, but with different json/yaml tags.
type printTask struct {
	URL                        string                  `json:"url" yaml:"url"`
	ID                         string                  `json:"id" yaml:"id"`
	Name                       string                  `json:"name" yaml:"name"`
	Slug                       string                  `json:"slug" yaml:"slug"`
	Description                string                  `json:"description" yaml:"description"`
	Image                      *string                 `json:"image" yaml:"image"`
	Command                    []string                `json:"command" yaml:"command"`
	Arguments                  []string                `json:"arguments" yaml:"arguments"`
	Parameters                 libapi.Parameters       `json:"parameters" yaml:"parameters"`
	Constraints                libapi.RunConstraints   `json:"constraints" yaml:"constraints"`
	Env                        libapi.TaskEnv          `json:"env" yaml:"env"`
	ResourceRequests           libapi.ResourceRequests `json:"resourceRequests" yaml:"resourceRequests"`
	Resources                  libapi.Resources        `json:"resources" yaml:"resources"`
	Kind                       build.TaskKind          `json:"builder" yaml:"builder"`
	KindOptions                build.KindOptions       `json:"builderConfig" yaml:"builderConfig"`
	Repo                       string                  `json:"repo" yaml:"repo"`
	RequireExplicitPermissions bool                    `json:"requireExplicitPermissions" yaml:"-"`
	Permissions                libapi.Permissions      `json:"permissions" yaml:"-"`
	ExecuteRules               libapi.ExecuteRules     `json:"executeRules" yaml:"executeRules"`
	Timeout                    int                     `json:"timeout" yaml:"timeout"`
	InterpolationMode          string                  `json:"-" yaml:"-"`
}

func printTasks(tasks []libapi.Task) []printTask {
	pts := make([]printTask, len(tasks))
	for i, t := range tasks {
		pts[i] = printTask(t)
	}
	return pts
}

// This struct mirrors api.Run, but with different json/yaml tags.
type printRun struct {
	RunID       string        `json:"id" yaml:"id"`
	TaskID      string        `json:"taskID" yaml:"taskID"`
	TaskName    string        `json:"taskName" yaml:"taskName"`
	TeamID      string        `json:"teamID" yaml:"teamID"`
	Status      api.RunStatus `json:"status" yaml:"status"`
	ParamValues api.Values    `json:"paramValues" yaml:"paramValues"`
	CreatedAt   time.Time     `json:"createdAt" yaml:"createdAt"`
	CreatorID   string        `json:"creatorID" yaml:"creatorID"`
	QueuedAt    *time.Time    `json:"queuedAt" yaml:"queuedAt"`
	ActiveAt    *time.Time    `json:"activeAt" yaml:"activeAt"`
	SucceededAt *time.Time    `json:"succeededAt" yaml:"succeededAt"`
	FailedAt    *time.Time    `json:"failedAt" yaml:"failedAt"`
	CancelledAt *time.Time    `json:"cancelledAt" yaml:"cancelledAt"`
	CancelledBy *string       `json:"cancelledBy" yaml:"cancelledBy"`
	EnvSlug     string        `json:"envSlug" yaml:"envSlug"`
}

func printRuns(runs []api.Run) []printRun {
	pruns := make([]printRun, len(runs))
	for i, r := range runs {
		pruns[i] = printRun(r)
	}
	return pruns
}
