package expressions

import (
	"context"
	"fmt"

	"github.com/airplanedev/lib/pkg/expressions/expressionsiface"
	"github.com/airplanedev/ojson"
	"github.com/airplanedev/path"
)

type LookupClient struct{}

var _ expressionsiface.ExpressionsService = LookupClient{}

// NewLookupClient is an ExpressionsService implementation that supports
// only JS-based paths. These paths are looked up as globals from
// opts.Globals.
//
// LookupClient is best-used for testing. LookupClient was previously used
// for JS expressions in Runbooks V0.
func NewLookupClient() LookupClient {
	return LookupClient{}
}

func (c LookupClient) Evaluate(ctx context.Context, exprs []string, opts expressionsiface.EvaluateOpts) (expressionsiface.EvaluateResponse, error) {
	resp := expressionsiface.EvaluateResponse{
		Results: []expressionsiface.ExpressionResult{},
	}

	for _, e := range exprs {
		p, err := path.FromJS(e)
		if err != nil {
			resp.Results = append(resp.Results, expressionsiface.ExpressionResult{
				ErrorMsg: "invalid path",
			})
			continue
		}

		if p.Len() == 0 {
			resp.Results = append(resp.Results, expressionsiface.ExpressionResult{
				ErrorMsg: "empty path",
			})
			continue
		}

		g, ok := p.At(0).(string)
		if !ok {
			resp.Results = append(resp.Results, expressionsiface.ExpressionResult{
				ErrorMsg: "expected a string",
			})
			continue
		}
		fn, ok := opts.Globals[g]
		if !ok {
			resp.Results = append(resp.Results, expressionsiface.ExpressionResult{
				ErrorMsg: fmt.Sprintf("unknown global: %q", g),
			})
			continue
		}

		// Truncate the name of the global from the path:
		result, err := fn(ctx, p.Sub(1))
		if err != nil {
			return expressionsiface.EvaluateResponse{}, err
		}

		resp.Results = append(resp.Results, result)
	}

	return resp, nil
}

// LookupFn is an ExpressionsLookupFn wrapper around Lookup.
func LookupFn(m interface{}) expressionsiface.ExpressionsLookupFn {
	return func(ctx context.Context, p path.P) (expressionsiface.ExpressionResult, error) {
		return Lookup(p, m), nil
	}
}

// Lookup is a helper looking up a path in a value. If the underlying
// value cannot be found, it will return errors via ExpressionResult.
func Lookup(p path.P, m interface{}) expressionsiface.ExpressionResult {
	resp := expressionsiface.ExpressionResult{
		Output: m,
	}
	for _, k := range p.Components() {
		switch kt := k.(type) {
		case string:
			if v, ok := resp.Output.(*ojson.Object); ok {
				resp.Output, _ = v.Get(kt)
			} else if v, ok := resp.Output.(map[string]interface{}); ok {
				resp.Output = v[kt]
			} else {
				return expressionsiface.ExpressionResult{
					ErrorMsg: fmt.Sprintf("expected a map: got %T", resp.Output),
				}
			}
		case int:
			if v, ok := resp.Output.([]interface{}); ok {
				resp.Output = v[kt]
			} else {
				return expressionsiface.ExpressionResult{
					ErrorMsg: fmt.Sprintf("expected a list: got %T", resp.Output),
				}
			}
		default:
			return expressionsiface.ExpressionResult{
				ErrorMsg: fmt.Sprintf("expected a string or integer path: got %T", k),
			}
		}
	}

	return resp
}
