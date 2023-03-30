package utils

import (
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

func ApplyTemplate(t string, data interface{}) (string, error) {
	tmpl, err := template.New("airplane").Parse(t)
	if err != nil {
		return "", errors.Wrap(err, "parsing template")
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", errors.Wrap(err, "executing template")
	}

	return buf.String(), nil
}

func InlineString(s string) string {
	// To inline a multi-line string into a Dockerfile, insert `\n\` characters:
	s = strings.Join(strings.Split(s, "\n"), "\\n\\\n")
	// Since the string is wrapped in single-quotes, escape any single-quotes
	// inside of the target string.
	s = strings.ReplaceAll(s, "'", `'"'"'`)
	s = strings.ReplaceAll(s, "%", `%%`)
	return "printf '" + s + "'"
}

// BackslashEscape escapes s by replacing `\` with `\\` and all runes in chars with `\{rune}`.
// Typically should backslashEscape(s, `"`) to escape backslashes and double quotes.
func BackslashEscape(s string, chars string) string {
	// Always escape backslash
	s = strings.ReplaceAll(s, `\`, `\\`)
	for _, char := range chars {
		s = strings.ReplaceAll(s, string(char), `\`+string(char))
	}
	return s
}
