package dev

import (
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/logs"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/builtins"
	"github.com/airplanedev/lib/pkg/deploy/discover"
)

type LocalRun struct {
	RunID            string                 `json:"runID"`
	Status           api.RunStatus          `json:"status"`
	Outputs          api.Outputs            `json:"outputs"`
	CreatedAt        time.Time              `json:"createdAt"`
	CreatorID        string                 `json:"creatorID"`
	SucceededAt      *time.Time             `json:"succeededAt"`
	FailedAt         *time.Time             `json:"failedAt"`
	ParamValues      map[string]interface{} `json:"paramValues"`
	Parameters       *libapi.Parameters     `json:"parameters"`
	ParentID         string                 `json:"parentID"`
	TaskID           string                 `json:"taskID"`
	TaskName         string                 `json:"taskName"`
	Displays         []libapi.Display       `json:"displays"`
	Prompts          []libapi.Prompt        `json:"prompts"`
	IsWaitingForUser bool                   `json:"isWaitingForUser"`

	// The version of the task at the time of the run execution
	TaskRevision discover.TaskConfig `json:"-"`

	// Map of a run's attached resources: slug to ID
	Resources map[string]string `json:"resources"`

	IsStdAPI      bool                   `json:"isStdAPI"`
	StdAPIRequest builtins.StdAPIRequest `json:"stdAPIRequest"`

	// internal fields
	LogStore  logs.LogBroker `json:"-"`
	LogBroker logs.LogBroker `json:"-"`
	Remote    bool           `json:"-"`
}

// NewLocalRun initializes a run for local dev.
func NewLocalRun() *LocalRun {
	return &LocalRun{
		Status:      api.RunQueued,
		ParamValues: map[string]interface{}{},
		Parameters:  &libapi.Parameters{},
		CreatedAt:   time.Now().UTC(),
		LogBroker:   logs.NewDevLogBroker(),
		Displays:    []libapi.Display{},
		Prompts:     []libapi.Prompt{},
		Resources:   map[string]string{},
	}
}

func GenerateRunID() string {
	return utils.GenerateID("devrun")
}
