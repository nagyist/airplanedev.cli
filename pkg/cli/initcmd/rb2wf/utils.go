package rb2wf

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	libapi "github.com/airplanedev/cli/pkg/cli/apiclient"
	"github.com/pkg/errors"
)

const (
	backtickQuoteStart = "__airplaneBacktickQuoteStart__"
	backtickQuoteEnd   = "__airplaneBacktickQuoteEnd__"
	quoteStart         = "__airplaneQuoteStart__"
	quoteEnd           = "__airplaneQuoteEnd__"
	noQuoteStart       = "__airplaneNoQuoteStart__"
	noQuoteEnd         = "__airplaneNoQuoteEnd__"
)

var (
	templateFuncs template.FuncMap = template.FuncMap{
		"commentLines": commentLines,
		"jsObj":        jsObj,
		"jsStr":        jsStr,
	}
)

func applyTemplate(t string, data interface{}) (string, error) {
	tmpl, err := template.New("airplane").Funcs(templateFuncs).Parse(t)
	if err != nil {
		return "", errors.Wrap(err, "parsing template")
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", errors.Wrap(err, "executing template")
	}

	return buf.String(), nil
}

func commentLines(input string) string {
	inputLines := strings.Split(input, "\n")
	outputLines := []string{}

	for _, inputLine := range inputLines {
		outputLines = append(outputLines, fmt.Sprintf("// %s", inputLine))
	}

	return strings.Join(outputLines, "\n")
}

func jsObj(obj interface{}) (string, error) {
	resultBytes, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	result := string(resultBytes)

	// Convert quote placeholders to actual quotes
	result = strings.ReplaceAll(result, fmt.Sprintf("\"%s", noQuoteStart), "")
	result = strings.ReplaceAll(result, fmt.Sprintf("%s\"", noQuoteEnd), "")
	result = strings.ReplaceAll(result, fmt.Sprintf("\"%s", quoteStart), "\"")
	result = strings.ReplaceAll(result, fmt.Sprintf("%s\"", quoteEnd), "\"")
	result = strings.ReplaceAll(result, fmt.Sprintf("\"%s", backtickQuoteStart), "`")
	result = strings.ReplaceAll(result, fmt.Sprintf("%s\"", backtickQuoteEnd), "`")
	return result, nil
}

func jsStr(obj interface{}) (string, error) {
	switch v := obj.(type) {
	case templateValue:
		templateContents := v.toTemplate()
		if strings.Contains(templateContents, "${") {
			// Use backtick quotes
			return fmt.Sprintf("`%s`", templateContents), nil
		}

		// Use regular quotes
		return fmt.Sprintf("\"%s\"", templateContents), nil
	default:
		return fmt.Sprintf("\"%v\"", obj), nil
	}
}

type templateValue struct {
	rawTemplate string
}

func (t templateValue) isFullTemplate() bool {
	return strings.HasPrefix(t.rawTemplate, "{{") &&
		strings.HasSuffix(t.rawTemplate, "}}")
}

func (t templateValue) MarshalJSON() ([]byte, error) {
	if t.isFullTemplate() {
		return []byte(
			fmt.Sprintf(
				"\"%s%s%s\"",
				noQuoteStart,
				t.rawTemplate[2:len(t.rawTemplate)-2],
				noQuoteEnd,
			),
		), nil
	}

	templateContents := t.toTemplate()

	if strings.Contains(templateContents, "${") {
		// Need to use backtick quotes
		return []byte(
			fmt.Sprintf(
				"\"%s%s%s\"",
				backtickQuoteStart,
				templateContents,
				backtickQuoteEnd,
			),
		), nil
	}

	// Use regular quotes
	return []byte(
		fmt.Sprintf(
			"\"%s%s%s\"",
			quoteStart,
			templateContents,
			quoteEnd,
		),
	), nil
}

// Show this message when a feature could not be mapped directly and requires
// user intervention.
var manualFixPlaceholder = "FIX_ME"

