package print

import (
	"encoding/json"
	"os"

	"github.com/airplanedev/cli/pkg/api"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/ojson"
)

// JSON implements a JSON formatter.
type JSON struct {
	enc *json.Encoder
}

var _ Formatter = &JSON{}

// NewJSONFormatter returns a new json formatter.
func NewJSONFormatter() *JSON {
	return &JSON{
		enc: json.NewEncoder(os.Stdout),
	}
}

// Encode allows external callers to use the same encoder
func (j *JSON) Encode(obj interface{}) error {
	return j.enc.Encode(obj)
}

// APIKeys implementation.
func (j *JSON) apiKeys(apiKeys []api.APIKey) error {
	return j.enc.Encode(apiKeys)
}

// Tasks implementation.
func (j *JSON) tasks(tasks []libapi.Task) error {
	return j.enc.Encode(printTasks(tasks))
}

// Task implementation.
func (j *JSON) task(task libapi.Task) error {
	return j.enc.Encode(printTask(task))
}

// Runs implementation.
func (j *JSON) runs(runs []api.Run) error {
	return j.enc.Encode(printRuns(runs))
}

// Run implementation.
func (j *JSON) run(run api.Run) error {
	return j.enc.Encode(printRun(run))
}

// Outputs implementation.
func (j *JSON) outputs(outputs api.Outputs) error {
	return j.enc.Encode(ojson.Value(outputs))
}

// Config implementation.
func (j *JSON) config(config api.Config) error {
	return j.enc.Encode(config)
}
