package apidev

import (
	"context"
	"net/http"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/pkg/errors"
)

type IsTaskSlugAvailableResponse struct {
	Available bool `json:"available"`
}

func IsTaskSlugAvailableHandler(ctx context.Context, s *state.State, r *http.Request) (IsTaskSlugAvailableResponse, error) {
	slug := r.URL.Query().Get("slug")

	available, err := IsTaskSlugAvailable(ctx, s, slug)
	if err != nil {
		return IsTaskSlugAvailableResponse{}, err
	}

	return IsTaskSlugAvailableResponse{
		Available: available,
	}, nil
}

func IsTaskSlugAvailable(ctx context.Context, s *state.State, slug string) (bool, error) {
	// Check remote tasks.
	_, err := s.RemoteClient.GetTaskMetadata(ctx, slug)
	if err == nil {
		// Got a hit, so it's not available.
		return false, nil
	}

	var merr *api.TaskMissingError
	if !errors.As(err, &merr) {
		return false, errors.Wrap(err, "unable to get task metadata")
	}

	// Check local tasks.
	if _, ok := s.TaskConfigs.Get(slug); ok {
		// Got a hit, so it's not available.
		return false, nil
	}

	return true, nil
}

type IsViewSlugAvailableResponse struct {
	Available bool `json:"available"`
}

func IsViewSlugAvailableHandler(ctx context.Context, s *state.State, r *http.Request) (IsViewSlugAvailableResponse, error) {
	slug := r.URL.Query().Get("slug")

	available, err := IsViewSlugAvailable(ctx, s, slug)
	if err != nil {
		return IsViewSlugAvailableResponse{}, err
	}

	return IsViewSlugAvailableResponse{
		Available: available,
	}, nil
}

func IsViewSlugAvailable(ctx context.Context, s *state.State, slug string) (bool, error) {
	// Check remote views.
	_, err := s.RemoteClient.GetViewMetadata(ctx, slug)
	if err == nil {
		// Got a hit, so it's not available.
		return false, nil
	}

	var merr *api.ViewMissingError
	if !errors.As(err, &merr) {
		return false, errors.Wrap(err, "unable to get view metadata")
	}

	// Check local views.
	if _, ok := s.ViewConfigs.Get(slug); ok {
		// Got a hit, so it's not available.
		return false, nil
	}

	return true, nil
}
