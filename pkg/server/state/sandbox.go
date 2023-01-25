package state

import (
	"context"
	"sync"
	"time"

	"github.com/airplanedev/cli/pkg/dev"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/lib/pkg/deploy/bundlediscover"
)

type SandboxState struct {
	OutdatedDependencies bool
	IsRebuilding         bool
	HasDependencyError   bool
	Logger               logger.Logger

	mu sync.Mutex
}

func NewSandboxState(l logger.Logger) *SandboxState {
	return &SandboxState{
		Logger: l,
	}
}

func (s *SandboxState) rebuild(ctx context.Context, bd *bundlediscover.Discoverer, paths ...string) {
	if s.IsRebuilding {
		return
	}

	s.Logger.Log("Reinstalling all dependencies...")
	s.mu.Lock()
	s.IsRebuilding = true
	s.mu.Unlock()

	err := dev.InstallAllBundleDependencies(ctx, bd, paths...)

	s.mu.Lock()
	if err == nil {
		s.OutdatedDependencies = false
		s.HasDependencyError = false
		s.Logger.Log("Reinstalled all dependencies.")
	} else {
		s.HasDependencyError = true
		s.Logger.Log("Error reinstalling dependencies: %v", err)
	}
	s.IsRebuilding = false
	s.mu.Unlock()
}

// Rebuilds fully asynchronously.
func (s *SandboxState) Rebuild(ctx context.Context, bd *bundlediscover.Discoverer, paths ...string) {
	go func() {
		s.rebuild(ctx, bd, paths...)
	}()
}

// Kicks off a rebuild and waits up to five seconds. Returns true iff the rebuild is continuing
// asynchronously.
func (s *SandboxState) RebuildWithTimeout(ctx context.Context, bd *bundlediscover.Discoverer, paths ...string) bool {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.rebuild(ctx, bd, paths...)
	}()

	return utils.WaitTimeout(&wg, time.Second*5)
}

func (s *SandboxState) MarkDependenciesOutdated() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OutdatedDependencies = true
}
