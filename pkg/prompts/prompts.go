package prompts

import (
	"os"

	"github.com/mattn/go-isatty"
)

// Prompter is an interface for prompting the user for input.
type Prompter interface {
	Confirm(question string, opts ...Opt) (bool, error)
	ConfirmWithAssumptions(question string, assumeYes, assumeNo bool, opts ...Opt) (bool, error)
	Input(question string, p *string, opts ...Opt) error
}

type opts struct {
	Required   bool
	Secret     bool
	Validators []func(interface{}) error
	Help       string
	Default    interface{}
	Select     []string
	Suggest    func(toComplete string) []string
}

type Opt func(opts *opts)

// WithRequired marks the prompt required.
func WithRequired() Opt {
	return func(o *opts) {
		o.Required = true
	}
}

// WithSecret hides the user's input.
func WithSecret() Opt {
	return func(o *opts) {
		o.Secret = true
	}
}

// WithHelp adds help text to the prompt.
func WithHelp(help string) Opt {
	return func(o *opts) {
		o.Help = help
	}
}

// WithSelectOptions adds a set of pre-defined options to the prompt.
func WithSelectOptions(selectOpts []string) Opt {
	return func(o *opts) {
		o.Select = selectOpts
	}
}

// WithValidator adds a validator for the user's input.
func WithValidator(validator func(interface{}) error) Opt {
	return func(o *opts) {
		o.Validators = append(o.Validators, validator)
	}
}

// WithDefault sets a default value for the prompt to use if the user does not enter a value.
func WithDefault(defaultValue interface{}) Opt {
	return func(o *opts) {
		o.Default = defaultValue
	}
}

// WithSuggest adds a suggestion function to the prompt that will suggest possible values to the user.
func WithSuggest(suggestionFunc func(toComplete string) []string) Opt {
	return func(opts *opts) {
		opts.Suggest = suggestionFunc
	}
}

// CanPrompt checks that both stdin and stderr are terminal
func CanPrompt() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stderr.Fd())
}
