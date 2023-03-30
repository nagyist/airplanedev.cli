package typescript

import (
	"bytes"
	"fmt"
	"io/fs"
	"text/template"

	"github.com/airplanedev/cli/pkg/runtime"
	"github.com/airplanedev/cli/pkg/runtime/javascript"
)

// Init register the runtime.
func init() {
	runtime.Register(".ts", Runtime{})
	runtime.Register(".tsx", Runtime{})
}

// Code template.
var code = template.Must(template.New("ts").Parse(`{{with .Comment -}}
{{.}}

{{end -}}
type Params = {
  {{- range .Params }}
  {{ .Name }}: {{ .Type }}
  {{- end }}
}

// This is your task's entrypoint. When your task is executed, this
// function will be called.
export default async function(params: Params) {
	const data = [
		{ id: 1, name: "Gabriel Davis", role: "Dentist" },
		{ id: 2, name: "Carolyn Garcia", role: "Sales" },
		{ id: 3, name: "Frances Hernandez", role: "Astronaut" },
		{ id: 4, name: "Melissa Rodriguez", role: "Engineer" },
		{ id: 5, name: "Jacob Hall", role: "Engineer" },
		{ id: 6, name: "Andrea Lopez", role: "Astronaut" },
	];

	// Sort the data in ascending order by name.
	data.sort((u1, u2) => {
		return u1.name.localeCompare(u2.name);
	});

	// You can return data to show output to users.
	// Output documentation: https://docs.airplane.dev/tasks/output
	return data;
}
`))

// Data represents the data template.
type data struct {
	Comment string
	Params  []param
}

// Param represents the parameter.
type param struct {
	Name string
	Type string
}

type Runtime struct {
	javascript.Runtime
}

// Generate implementation.
func (r Runtime) Generate(t *runtime.Task) ([]byte, fs.FileMode, error) {
	d := data{}
	if t != nil {
		d.Comment = runtime.Comment(r, t.URL)
		for _, p := range t.Parameters {
			d.Params = append(d.Params, param{
				Name: p.Slug,
				Type: typeof(p.Type),
			})
		}
	}

	var buf bytes.Buffer
	if err := code.Execute(&buf, d); err != nil {
		return nil, 0, fmt.Errorf("typescript: template execute - %w", err)
	}

	return buf.Bytes(), 0644, nil
}

// Typeof translates the given type to typescript.
func typeof(t runtime.Type) string {
	switch t {
	case runtime.TypeInteger, runtime.TypeFloat:
		return "number"
	case runtime.TypeDate, runtime.TypeDatetime:
		return "string"
	case runtime.TypeBoolean:
		return "boolean"
	case runtime.TypeString:
		return "string"
	case runtime.TypeUpload:
		return "string"
	default:
		return "unknown"
	}
}
