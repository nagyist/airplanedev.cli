package runtime

import (
	"context"
	"fmt"
	"reflect"

	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/expressions"
	"github.com/airplanedev/lib/pkg/expressions/expressionsiface"
	"github.com/pkg/errors"
)

type EvalGlobals struct {
	ParamValues map[string]interface{}
	Configs     map[string]interface{}
}

// EvalTemplates takes in the user input and evaluates the JST against
// configs and param values. Example: {{configs.API_KEY}} and {{params.user_name}}
// Uses the expressions package until we move to V8
func EvalTemplates(ctx context.Context, inputs map[string]interface{}, globals EvalGlobals) (map[string]interface{}, error) {
	evaluatedExpressions := map[string]interface{}{}
	c := expressions.NewLookupClient()
	for k, arg := range inputs {
		argString, ok := arg.(string)
		if !ok {
			return nil, fmt.Errorf("invalid input: '%v' for %s", arg, k)
		}
		tmpl := expressions.NewTemplate(argString)
		result, err := tmpl.Evaluate(ctx, c, expressionsiface.EvaluateOpts{
			Globals: map[string]expressionsiface.ExpressionsLookupFn{
				"params":  expressions.LookupFn(globals.ParamValues),
				"configs": expressions.LookupFn(globals.Configs),
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "error evaluating template")
		}
		evaluatedExpressions[k] = result.Output
	}
	return evaluatedExpressions, nil
}

// EvalRunOptionTemplates takes in the run options
// and evaluates templates found in the kindOptions
func EvalRunOptionTemplates(ctx context.Context, opts PrepareRunOptions) (build.KindOptions, error) {
	evaluatedOptions := map[string]interface{}{}
	for field, val := range opts.KindOptions {
		if reflect.ValueOf(val).Kind() == reflect.Map {
			jstMap, ok := opts.KindOptions[field].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid %s", field)
			}

			evaluated, err := EvalTemplates(ctx, jstMap, EvalGlobals{
				ParamValues: opts.ParamValues,
				Configs:     opts.ConfigVars,
			})
			if err != nil {
				return nil, err
			}
			evaluatedOptions[field] = evaluated
		} else {
			evaluatedOptions[field] = val
		}
	}
	return evaluatedOptions, nil
}
