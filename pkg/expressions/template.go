package expressions

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/airplanedev/lib/pkg/expressions/expressionsiface"
	"github.com/pkg/errors"
)

// Template is an expression-based templating language. Currently, only JS expressions
// are supported. An example template might look like this:
//
//	Hello, {{ outputs.name ?? "friend" }}!
//
// The template itself, the string above, is a string that can contain zero or
// more JS expressions. Each JS expression is wrapped in double curlys.
//
// A template can be created and evaluated like so:
//
//	tmpl, err := NewTemplate(`Hello, {{ ["friend", "!"].join(" ") }}!`)
//	esvc := // an ap.ExpressionsService implementation
//	result, err := tmpl.Evaluate(ctx, esvc, ap.EvaluateOpts{})
//	fmt.Printf("output: %s", result.Output)
//
// You will need to provide an ExpressionsService implementation which will be
// consulted to evaluate any JS expressions found in the template. If any
// expression errors, a list of errors will be returned (one per errored expression).
// Each error will include the indexes of the expression that errored, along with
// the error returned by the underlying ExpressionsService implementation. An error
// will only be returned directly from Evaluate if an unexpected error occurred while
// templating and should be treated as a 5xx-style error.
//
// To check if a template is valid, call `.Validate()` on a `Template`. If an invalid
// template is used for evaluation, a validation error will be returned.
//
// Templates are most often used to produce strings, such as in the example
// above. However, if a template consists solely of a JS expression, then the
// output of the template will match that of the underlying expression. f.e.:
//
//	{{ outputs.email_list }}
//
// Would produce the following:
//
//	Result {
//	  Output: ["pam@airplane.dev", "jim@airplane.dev"]
//	}
//
// If a JS expression returns a non-string value, and that expression is not the
// sole fragment within the template, then the value will be best-effort casted
// to a string. For example, the integer 123 would be inserted as "123". If an
// expression value is returned that cannot be serialized as a string, then that
// expression will be treated as errored.
//
// Note that a template with whitespace such as " {{ outputs.email_list }}" would
// lead to the casting behavior as described in the previous paragraph. It's the
// responsibility of upstream callers to decide if they want to trim this whitespace
// before calling `NewTemplate`.
//
// To handle cases where users want to include curlys in raw text fragments, the templating
// engine supports escaping. Curlys can be escaped with backlashes (e.g. `\}` or `\{`) and
// backslashes can be escaped with backslashes (e.g. `\\`).
//
// To handle cases where users want to include curlys in expression fragments, the templating
// engine does a best-effort attempt to parse JS syntax and ignore unrelated curlys. Escaping
// rules for raw text fragments are not handled here. For now, this best-effort attempt
// includes ignoring evenly-matched curlys (iow, "for every opening curly, a closing curly will be
// ignored") and curlys inside of strings (specificed with single quotes, double quotes, or
// backticks). In the future, this logic will likely be expanded upon, f.e. to handle comments
// and regular expressions. If this parsing logic fails to find a closing `}}`, which happens
// when invalid JS is included in a template expression, it will pick the last `}}` it found
// instead else it will return an error. This optimizes for the common case, where a template
// includes just one expression fragment.
type Template struct {
	Raw string

	fragments []fragment
	err       error
}

var _ fmt.Stringer = Template{}
var _ json.Marshaler = Template{}
var _ json.Unmarshaler = &Template{}

type fragment struct {
	expression string
	raw        string
	start      int
	end        int
}

type Result struct {
	Output interface{} `json:"output"`
	Errors []Error     `json:"errors"`
}

type Error struct {
	Start int
	End   int
	Msg   string
}

func (e Error) String() string {
	return fmt.Sprintf("%s (%d:%d)", e.Msg, e.Start, e.End)
}

type quoteMode int

const (
	quoteModeNone quoteMode = iota
	quoteModeSingle
	quoteModeDouble
	quoteModeBacktick
)

var errInvalidTemplate = errors.New("invalid template: found a {{ without a }}")

