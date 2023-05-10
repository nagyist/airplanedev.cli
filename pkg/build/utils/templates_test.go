package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInlineString(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		expected string
	}{
		{
			desc:     "escapes single quotes",
			input:    `The sheep couldn't sleep, no matter how many humans he counted.`,
			expected: `printf 'The sheep couldn'"'"'t sleep, no matter how many humans he counted.'`,
		},
		{
			desc:     "escapes multiple single quotes",
			input:    `'''`,
			expected: `printf ''"'"''"'"''"'"''`,
		},
		{
			desc:     "escapes percent signs",
			input:    `I am 30% better than you!`,
			expected: `printf 'I am 30%% better than you!'`,
		},
		{
			desc:     "newlines are preserved",
			input:    `hi\nline`,
			expected: `printf 'hi\nline'`,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require := require.New(t)
			require.Equal(tC.expected, InlineString(tC.input))
		})
	}
}
