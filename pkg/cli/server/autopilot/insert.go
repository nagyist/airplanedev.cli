package autopilot

import (
	"context"
	"fmt"
	"math"
	"strings"

	api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	libhttp "github.com/airplanedev/cli/pkg/cli/apiclient/http"
	"github.com/airplanedev/cli/pkg/cli/server/state"
)

const insertBlock = "{{INSERT}}"

func insert(ctx context.Context, s *state.State, prompt string, generateContext GenerateContext) (string, error) {
	switch generateContext.Subject {
	case View:
		return insertIntoView(ctx, s, prompt, generateContext)
	default:
		return "", libhttp.NewErrBadRequest("unsupported subject")
	}
}

func insertIntoView(ctx context.Context, s *state.State, prompt string, generateContext GenerateContext) (string, error) {
	switch generateContext.SubjectKind {
	case Component:
		return insertComponentIntoView(ctx, s, prompt, generateContext.GenerateViewContext)
	default:
		return "", libhttp.NewErrBadRequest("unsupported subject kind")
	}
}

func insertComponentIntoView(ctx context.Context, s *state.State, userPrompt string, context *GenerateViewContext) (string, error) {
	if context == nil {
		return "", libhttp.NewErrBadRequest("missing view context")
	}

	res, err := s.RemoteClient.AutopilotComplete(ctx, api.AutopilotCompleteRequest{
		Type:   api.ViewComponentCompletionType,
		Prompt: generateViewInsertionPrompt(userPrompt, context),
		Context: &api.CompleteContext{
			CompleteViewComponentContext: &api.CompleteViewComponentContext{
				Kind: context.Kind,
			},
		},
	})
	if err != nil {
		return "", err
	}

	indentationSpaces, insertLineLeadingSpaces := getIndentation(context.Code, context.CursorPosition)
	return indentContent(res.Content, indentationSpaces, insertLineLeadingSpaces), nil
}

func generateViewInsertionPrompt(userPrompt string, context *GenerateViewContext) string {
	var builder strings.Builder
	// We need to mention to only include the generated component in the response or else the model will attempt to
	// generate the entire file.
	builder.WriteString(fmt.Sprintf("Replace %s with a %s component that does the following: %q.", insertBlock, context.Kind, userPrompt))
	builder.WriteString(" ONLY return the generated component.")
	builder.WriteString("\n")
	builder.WriteString(addInsertBlock(context.Code, context.CursorPosition))
	builder.WriteString("\n")

	return builder.String()
}

// addInsertBlock inserts the insertBlock into the code at the given cursor position so that the model can replace it.
func addInsertBlock(code string, cursorPosition int) string {
	return code[:cursorPosition] + "\n" + insertBlock + "\n" + code[cursorPosition:]
}

// getIndentation returns the indentation level of the line before the cursor to use as the indentation level. It also
// returns the number of leading spaces on the line that the cursor is on so that we can properly indent the first line
// of the inserted content.
func getIndentation(code string, cursorPosition int) (int, int) {
	prefix := code[:cursorPosition]

	lines := strings.Split(prefix, "\n")
	if len(lines) < 2 {
		return 0, 0
	}

	// The penultimate line is what our indentation level should be.
	indentationSpaces := getNumLeadingSpaces(lines[len(lines)-2])

	// The last line is the line that the cursor is on. Track how many spaces are in front of the cursor so that we
	// can properly indent this line.
	insertLineLeadingSpaces := getNumLeadingSpaces(lines[len(lines)-1])

	return indentationSpaces, insertLineLeadingSpaces
}

// getNumLeadingSpaces returns the number of leading spaces in the given line.
// TODO: Add support for arbitrary whitespace, such as tabs.
func getNumLeadingSpaces(line string) int {
	for i, c := range line {
		if c != ' ' {
			return i
		}
	}
	return len(line)
}

// indentContent indents the generated content by the given number of spaces. We handle a special case with the first
// line.
func indentContent(content string, indentationSpaces int, insertLineLeadingSpaces int) string {
	var builder strings.Builder
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		numSpaces := indentationSpaces
		// The first line may already have spaces. We should only insert the number of spaces needed to get to the
		// indentation level.
		if i == 0 {
			numSpaces = int(math.Max(0, float64(indentationSpaces-insertLineLeadingSpaces)))
		}

		builder.WriteString(strings.Repeat(" ", numSpaces))
		builder.WriteString(line)

		if i != len(lines)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String()
}
