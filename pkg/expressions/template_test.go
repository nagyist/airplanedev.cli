package expressions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/airplanedev/lib/pkg/expressions/expressionsiface"
	"github.com/stretchr/testify/require"
)

func TestParsing(t *testing.T) {
	testCases := []struct {
		tmpl      string
		fragments []fragment
	}{
		// Curly edge cases are parsed correctly:
		{`{{}}`, []fragment{
			{raw: "{{}}", start: 0, end: 4},
		}},
		{`{{{}}}`, []fragment{
			{expression: "{}", raw: "{{{}}}", start: 0, end: 6},
		}},
		{`{}{{{}}}`, []fragment{
			{raw: "{}", start: 0, end: 2},
			{expression: "{}", raw: "{{{}}}", start: 2, end: 8},
		}},
		{`{{{"a":{"b":"c"}}}}`, []fragment{
			{expression: `{"a":{"b":"c"}}`, raw: `{{{"a":{"b":"c"}}}}`, start: 0, end: 19},
		}},
		// Curlys in quotes are ignored:
		{`{{ "{" }}`, []fragment{
			{expression: `"{"`, raw: `{{ "{" }}`, start: 0, end: 9},
		}},
		{`{{ '{' }}`, []fragment{
			{expression: `'{'`, raw: `{{ '{' }}`, start: 0, end: 9},
		}},
		{"{{ `{` }}", []fragment{
			{expression: "`{`", raw: "{{ `{` }}", start: 0, end: 9},
		}},
		// Quotes within quotes are ignored:
		{"{{ '\"`' \"'`\" `\"'` }}", []fragment{
			{expression: "'\"`' \"'`\" `\"'`", raw: "{{ '\"`' \"'`\" `\"'` }}", start: 0, end: 20},
		}},
		// Quote escapes are honored:
		{"{{ '\\'' \"\\\"\" `\\`` }}", []fragment{
			{expression: "'\\'' \"\\\"\" `\\``", raw: "{{ '\\'' \"\\\"\" `\\`` }}", start: 0, end: 20},
		}},
		// Matched double curlys are ignored
		{`{{ {"user": { "name": "john" }} }}`, []fragment{
			{expression: `{"user": { "name": "john" }}`, raw: `{{ {"user": { "name": "john" }} }}`, start: 0, end: 34},
		}},
		// Closing double curlys in strings are ignored
		{`{{ "}}" }}`, []fragment{
			{expression: `"}}"`, raw: `{{ "}}" }}`, start: 0, end: 10},
		}},
		// Invalid JS: only adjacent double curlys close a template expression
		{`{{ } } }}`, []fragment{
			{expression: `} }`, raw: `{{ } } }}`, start: 0, end: 9},
		}},
		// Invalid JS: unevenly matched curlys still close
		{`{{ {}} }}`, []fragment{
			{expression: `{}}`, raw: `{{ {}} }}`, start: 0, end: 9},
		}},
		// Invalid JS: unevenly matched curlys still close with raw text
		{`Hello {{ {}} }}!`, []fragment{
			{raw: `Hello `, start: 0, end: 6},
			{expression: `{}}`, raw: `{{ {}} }}`, start: 6, end: 15},
			{raw: `!`, start: 15, end: 16},
		}},
		// Invalid JS: multiple unevenly matched curlys still close with raw text
		{`Hello {{ {}} }}! Meet {{ "john" }}!`, []fragment{
			{raw: `Hello `, start: 0, end: 6},
			{expression: `{}} }}! Meet {{ "john"`, raw: `{{ {}} }}! Meet {{ "john" }}`, start: 6, end: 34},
			{raw: `!`, start: 34, end: 35},
		}},
		// Unmatched closing }}'s are allowed
		{`}}`, []fragment{
			{raw: `}}`, start: 0, end: 2},
		}},
		{`{{}}}}`, []fragment{
			{raw: "{{}}", start: 0, end: 4},
			{raw: "}}", start: 4, end: 6},
		}},
	}
	for _, tC := range testCases {
		t.Run(tC.tmpl, func(t *testing.T) {
			tmpl := NewTemplate(tC.tmpl)
			require.NoError(t, tmpl.Validate())
			require.Equal(t, tC.fragments, tmpl.fragments)
		})
	}
}

func TestInvalidTemplates(t *testing.T) {
	require := require.New(t)

	// Every {{ should have a closing }}.
	tmpl := NewTemplate(`{{`)
	require.Error(tmpl.Validate())
	tmpl = NewTemplate(`{{}}{{`)
	require.Error(tmpl.Validate())
}

