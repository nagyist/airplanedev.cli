package print

import (
	"os"

	"github.com/airplanedev/cli/pkg/api"
	libapi "github.com/airplanedev/lib/pkg/api"
	"gopkg.in/yaml.v3"
)

// YAML implements a YAML formatter.
//
// Its zero-value is ready for use.
type YAML struct{}

var _ Formatter = YAML{}

// Encode allows external callers to use the same encoder
func (YAML) Encode(obj interface{}) error {
	return yaml.NewEncoder(os.Stdout).Encode(obj)
}

// APIKeys implementation.
func (YAML) apiKeys(apiKeys []api.APIKey) error {
	return yaml.NewEncoder(os.Stdout).Encode(apiKeys)
}

// Tasks implementation.
func (YAML) tasks(tasks []libapi.Task) error {
	return yaml.NewEncoder(os.Stdout).Encode(printTasks(tasks))
}

// Task implementation.
func (YAML) task(task libapi.Task) error {
	return yaml.NewEncoder(os.Stdout).Encode(printTask(task))
}

// Runs implementation.
func (YAML) runs(runs []api.Run) error {
	return yaml.NewEncoder(os.Stdout).Encode(printRuns(runs))
}

// Run implementation.
func (YAML) run(run api.Run) error {
	return yaml.NewEncoder(os.Stdout).Encode(printRun(run))
}

// Outputs implementation.
func (YAML) outputs(outputs api.Outputs) error {
	// TODO: update ojson to handle yaml properly
	return yaml.NewEncoder(os.Stdout).Encode(outputs.V)
}

// Config implementation.
func (YAML) config(config api.Config) error {
	return yaml.NewEncoder(os.Stdout).Encode(config)
}