// NewTemplate parses the provided template and returns an error if the
// template is invalid.
func NewTemplate(s string) Template {
	t := Template{
		Raw:       s,
		fragments: []fragment{},
	}

	var ci int                           // capture index: we've captured runes from [0, ci)
	runes := []rune(s)                   // operate on runes not bytes: https://go.dev/blog/strings
	var next []rune                      // next fragment to capture
	for li := 0; li < len(runes); li++ { // lookahead index
		r := runes[li] // current rune

		isLast := li+1 == len(runes)
		if isLast {
			next = append(next, r)
			continue
		}

		nr := runes[li+1] // next rune

		switch r {
		case '{':
			// If this is not a `{{`, treat it as a raw text.
			if nr != '{' {
				next = append(next, r)
				continue
			}

			// Otherwise, we found a `{{`. Start processing a template expression.
			// First off, go ahead and capture everything before the `{{` as a fragment.
			if len(next) > 0 {
				t.fragments = append(t.fragments, fragment{
					raw:   string(next),
					start: ci,
					end:   li,
				})
				next = []rune{}
			}

			// Next, get the length of the template expression and extract it as a fragment.
			n, err := measureTemplateExpression(runes[li:])
			if err != nil {
				t.err = err
				return t
			}
			t.fragments = append(t.fragments, fragment{
				expression: strings.TrimSpace(string(runes[li+2 : li+n+1-2])), // Trim the `{{}}`'s off
				raw:        string(runes[li : li+n+1]),
				start:      li,
				end:        li + n + 1,
			})
			ci = li + n + 1
			li += n
		case '\\':
			// The backslash is used to escape special runes. To check for this, we inspect the next rune.
			switch nr {
			case '{', '}', '\\':
				// "Escape" the next rune so it is treated as raw text by capturing `nr`.
				next = append(next, nr) // capture `nr` and ignore `r`
				li++                    // skip `nr`
			default:
				// This is not a valid escape sequence, so we treat the `\` as raw text.
				next = append(next, r)
			}
		default:
			// This is not a special rune, so just capture it like normal.
			// Note this includes the `}` rune which we only look for after finding a `{{`.
			next = append(next, r)
		}
	}

	// Capture any remaining text as a final raw fragment.
	if len(next) > 0 {
		t.fragments = append(t.fragments, fragment{
			raw:   string(next),
			start: ci,
			end:   len(runes),
		})
	}

	return t
}

// measureTemplateExpression returns the length of a template expression within `runes`. The first
// two runes in `runes` must both be opening curly brackets. If there are no valid closing double
// curlys in `runes`, an error will be returned. Otherwise, the length of the template expression
// (which includes both the opening and closing double curlys) will be returned.
func measureTemplateExpression(runes []rune) (int, error) {
	// Next, we'll traverse forward until we reach the `}}` that end this template expression.
	// However, the contents of this template expression could contain unrelated curlys.
	// To handle this, maintain the current depth of curlys so we can ignore them.
	depth := 0
	// In certain cases, we ignore curlys. Specifically, if a curly is inside of a JS string
	// then we ignore it. We may expand this in the future.
	qm := quoteModeNone
	// If a template expression contains invalid JS, we'll "close" the template expression
	// with the last `}}` we find, if any.
	var pr rune   // previous rune
	lastdci := -1 // last double curly index
	// Start our traversal at 2 to skip past the two opening curlys.
	for i := 2; i < len(runes); i++ {
		r := runes[i]

		switch r {
		case '\\':
			if i+1 < len(runes) {
				nr := runes[i+1]
				switch nr {
				case '"', '\'', '`':
					// If we find an escaped quote, skip it in the next pass so it won't update `qm`.
					i++
				}
			}
		case '"':
			switch qm {
			case quoteModeNone:
				qm = quoteModeDouble
			case quoteModeDouble:
				qm = quoteModeNone
			}
		case '\'':
			switch qm {
			case quoteModeNone:
				qm = quoteModeSingle
			case quoteModeSingle:
				qm = quoteModeNone
			}
		case '`':
			switch qm {
			case quoteModeNone:
				qm = quoteModeBacktick
			case quoteModeBacktick:
				qm = quoteModeNone
			}
		case '{':
			// Ignore curlys inside of strings:
			if qm == quoteModeNone {
				depth++
			}
		case '}':
			// Ignore curlys inside of strings:
			if qm == quoteModeNone {
				depth--
			}

			if pr == '}' {
				// We've found the double closing curlys.
				lastdci = i
				if depth == -2 {
					// These curlys match with the original curlys.
					return i, nil
				}
			}
		}

		pr = r
	}

	// We were not able to find a closing double curly.
	// If we found as least one double curly, treat that as the end of the template expression.
	if lastdci > -1 {
		return lastdci, nil
	}
	// Otherwise, return an error.
	return 0, errInvalidTemplate
}

