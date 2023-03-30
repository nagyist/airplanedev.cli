package dev

import (
	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/logger"
	liblogs "github.com/airplanedev/cli/pkg/logs"
)

// scanForErrors scans a batch of logs for common errors that we monitor for.
//
// Any errors found will be reported (via analytics events and debug logs).
func scanForErrors(c api.APIClient, log string) {
	if module, ok := liblogs.ScanForErrorNodeESM(log); ok {
		analytics.Track(c, "Run Error Detected", map[string]interface{}{
			"error":  "node-esm-only-dependency",
			"module": module,
		})
		logger.Debug("Run failed from ESM-only dependency on %q", module)
	}
}
