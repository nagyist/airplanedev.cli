package dev

import (
	"time"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/dev/logs"
	"github.com/airplanedev/cli/pkg/utils"
	libapi "github.com/airplanedev/lib/pkg/api"
)

type LocalRun struct {
	RunID       string                 `json:"runID"`
	Status      api.RunStatus          `json:"status"`
	Outputs     api.Outputs            `json:"outputs"`
	CreatedAt   time.Time              `json:"createdAt"`
	CreatorID   string                 `json:"creatorID"`
	SucceededAt *time.Time             `json:"succeededAt"`
	FailedAt    *time.Time             `json:"failedAt"`
	ParamValues map[string]interface{} `json:"paramValues"`
	Parameters  *libapi.Parameters     `json:"parameters"`
	ParentID    string                 `json:"parentID"`
	TaskID      string                 `json:"taskID"`
	TaskName    string                 `json:"taskName"`
	Displays    []libapi.Display       `json:"displays"`
	Prompts     []libapi.Prompt        `json:"prompts"`

	// Map of a run's attached resources: slug to ID
	Resources map[string]string `json:"resources"`

	IsStdAPI      bool          `json:"isStdAPI"`
	StdAPIRequest StdAPIRequest `json:"stdAPIRequest"`

	// internal fields
	LogStore  logs.LogBroker `json:"-"`
	LogBroker logs.LogBroker `json:"-"`
}

// NewLocalRun initializes a run for local dev.
func NewLocalRun() *LocalRun {
	return &LocalRun{
		Status:      api.RunQueued,
		ParamValues: map[string]interface{}{},
		CreatedAt:   time.Now(),
		LogBroker:   logs.NewDevLogBroker(),
		Displays:    []libapi.Display{},
		Prompts:     []libapi.Prompt{},
		Resources:   map[string]string{},
	}
}

func GenerateRunID() string {
	return utils.GenerateID("run")
}
