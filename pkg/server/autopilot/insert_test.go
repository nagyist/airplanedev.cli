package autopilot

import (
	"context"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/stretchr/testify/require"
)

func TestInsertIntoView(t *testing.T) {
	require := require.New(t)

	userPrompt := "list all users"
	viewCtx := &GenerateViewContext{
		CursorPosition: 17,
		Kind:           api.ViewComponentKindTable,
		Code: `    let x = 4;
  
let y = 5;`}

	prompt := generateViewInsertionPrompt(userPrompt, viewCtx)

	s := &state.State{
		RemoteClient: &api.MockClient{
			AutopilotResponses: map[string]string{
				prompt: `let table = "hello";`,
			},
		},
	}

	content, err := insertComponentIntoView(context.Background(), s, userPrompt, viewCtx)
	require.NoError(err)
	require.Equal(`  let table = "hello";`, content)

}

func TestGetIndentation(t *testing.T) {
	testCases := []struct {
		name                            string
		code                            string
		cursorPosition                  int
		expectedIndentationLevel        int
		expectedInsertLineLeadingSpaces int
	}{
		{
			name: "no indentation",
			code: `let x = 4;
let y = 3;`,
			cursorPosition:                  11,
			expectedIndentationLevel:        0,
			expectedInsertLineLeadingSpaces: 0,
		},
		{
			name: "normal indentation",
			code: `    let x = 4;
    let y = 3;`,
			cursorPosition:                  19,
			expectedIndentationLevel:        4,
			expectedInsertLineLeadingSpaces: 4,
		},
		{
			name: "no leading indentation on insert line",
			code: `    let x = 4;
let y = 3;`,
			cursorPosition:                  15,
			expectedIndentationLevel:        4,
			expectedInsertLineLeadingSpaces: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require := require.New(t)
			indentationLevel, insertLineLeadingSpaces := getIndentation(testCase.code, testCase.cursorPosition)
			require.Equal(testCase.expectedIndentationLevel, indentationLevel)
			require.Equal(testCase.expectedInsertLineLeadingSpaces, insertLineLeadingSpaces)
		})
	}
}

func TestGetNumLeadingSpaces(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "no leading spaces",
			input:    "foo",
			expected: 0,
		},
		{
			name:     "one leading space",
			input:    " foo",
			expected: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require := require.New(t)
			require.Equal(testCase.expected, getNumLeadingSpaces(testCase.input))
		})
	}
}

func TestIndentContent(t *testing.T) {
	testCases := []struct {
		name                    string
		indentationSpaces       int
		insertLineLeadingSpaces int
		input                   string
		expected                string
	}{
		{
			name:                    "One line",
			indentationSpaces:       4,
			insertLineLeadingSpaces: 0,
			input:                   "let x = 4;",
			expected:                "    let x = 4;",
		},
		{
			name:                    "Multiple lines",
			indentationSpaces:       4,
			insertLineLeadingSpaces: 0,
			input: `let x = 4;
let y = 3;`,
			expected: `    let x = 4;
    let y = 3;`,
		},
		{
			name:                    "First line indented already",
			indentationSpaces:       4,
			insertLineLeadingSpaces: 4,
			input: `let x = 4;
let y = 3;`,
			expected: `let x = 4;
    let y = 3;`,
		},
		{
			name:                    "First line under-indented",
			indentationSpaces:       4,
			insertLineLeadingSpaces: 2,
			input: `let x = 4;
let y = 3;`,
			expected: `  let x = 4;
    let y = 3;`,
		},
		{
			name:                    "First line over-indented",
			indentationSpaces:       4,
			insertLineLeadingSpaces: 8,
			input: `let x = 4;
let y = 3;`,
			expected: `let x = 4;
    let y = 3;`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require := require.New(t)
			require.Equal(testCase.expected, indentContent(
				testCase.input,
				testCase.indentationSpaces,
				testCase.insertLineLeadingSpaces,
			))
		})
	}
}
