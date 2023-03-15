package params

import (
	"flag"
	"fmt"
	"reflect"
	"regexp"

	"github.com/AlecAivazis/survey/v2"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/prompts"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/pkg/errors"
)

// CLI parses a list of flags as Airplane parameters and returns the values.
//
// A flag.ErrHelp error will be returned if a -h or --help was provided, in which case
// this function will print out help text on how to pass this task's parameters as flags.
func CLI(args []string, taskName string, parameters libapi.Parameters, p prompts.Prompter) (api.Values, error) {
	values := api.Values{}

	if len(args) > 0 {
		// If args have been passed in, parse them as flags
		set := flagset(taskName, parameters, values)
		if err := set.Parse(args); err != nil {
			return nil, err
		}
	} else {
		// Otherwise, try to prompt for parameters
		if err := promptForParamValues(parameters, values, p); err != nil {
			return nil, err
		}
	}

	return values, nil
}

// Flagset returns a new flagset from the given task parameters.
func flagset(taskName string, parameters libapi.Parameters, args api.Values) *flag.FlagSet {
	var set = flag.NewFlagSet(taskName, flag.ContinueOnError)

	set.Usage = func() {
		logger.Log("\n%s Usage:", taskName)
		set.VisitAll(func(f *flag.Flag) {
			logger.Log("  --%s %s (default: %q)", f.Name, f.Usage, f.DefValue)
		})
		logger.Log("")
	}

	for i := range parameters {
		// Scope p here (& not above) so we can use it in the closure.
		// See also: https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
		p := parameters[i]
		set.Func(p.Slug, p.Desc, func(v string) (err error) {
			args[p.Slug], err = ParseInput(p, v)
			if err != nil {
				return errors.Wrap(err, "converting input to API value")
			}
			return
		})
	}

	return set
}

// promptForParamValues attempts to prompt user for param values, setting them on `params`
// If there are no parameters, does nothing.
// If TTY, prompts for parameters and then asks user to confirm.
// If no TTY, errors.
func promptForParamValues(
	parameters libapi.Parameters,
	paramValues map[string]interface{},
	p prompts.Prompter,
) error {
	if len(parameters) == 0 {
		return nil
	}

	if !prompts.CanPrompt() {
		// Error since we have no params and no way to prompt for it
		// TODO: if all parameters optional (or have defaults), do not error.
		logger.Log("Parameters were not specified! Task has %d parameter(s):\n", len(parameters))
		for _, param := range parameters {
			var req string
			if !param.Constraints.Optional {
				req = "*"
			}
			logger.Log("  %s%s %s", param.Name, req, logger.Gray("(--%s)", param.Slug))
			logger.Log("    %s %s", param.Type, param.Desc)
		}
		return errors.New("missing parameters")
	}

	for _, param := range parameters {
		if param.Type == libapi.TypeUpload {
			logger.Log(logger.Yellow("Skipping %s - uploads are not supported in CLI", param.Name))
			continue
		}

		message := fmt.Sprintf("%s %s:", param.Name, logger.Gray("(--%s)", param.Slug))
		defaultValue, err := APIValueToInput(param, param.Default)
		if err != nil {
			return err
		}

		opts := []prompts.Opt{
			prompts.WithValidator(validateParam(param)),
			prompts.WithHelp(param.Desc),
		}
		if !param.Constraints.Optional {
			opts = append(opts, prompts.WithRequired())
		}
		if param.Constraints.Regex != "" {
			opts = append(opts, prompts.WithValidator(validateRegex(param.Constraints.Regex, param.Constraints.Optional)))
		}
		var inputValue string

		switch param.Type {
		case libapi.TypeBoolean:
			if defaultValue != "" {
				opts = append(opts, prompts.WithDefault(defaultValue))
			}
			opts = append(opts, prompts.WithSelectOptions([]string{YesString, NoString}))
			if err := p.Input(message, &inputValue, opts...); err != nil {
				return err
			}
		default:
			opts = append(opts, prompts.WithDefault(defaultValue))
			if err := p.Input(message, &inputValue, opts...); err != nil {
				return err
			}
		}

		value, err := ParseInput(param, inputValue)
		if err != nil {
			return err
		}
		if value != nil {
			paramValues[param.Slug] = value
		}
	}

	if confirmed, err := p.Confirm("Execute?", prompts.WithDefault(true)); err != nil {
		return err
	} else if !confirmed {
		return errors.New("user cancelled")
	}

	return nil
}

// validateParam returns a survey.Validator to perform rudimentary checks on CLI input
func validateParam(param libapi.Parameter) func(interface{}) error {
	return func(ans interface{}) error {
		var v string
		switch a := ans.(type) {
		case string:
			v = a
		case survey.OptionAnswer:
			v = a.Value
		default:
			return errors.Errorf("unexpected answer of type %s", reflect.TypeOf(a).Name())
		}
		return ValidateInput(param, v)
	}
}

// validateRegex returns a Survey validator from the pattern
func validateRegex(pattern string, optional bool) func(interface{}) error {
	return func(val interface{}) error {
		str, ok := val.(string)
		if !ok {
			return errors.New("expected string")
		}
		if str == "" && optional {
			return nil
		}
		matched, err := regexp.MatchString(pattern, str)
		if err != nil {
			return errors.Errorf("errored matching against regex: %s", err)
		}
		if !matched {
			return errors.Errorf("must match regex pattern: %s", pattern)
		}
		return nil
	}
}
