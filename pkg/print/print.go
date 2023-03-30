package print //nolint: predeclared

import (
	"strings"

	libapi "github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/logger"
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
	apiKeys([]api.APIKey)
	tasks([]libapi.Task)
	task(libapi.Task)
	runs([]api.Run)
	run(api.Run)
	outputs(api.Outputs)
	config(api.Config)
}

// APIKeys prints one or more API keys.
func APIKeys(apiKeys []api.APIKey) {
	DefaultFormatter.apiKeys(apiKeys)
}

// Tasks prints the given slice of tasks using the default formatter.
func Tasks(tasks []libapi.Task) {
	DefaultFormatter.tasks(tasks)
}

// Task prints a single task.
func Task(task libapi.Task) {
	DefaultFormatter.task(task)
}

// Runs prints the given runs.
func Runs(runs []api.Run) {
	DefaultFormatter.runs(runs)
}

// Run prints a single run.
func Run(run api.Run) {
	DefaultFormatter.run(run)
}

// Outputs prints a collection of outputs.
func Outputs(outputs api.Outputs) {
	DefaultFormatter.outputs(outputs)
}

// Config prints a single config var.
func Config(config api.Config) {
	DefaultFormatter.config(config)
}

// Print outputs obj based on DefaultFormatter
// If JSON or YAML, uses that formatter to encode obj
// Otherwise, calls defaultPrintFunc to render the obj
func Print(obj interface{}, defaultPrintFunc func()) {
	switch f := DefaultFormatter.(type) {
	case *JSON:
		f.Encode(obj)
	case YAML:
		f.Encode(obj)
	default:
		defaultPrintFunc()
	}
}

// BoxPrint pretty prints a box around the given string - for example, BoxPrint("hello") would output
// +-------+
// | hello |
// +-------+
func BoxPrint(s string) {
	BoxPrintWithPrefix(s, "")
}

func BoxPrintWithPrefix(s, prefix string) {
	Print(s, func() {
		lines := strings.Split(s, "\n")
		sLen := 0
		for _, line := range lines {
			if len(line) > sLen {
				sLen = len(line)
			}
		}
		logger.Log(prefix + "+" + strings.Repeat("-", sLen+2) + "+")
		for _, line := range lines {
			padding := strings.Repeat(" ", sLen-len(line))
			logger.Log(prefix + "| " + line + padding + " |")
		}
		logger.Log(prefix + "+" + strings.Repeat("-", sLen+2) + "+")
	})
}

func handleErr(err error) {
	if err != nil {
		logger.Error("failed to print output: %+v", err)
	}
}
