package flagsiface

import (
	"context"

	"github.com/airplanedev/cli/pkg/logger"
)

const (
	DefaultInlineConfigTasks = "default-inline-tasks"
)

// Flaggers are the primary mechanism for dynamically adjusting runtime behavior
// on a per-customer level.
type Flagger interface {
	// Bool returns a boolean representing the state of `flag`. For example:
	//
	// 	if on := flagger.Bool(ctx, "my-flag"); on {
	// 		// ...
	// 	}
	//
	// If an error is encountered, `opts.Default` will be returned.
	Bool(ctx context.Context, l logger.Logger, flag string, opts ...BoolOpts) bool
}

// BoolOpts provides optional configuration for changing the behavior of
// a Flagger implementation.
type BoolOpts struct {
	// Default determines what to return if an error is encountered while
	// checking a flag.
	//
	// Defaults to `false`.
	Default bool
}
