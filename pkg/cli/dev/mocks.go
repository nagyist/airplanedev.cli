package dev

import (
	"context"
	"sync"

	"github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/stretchr/testify/mock"
)

// MockExecutor is a mock implementation of the Executor interface.
type MockExecutor struct {
	mock.Mock
	WG sync.WaitGroup
}

func (m *MockExecutor) Execute(ctx context.Context, config LocalRunConfig) (api.Outputs, error) {
	// The run ID is generated inside the `/v0/tasks/execute` handler, and so we don't check equality here.
	config.ID = ""
	args := m.Called(ctx, config)
	return api.Outputs{}, args.Error(1)
}

func (m *MockExecutor) Refresh() error {
	defer m.WG.Done()
	args := m.Called()
	return args.Error(0)
}