var runbookEnvVars = map[string]string{
	"session.id":            "process.env.AIRPLANE_RUN_ID",
	"session.url":           "process.env.AIRPLANE_RUN_URL",
	"session.name":          "process.env.AIRPLANE_RUN_ID",
	"session.creator.id":    "process.env.AIRPLANE_RUNNER_ID",
	"session.creator.email": "process.env.AIRPLANE_RUNNER_EMAIL",
	"session.creator.name":  "process.env.AIRPLANE_RUNNER_NAME",
	"runbook.id":            "process.env.AIRPLANE_TASK_ID",
	"runbook.url":           "process.env.AIRPLANE_TASK_URL",
	"runbook.name":          "process.env.AIRPLANE_TASK_NAME",
	"env.id":                "process.env.AIRPLANE_ENV_ID",
	"env.slug":              "process.env.AIRPLANE_ENV_SLUG",
	"env.name":              "process.env.AIRPLANE_ENV_NAME",
	"env.is_default":        "process.env.AIRPLANE_ENV_IS_DEFAULT",

	"block.id":   "process.env." + manualFixPlaceholder,
	"block.slug": "process.env." + manualFixPlaceholder,
}

// transformTemplate transforms JSTs from a format that works in runbooks to what works
// inside of a task.
func transformTemplate(tmpl string, runbookInfo runbookInfo) string {
	// Transform JST globals into their corresponding environment variable.
	for envVar, processEnv := range runbookEnvVars {
		tmpl = strings.ReplaceAll(tmpl, envVar, processEnv)
	}

	// The semantics for accessing form values is different in tasks than runbooks.
	// In tasks, we return the values directly while in runbooks we return them under a
	// `.output` field.
	for _, block := range runbookInfo.blocks.Blocks {
		if block.BlockKind != "form" {
			continue
		}

		// Replace `form1.output.foo` -> `form1.foo`
		tmpl = strings.ReplaceAll(tmpl, block.Slug+".output", block.Slug)
	}

	return tmpl
}

func (t templateValue) toTemplate() string {
	template := strings.ReplaceAll(t.rawTemplate, "{{", "${")
	return strings.ReplaceAll(template, "}}", "}")
}

func interfaceToJSObj(
	obj interface{},
	removeKeys map[string]struct{},
	runbookInfo runbookInfo,
) (interface{}, error) {
	switch v := obj.(type) {
	case map[string]interface{}:
		airplaneType := v["__airplaneType"]
		if airplaneType == "template" {
			rawTemplate, ok := v["raw"].(string)
			if !ok {
				return nil, errors.Errorf("got non-string raw value: %+v", v["raw"])
			}

			return templateValue{transformTemplate(rawTemplate, runbookInfo)}, nil
		} else {
			result := map[string]interface{}{}
			for key, value := range v {
				if removeKeys != nil {
					if _, ok := removeKeys[key]; ok {
						continue
					}
				}

				var err error
				result[key], err = interfaceToJSObj(value, removeKeys, runbookInfo)
				if err != nil {
					return nil, err
				}
			}
			return result, nil
		}
	case []interface{}:
		items := []interface{}{}
		for _, obj := range v {
			item, err := interfaceToJSObj(obj, removeKeys, runbookInfo)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	}

	return obj, nil
}

func getParamType(paramType libapi.Type, component libapi.Component) string {
	if component == libapi.ComponentEditorSQL {
		return "sql"
	} else if component == libapi.ComponentTextarea {
		return "longtext"
	} else if paramType == libapi.TypeString {
		return "shorttext"
	} else if paramType == libapi.TypeInteger {
		return "integer"
	} else if paramType == libapi.TypeFloat {
		return "float"
	}
	// date, datetime, bool are the same as their types
	return string(paramType)
}

func firstValue(values map[string]string) string {
	for _, value := range values {
		return value
	}

	return ""
}

func constraintsToMap(constraints libapi.RunConstraints) map[string]string {
	constraintsMap := map[string]string{}

	for _, label := range constraints.Labels {
		// TODO: Support templating
		constraintsMap[label.Key] = label.Value
	}

	return constraintsMap
}
