// nolint: predeclared
package print

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	libapi "github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/parameters"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/airplanedev/ojson"
	"github.com/olekukonko/tablewriter"
)

// Table implements a table formatter.
//
// Its zero-value is ready for use.
type Table struct{}

// APIKeys implementation.
func (t Table) apiKeys(apiKeys []api.APIKey) {
	tw := tablewriter.NewWriter(os.Stdout)
	tw.SetBorder(false)
	tw.SetHeader([]string{"id", "created at", "name"})

	for _, k := range apiKeys {
		tw.Append([]string{
			k.ID,
			k.CreatedAt.Format(time.RFC3339),
			k.Name,
		})
	}

	tw.Render()
}

// Tasks implementation.
func (t Table) tasks(tasks []libapi.Task) {
	tw := tablewriter.NewWriter(os.Stdout)
	tw.SetBorder(false)
	tw.SetHeader([]string{"name", "slug", "builder", "parameters"})
	tw.SetRowLine(true)
	tw.SetAutoWrapText(false)
	tw.SetCaption(true, "* indicates a required parameter")

	for _, t := range tasks {
		builder := string(t.Kind)

		var parametersStr string
		if len(t.Parameters) > 0 {
			var ps []string
			for _, p := range t.Parameters {
				var reqStr string
				if !p.Constraints.Optional {
					reqStr = "*"
				}

				var defaultStr string
				if p.Default != nil {
					defaultVal, err := parameters.APIValueToInput(p, p.Default)
					if err != nil {
						defaultVal = "<unknown>"
					}
					defaultStr = fmt.Sprintf(" (default: %s)", defaultVal)
				}

				ps = append(ps, fmt.Sprintf("- %s%s [%s]%s", p.Slug, reqStr, string(p.Type), defaultStr))
			}
			parametersStr = strings.Join(ps, "\n")
		}

		tw.Append([]string{
			t.Name,
			t.Slug,
			builder,
			parametersStr,
		})
	}

	tw.Render()
}

// Task implementation.
func (t Table) task(task libapi.Task) {
	builderStr := task.Kind

	fmt.Fprintln(os.Stdout, "Name:       ", task.Name)
	fmt.Fprintln(os.Stdout, "Slug:       ", task.Slug)
	fmt.Fprintln(os.Stdout, "Description:", task.Description)
	fmt.Fprintln(os.Stdout, "Builder:    ", builderStr)
	fmt.Fprintln(os.Stdout, "")

	if len(task.Parameters) > 0 {
		fmt.Fprintln(os.Stdout, "Task Parameters:")
		fmt.Fprintln(os.Stdout, "")
		tw := tablewriter.NewWriter(os.Stdout)
		tw.SetBorder(false)
		tw.SetHeader([]string{"name", "slug", "description", "type", "required", "default"})

		for _, p := range task.Parameters {
			requiredStr := "yes"
			if p.Constraints.Optional {
				requiredStr = "no"
			}

			defaultStr, err := parameters.APIValueToInput(p, p.Default)
			if err != nil {
				defaultStr = "<unknown>"
			}

			tw.Append([]string{
				p.Name,
				p.Slug,
				p.Desc,
				string(p.Type),
				requiredStr,
				defaultStr,
			})
		}

		tw.Render()
	}
}

// Runs implementation.
func (t Table) runs(runs []api.Run) {
	tw := tablewriter.NewWriter(os.Stdout)
	tw.SetBorder(false)
	tw.SetHeader([]string{"id", "task", "status", "created at", "ended at"})

	for _, run := range runs {
		var endedAt string

		switch {
		case run.SucceededAt != nil:
			endedAt = run.SucceededAt.Format(time.RFC3339)
		case run.FailedAt != nil:
			endedAt = run.FailedAt.Format(time.RFC3339)
		case run.CancelledAt != nil:
			endedAt = run.CancelledAt.Format(time.RFC3339)
		}

		tw.Append([]string{
			run.RunID,
			run.TaskName,
			string(run.Status),
			run.CreatedAt.Format(time.RFC3339),
			endedAt,
		})
	}

	tw.Render()
}

// Run implementation.
func (t Table) run(run api.Run) {
	t.runs([]api.Run{run})
}

// print outputs as table
func (t Table) outputs(outputs api.Outputs) {
	// Sort the output keys to match the UI.
	switch t := outputs.V.(type) {
	case *ojson.Object:
		for _, key := range t.KeyOrder() {
			fmt.Fprintln(os.Stdout, "")
			fmt.Fprintln(os.Stdout, logger.Bold(key))

			v, _ := t.Get(key)
			switch t2 := v.(type) {
			case []interface{}:
				ok, jsonObjects := parseArrayOfJsonObject(t2)
				if ok {
					printOutputTable(jsonObjects)
				} else {
					printOutputArray(t2)
				}
			default:
				fmt.Fprintln(os.Stdout, getCellValue(t2))
			}
		}
	case []interface{}:
		ok, jsonObjects := parseArrayOfJsonObject(t)
		if ok {
			printOutputTable(jsonObjects)
		} else {
			printOutputArray(t)
		}
	default:
		if v, err := json.Marshal(t); err != nil {
			handleErr(err)
		} else {
			fmt.Fprintf(os.Stdout, "%s\n", v)
		}
	}
}

func parseArrayOfJsonObject(values []interface{}) (bool, []*ojson.Object) {
	var jsonObjects []*ojson.Object
	for _, value := range values {
		switch t := value.(type) {
		case *ojson.Object:
			jsonObjects = append(jsonObjects, t)
		default:
			return false, nil
		}
	}
	return true, jsonObjects
}

func printOutputTable(objects []*ojson.Object) {
	keyMap := make(map[string]bool)
	var keyList []string
	for _, object := range objects {
		for _, key := range object.KeyOrder() {
			// add key to keyList if not already there
			if _, ok := keyMap[key]; !ok {
				keyList = append(keyList, key)
			}
			keyMap[key] = true
		}
	}

	tw := newTableWriter()
	tw.SetHeader(keyList)
	for _, object := range objects {
		values := make([]string, len(keyList))
		for i, key := range keyList {
			v, _ := object.Get(key)
			values[i] = getCellValue(v)
		}
		tw.Append(values)
	}
	tw.Render()
}

func printOutputArray(values []interface{}) {
	tw := newTableWriter()
	for _, value := range values {
		tw.Append([]string{getCellValue(value)})
	}
	tw.Render()
}

func newTableWriter() *tablewriter.Table {
	tw := tablewriter.NewWriter(os.Stdout)
	tw.SetBorder(true)
	tw.SetAutoWrapText(true)
	tw.SetColWidth(70)
	// Do not automatically uppercase table headers:
	tw.SetAutoFormatHeaders(false)
	return tw
}

func getCellValue(value interface{}) string {
	switch t := value.(type) {
	case int:
		return strconv.Itoa(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case string:
		return t
	case nil:
		return ""
	default:
		v, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(v)
	}
}

// print config as table
func (t Table) config(config api.Config) {
	// Nothing fancy, just the value
	var valueStr string
	if config.IsSecret {
		valueStr = logger.Gray("<secret value hidden>")
	} else {
		valueStr = config.Value
	}
	fmt.Fprintln(os.Stdout, valueStr)
}
