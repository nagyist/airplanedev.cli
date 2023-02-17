package main

import (
	"context"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/airplanedev/cli/cmd/airplane/root"
	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/utils"
	"github.com/airplanedev/trap"
	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
	_ "github.com/segmentio/events/v2/text"
)

var (
	version = "<dev>"
)

func main() {
	var cmd = root.New()
	var ctx = trap.Context()

	cmd.Version = version

	defer func() {
		if r := recover(); r != nil {
			logger.Error("The CLI unexpectedly crashed: %+v", r) // This does not print the stack trace.
			if logger.EnableDebug {
				logger.Debug(string(debug.Stack()))
			} else {
				logger.Log("An internal error occurred, run with --debug for more information")
			}
			sentry.CurrentHub().Recover(r)
			sentry.Flush(time.Second * 5)
			analytics.Close()
			os.Exit(1)
		}
	}()

	if err := cmd.ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			// TODO(amir): output operation canceled?
			return
		}

		logger.Debug("Error: %+v", err)
		logger.Log("")
		if exerr, ok := errors.Cause(err).(utils.ErrorExplained); ok {
			logger.Error(capitalize(exerr.Error()))
			logger.Log("")
			logger.Log(capitalize(exerr.ExplainError()))
		} else {
			logger.Error(capitalize(errors.Cause(err).Error()))
		}
		logger.Log("")

		analytics.ReportError(err)
		analytics.Close()
		os.Exit(1)
	}
}

func capitalize(str string) string {
	if len(str) > 0 {
		return strings.ToUpper(str[0:1]) + str[1:]
	}
	return str
}
