package print

import (
	"github.com/airplanedev/cli/pkg/api"
	libapi "github.com/airplanedev/lib/pkg/api"
)

var (
	// DefaultFormatter is the default formatter to use.
	//
	// It defaults to the `table` formatter which prints
	// to the CLI using the tablewriter package.
	DefaultFormatter Formatter = Table{}
)

// Formatter represents an output formatter.
type Formatter interface {
	apiKeys([]api.APIKey) error
	tasks([]libapi.Task) error
	task(libapi.Task) error
	runs([]api.Run) error
	run(api.Run) error
	outputs(api.Outputs) error
	config(api.Config) error
}

// APIKeys prints one or more API keys.
func APIKeys(apiKeys []api.APIKey) error {
	return DefaultFormatter.apiKeys(apiKeys)
}

// Tasks prints the given slice of tasks using the default formatter.
func Tasks(tasks []libapi.Task) error {
	return DefaultFormatter.tasks(tasks)
}

// Task prints a single task.
func Task(task libapi.Task) error {
	return DefaultFormatter.task(task)
}

// Runs prints the given runs.
func Runs(runs []api.Run) error {
	return DefaultFormatter.runs(runs)
}

// Run prints a single run.
func Run(run api.Run) error {
	return DefaultFormatter.run(run)
}

// Outputs prints a collection of outputs.
func Outputs(outputs api.Outputs) error {
	return DefaultFormatter.outputs(outputs)
}

// Config prints a single config var.
func Config(config api.Config) error {
	return DefaultFormatter.config(config)
}

// Print outputs obj based on DefaultFormatter
// If JSON or YAML, uses that formatter to encode obj
// Otherwise, calls defaultPrintFunc to render the obj
func Print(obj interface{}, defaultPrintFunc func()) error {
	switch f := DefaultFormatter.(type) {
	case *JSON:
		return f.Encode(obj)
	case YAML:
		return f.Encode(obj)
	default:
		defaultPrintFunc()
	}
	return nil
}
