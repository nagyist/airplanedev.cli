package print //nolint: predeclared

import (
	"os"

	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"gopkg.in/yaml.v3"
)

// YAML implements a YAML formatter.
//
// Its zero-value is ready for use.
type YAML struct{}

// Encode allows external callers to use the same encoder
func (YAML) Encode(obj interface{}) {
	handleErr(yaml.NewEncoder(os.Stdout).Encode(obj))
}

// APIKeys implementation.
func (YAML) apiKeys(apiKeys []api.APIKey) {
	handleErr(yaml.NewEncoder(os.Stdout).Encode(apiKeys))
}

// Tasks implementation.
func (YAML) tasks(tasks []libapi.Task) {
	handleErr(yaml.NewEncoder(os.Stdout).Encode(printTasks(tasks)))
}

// Task implementation.
func (YAML) task(task libapi.Task) {
	handleErr(yaml.NewEncoder(os.Stdout).Encode(printTask(task)))
}

// Runs implementation.
func (YAML) runs(runs []api.Run) {
	handleErr(yaml.NewEncoder(os.Stdout).Encode(printRuns(runs)))
}

// Run implementation.
func (YAML) run(run api.Run) {
	handleErr(yaml.NewEncoder(os.Stdout).Encode(printRun(run)))
}

// Outputs implementation.
func (YAML) outputs(outputs api.Outputs) {
	// TODO: update ojson to handle yaml properly
	handleErr(yaml.NewEncoder(os.Stdout).Encode(outputs.V))
}

// Config implementation.
func (YAML) config(config api.Config) {
	handleErr(yaml.NewEncoder(os.Stdout).Encode(config))
}
