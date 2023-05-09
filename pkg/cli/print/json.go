package print //nolint: predeclared

import (
	"encoding/json"
	"os"

	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/airplanedev/ojson"
)

// JSON implements a JSON formatter.
type JSON struct {
	enc *json.Encoder
}

// NewJSONFormatter returns a new json formatter.
func NewJSONFormatter() *JSON {
	return &JSON{
		enc: json.NewEncoder(os.Stdout),
	}
}

// Encode allows external callers to use the same encoder
func (j *JSON) Encode(obj interface{}) {
	handleErr(j.enc.Encode(obj))
}

// APIKeys implementation.
func (j *JSON) apiKeys(apiKeys []api.APIKey) {
	handleErr(j.enc.Encode(apiKeys))
}

// Tasks implementation.
func (j *JSON) tasks(tasks []libapi.Task) {
	handleErr(j.enc.Encode(printTasks(tasks)))
}

// Task implementation.
func (j *JSON) task(task libapi.Task) {
	handleErr(j.enc.Encode(printTask(task)))
}

// Runs implementation.
func (j *JSON) runs(runs []api.Run) {
	handleErr(j.enc.Encode(printRuns(runs)))
}

// Run implementation.
func (j *JSON) run(run api.Run) {
	handleErr(j.enc.Encode(printRun(run)))
}

// Outputs implementation.
func (j *JSON) outputs(outputs api.Outputs) {
	handleErr(j.enc.Encode(ojson.Value(outputs)))
}

// Config implementation.
func (j *JSON) config(config api.Config) {
	handleErr(j.enc.Encode(config))
}
