package prompts

import (
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/errors"
)

// Surveyor is a Prompter that uses the survey package.
type Surveyor struct{}

var _ Prompter = Surveyor{}

func (s Surveyor) Confirm(question string, o ...Opt) (bool, error) {
	promptOpts := &opts{
		Default: true,
	}

	for _, opt := range o {
		opt(promptOpts)
	}

	d, ok := promptOpts.Default.(bool)
	if !ok {
		return false, errors.New("default value must be a bool")
	}

	askOpts := generateAskOpts(promptOpts)

	if err := survey.AskOne(
		&survey.Confirm{
			Message: question,
			Default: d,
		},
		&ok,
		askOpts...,
	); err != nil {
		return false, errors.Wrap(err, "confirming")
	}

	return ok, nil
}

func (s Surveyor) ConfirmWithAssumptions(question string, assumeYes, assumeNo bool, opts ...Opt) (bool, error) {
	if assumeYes {
		return true, nil
	}
	if assumeNo {
		return false, nil
	}

	return s.Confirm(question, opts...)
}

func (s Surveyor) Input(question string, p *string, o ...Opt) error {
	promptOpts := &opts{}
	for _, opt := range o {
		opt(promptOpts)
	}

	askOpts := generateAskOpts(promptOpts)

	var prompt survey.Prompt
	if promptOpts.Secret {
		prompt = &survey.Password{
			Message: question,
			Help:    promptOpts.Help,
		}
	} else if promptOpts.Select != nil {
		prompt = &survey.Select{
			Message: question,
			Options: promptOpts.Select,
			Default: promptOpts.Default,
			Help:    promptOpts.Help,
		}
	} else {
		var d string
		if promptOpts.Default != nil {
			var ok bool
			if d, ok = promptOpts.Default.(string); !ok {
				return errors.New("default value must be a string")
			}
		}

		prompt = &survey.Input{
			Message: question,
			Suggest: promptOpts.Suggest,
			Default: d,
			Help:    promptOpts.Help,
		}
	}

	if err := survey.AskOne(
		prompt,
		p,
		askOpts...,
	); err != nil {
		return errors.Wrap(err, "prompting for input")
	}

	return nil
}

func generateAskOpts(promptOpts *opts) []survey.AskOpt {
	askOpts := []survey.AskOpt{survey.WithStdio(os.Stdin, os.Stderr, os.Stderr)}

	if promptOpts.Required {
		askOpts = append(askOpts, survey.WithValidator(survey.Required))
	}

	for _, validator := range promptOpts.Validators {
		askOpts = append(askOpts, survey.WithValidator(validator))
	}

	return askOpts
}