func TestTemplate(tt *testing.T) {
	ctx := context.Background()
	c := NewLookupClient()

	for _, test := range []struct {
		Template string
		Output   interface{}
		Errors   []Error
	}{
		{
			// handles empty strings
			Template: ``,
			Output:   ``,
		},
		{
			// handles strings with no expressions
			Template: `Hello, world!`,
			Output:   `Hello, world!`,
		},
		{
			// captures expressions at end of string
			Template: `Hello, {{block1.str}}`,
			Output:   `Hello, world`,
		},
		{
			// captures expressions at beginning of string
			// handles spaces inside of expressions
			Template: `{{ block1.str }} is round`,
			Output:   `world is round`,
		},
		{
			// handles templates that are just an expression
			Template: `{{ block1.str }}`,
			Output:   `world`,
		},
		{
			// handles tabs inside of expressions
			Template: `{{	block1.str	}}`,
			Output: `world`,
		},
		{
			// handles multiple expressions
			Template: `{{	block1.str	}} {{block1.str}} {{block1.empty}}`,
			Output: `world world `,
		},
		{
			// solo expressions can return non-string outputs: nulls
			Template: `{{block1.null}}`,
			Output:   nil,
		},
		{
			// solo expressions can return non-string outputs: integers
			Template: `{{block1.int}}`,
			Output:   10,
		},
		{
			// solo expressions can return non-string outputs: floats
			Template: `{{block1.float}}`,
			Output:   1.23,
		},
		{
			// solo expressions can return non-string outputs: booleans
			Template: `{{block1.bool}}`,
			Output:   true,
		},
		{
			// solo expressions can return non-string outputs: arrays
			Template: `{{block1.array}}`,
			Output:   []interface{}{"hello", 123},
		},
		{
			// solo expressions can return non-string outputs: objects
			Template: `{{block1.object}}`,
			Output:   map[string]interface{}{"foo": 123},
		},
		{
			// non-string expressions are casted: ints
			Template: `There are {{block1.int}} of you.`,
			Output:   "There are 10 of you.",
		},
		{
			// non-string expressions are casted: floats
			Template: `What's bigger than 1? {{block1.float}}`,
			Output:   "What's bigger than 1? 1.23",
		},
		{
			// non-string expressions are casted: booleans
			Template: `Climate change: {{block1.bool}}`,
			Output:   "Climate change: true",
		},
		{
			// non-string expressions are casted: arrays
			Template: `Climate change: {{block1.array}}`,
			Output:   `Climate change: ["hello",123]`,
		},
		{
			// non-string expressions are casted: objects
			Template: `Climate change: {{block1.object}}`,
			Output:   `Climate change: {"foo":123}`,
		},
		{
			// failed expressions err and return correct positions, while also
			// squashing invalid templates into empty strings
			Template: `this is a {{oops}} with another {{ again }}`,
			Output:   `this is a  with another `,
			Errors: []Error{
				{Start: 10, End: 18, Msg: `unknown global: "oops"`},
				{Start: 32, End: 43, Msg: `unknown global: "again"`},
			},
		},
		{
			// mixing failed and valid expressions evaluates the valid ones and
			// squashes the failed ones
			Template: `this is a {{oops}} with another {{block1.str}}`,
			Output:   `this is a  with another world`,
			Errors: []Error{
				{Start: 10, End: 18, Msg: `unknown global: "oops"`},
			},
		},
		{
			// Curlys can be escaped as raw text
			Template: `\{\{`,
			Output:   "{{",
		},
		{
			// Curlys can be escaped as raw text
			Template: `\}\}`,
			Output:   "}}",
		},
		{
			// Unmatched closing curlys are treated as raw text: example JSON
			Template: `{"query": {"foo": "bar"}}`,
			Output:   `{"query": {"foo": "bar"}}`,
		},
		{
			// Unmatched closing curlys are treated as raw text
			Template: `}}`,
			Output:   "}}",
		},
		{
			// Various kinds of escapes can be used.
			Template: `\\\{{ {{ block1.bool }} \\\}}`,
			Output:   `\{{ true \}}`,
		},
		{
			// Raw text with unicode works correctly
			Template: `こんにちは世界`,
			Output:   `こんにちは世界`,
		},
		{
			// Raw text with unicode works correctly
			Template: `こんにちは{{ block1["こんにちは"] }}`,
			Output:   `こんにちは世界`,
		},
		{
			// Raw text with unicode works correctly
			Template: `👋{{ block1["👋"] }}`,
			Output:   `👋🌎`,
		},
	} {
		tt.Run(test.Template, func(t *testing.T) {
			require := require.New(t)
			tmpl := NewTemplate(test.Template)
			require.NoError(tmpl.Validate())
			result, err := tmpl.Evaluate(ctx, c, expressionsiface.EvaluateOpts{
				Globals: map[string]expressionsiface.ExpressionsLookupFn{
					"block1": LookupFn(map[string]interface{}{
						"str":    "world",
						"empty":  "",
						"null":   nil,
						"int":    10,
						"float":  1.23,
						"bool":   true,
						"array":  []interface{}{"hello", 123},
						"object": map[string]interface{}{"foo": 123},
						"こんにちは":  "世界",
						"👋":      "🌎",
					}),
				},
			})
			require.NoError(err)
			require.Equal(Result{
				Output: test.Output,
				Errors: test.Errors,
			}, result)
		})
	}
}

func TestTemplateJSON(t *testing.T) {
	require := require.New(t)

	var tmpl Template
	require.NoError(json.Unmarshal([]byte(`
		{
			"__airplaneType": "template",
			"raw": "Hello, {{world}}"
		}
	`), &tmpl))
	require.NoError(tmpl.Validate())
	require.Equal("Hello, {{world}}", tmpl.Raw)

	mt, err := json.MarshalIndent(tmpl, "", "\t")
	require.NoError(err)
	require.Equal(heredoc.Doc(`
		{
			"__airplaneType": "template",
			"raw": "Hello, {{world}}"
		}
	`), string(mt)+"\n")
}