// Validate returns an error if `t` is an invalid template.
func (t Template) Validate() error {
	return t.err
}

// Evaluate evaluates the parsed template and uses the provided `svc` to evaluate
// any JS expressions in the template. An error is returned if an unexpected error
// occurs while evaluating the template. If the `svc` indicates that an expression
// produced an error, then that error will be returned as part of `Result`.
func (t Template) Evaluate(ctx context.Context, esvc expressionsiface.ExpressionsService, opts expressionsiface.EvaluateOpts) (Result, error) {
	if esvc == nil {
		return Result{}, errors.New("ExpressionsService is not set")
	}
	if err := t.Validate(); err != nil {
		return Result{
			Errors: []Error{
				{Start: 0, End: len(t.Raw), Msg: err.Error()},
			},
		}, nil
	}

	if len(t.fragments) == 0 {
		return Result{
			Output: "",
		}, nil
	}

	exprs := []string{}
	for _, f := range t.fragments {
		if f.expression != "" {
			exprs = append(exprs, f.expression)
		}
	}

	if len(exprs) == 0 {
		// If there are no expressions, then there was only a single fragment.
		return Result{
			Output: t.fragments[0].raw,
		}, nil
	}

	resp, err := esvc.Evaluate(ctx, exprs, opts)
	if err != nil {
		return Result{}, err
	}

	// If the template is a single expression, then we can return a non-string value.
	if len(exprs) == 1 && len(t.fragments) == 1 {
		f := t.fragments[0]
		sr := resp.Results[0]
		if sr.ErrorMsg != "" {
			return Result{
				Output: "",
				Errors: []Error{
					{
						Start: f.start,
						End:   f.end,
						Msg:   sr.ErrorMsg,
					},
				},
			}, nil
		}

		return Result{
			Output: sr.Output,
		}, nil
	}

	var si int // expression index
	str := strings.Builder{}
	result := Result{}
	for _, f := range t.fragments {
		if f.expression == "" {
			if _, err := str.WriteString(f.raw); err != nil {
				return Result{}, errors.Wrap(err, "writing string to builder")
			}
			continue
		}

		sr := resp.Results[si]
		si++
		if sr.ErrorMsg != "" {
			result.Errors = append(result.Errors, Error{
				Start: f.start,
				End:   f.end,
				Msg:   sr.ErrorMsg,
			})
			continue
		}

		// Cast the expression's output to a string:
		switch o := sr.Output.(type) {
		case nil:
			// nil returns are treated as empty strings.
		case string:
			if _, err := str.WriteString(o); err != nil {
				return Result{}, errors.Wrap(err, "writing string to builder")
			}
		case int:
			if _, err := str.WriteString(strconv.FormatInt(int64(o), 10)); err != nil {
				return Result{}, errors.Wrap(err, "converting output to integer")
			}
		case float64:
			if _, err := str.WriteString(strconv.FormatFloat(o, 'f', -1, 64)); err != nil {
				return Result{}, errors.Wrap(err, "converting output to float")
			}
		case bool:
			if _, err := str.WriteString(strconv.FormatBool(o)); err != nil {
				return Result{}, errors.Wrap(err, "converting output to boolean")
			}
		default:
			out, err := json.Marshal(sr.Output)
			if err != nil {
				err = errors.Wrap(err, "serializing output as JSON")
				result.Errors = append(result.Errors, Error{
					Start: f.start,
					End:   f.end,
					Msg:   err.Error(),
				})
				continue
			}
			if _, err := str.Write(out); err != nil {
				return Result{}, errors.Wrap(err, "writing serialized output to buffer")
			}
		}
	}

	// Produce an output regardless of errors - errored templates will resolve
	// into empty strings.
	result.Output = str.String()

	return result, nil
}

func (t Template) String() string {
	return t.Raw
}

func (t Template) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"__airplaneType": "template",
		"raw":            t.Raw,
	})
}

func (t *Template) UnmarshalJSON(b []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	var ok bool
	*t, ok = AsTemplate(m)
	if !ok {
		return errors.New("unable to unmarshal template")
	}

	return nil
}

func AsTemplate(v interface{}) (Template, bool) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return Template{}, false
	}
	if m["__airplaneType"] != "template" {
		return Template{}, false
	}
	raw, _ := m["raw"].(string)

	return NewTemplate(raw), true
}
