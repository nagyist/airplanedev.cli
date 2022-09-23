package dev

import (
	"context"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/stretchr/testify/mock"
)

// MockExecutor is a mock implementation of the Executor interface.
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Execute(ctx context.Context, config LocalRunConfig) (api.Outputs, error) {
	// The run ID is generated inside the `/v0/tasks/execute` handler, and so we don't check equality here.
	config.ID = ""
	args := m.Called(ctx, config)
	return api.Outputs{}, args.Error(0)
}
