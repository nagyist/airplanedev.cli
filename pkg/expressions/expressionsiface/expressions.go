package expressionsiface

import (
	"context"
	"time"

	"github.com/airplanedev/path"
)

// ExpressionsService is the service interface for executing arbitrary JS expressions.
type ExpressionsService interface {
	// Evaluate receives a series of JS expressions to evaluate.
	//
	// Each expression is evaluated independently and therefore
	// cannot reference declarations from other expressions.
	//
	// As long as error is nil, the number of results in the
	// response will always be equal to the number of expressions
	// in the request. The indexes of results in the response
	// will always align with the indexes of expressions in the
	// request.
	Evaluate(ctx context.Context, exprs []string, opts EvaluateOpts) (EvaluateResponse, error)
}

// EvaluateOpts provides optional configuration for changing the behavior of
// an Evaluate call.
type EvaluateOpts struct {
	// Globals define dynamically-backed top-level variables that can be accessed
	// by users as normal JS globals. Globals are the primary mechanism for providing
	// Airplane-based functionality relevant to the context in which the JS expression
	// is executing. When a global is evaluated, the ExpressionsLookupFn will be called
	// to dynamically evaluate it.
	Globals map[string]ExpressionsLookupFn

	// Optional, set to override default timeout. Mostly used for testing.
	Timeout time.Duration
}

// EvaluateResponse represents the response of a call to Evaluate.
type EvaluateResponse struct {
	Results []ExpressionResult
}

// ExpressionResult is the output of a single JS expression. If a non-empty
// ErrorMsg is returned, then the expression should be considered as having
// failed. Otherwise, the Output field will contain the (possibly nil) output
// of the expression.
type ExpressionResult struct {
	Output   interface{}
	ErrorMsg string
}

// ExpressionsLookupFn is a callback triggered whenever a user requests a lookup
// on a dynamically-backed global variable. See EvaluateOpts.Globals.
//
// Each ExpressionsLookupFn will be passed a `context.Context` that will be canceled
// if the underlying V8 expression is canceled or timed out. Callbacks should listen
// for cancelation and return the following if the callback did not finish:
//
//	ap.ExpressionResult{
//	  ErrorMsg: ErrEvaluationCanceled.Error(),
//	}, nil
type ExpressionsLookupFn func(ctx context.Context, p path.P) (ExpressionResult, error)
