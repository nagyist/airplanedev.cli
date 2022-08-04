package dev

import "github.com/airplanedev/cli/pkg/api"

type LocalRun struct {
	Status    api.RunStatus
	Outputs   api.Outputs
	LogConfig LogConfig
}

// NewLocalRun initializes a run for local dev.
func NewLocalRun() *LocalRun {
	return &LocalRun{
		Status: api.RunQueued,
		LogConfig: LogConfig{
			Channel: make(chan string),
			Logs:    make([]string, 0),
		},
	}
}
