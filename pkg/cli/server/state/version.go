package state

import (
	"context"

	"github.com/airplanedev/cli/pkg/cli/update"
	"github.com/airplanedev/cli/pkg/version"
)

type VersionMetadata struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	IsLatest bool   `json:"isLatest"`
}

func (s *State) Version(ctx context.Context) VersionMetadata {
	s.versionMu.Lock()
	defer s.versionMu.Unlock()

	if s.version == nil {
		isLatest := update.CheckLatest(ctx, nil)
		s.version = &VersionMetadata{
			Status:   "ok",
			Version:  version.Get(),
			IsLatest: isLatest,
		}
	}

	return *s.version
}
