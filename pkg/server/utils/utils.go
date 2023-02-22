package utils

import (
	"net/http"

	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/airplanedev/cli/pkg/utils/pointers"
)

const NO_FALLBACK_ENVIRONMENT = "no-fallback-environment"

func GetEffectiveEnvSlugFromRequest(s *state.State, r *http.Request) *string {
	envSlug := r.Header.Get("X-Airplane-Studio-Fallback-Env-Slug")
	if envSlug == "" {
		return s.InitialRemoteEnvSlug
	} else if envSlug == NO_FALLBACK_ENVIRONMENT {
		return nil
	}
	return pointers.String(envSlug)
}
