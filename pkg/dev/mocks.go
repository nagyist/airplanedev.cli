package dev

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockExecutor is a mock implementation of the Executor interface.
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Execute(ctx context.Context, config LocalRunConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}
