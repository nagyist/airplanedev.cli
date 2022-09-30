package expressions

import (
	"context"
	"testing"

	"github.com/airplanedev/lib/pkg/expressions/expressionsiface"
	"github.com/stretchr/testify/require"
)

func TestLookupClient(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	globals := map[string]expressionsiface.ExpressionsLookupFn{
		"foo": LookupFn(map[string]interface{}{
			"bar": "fuzzbuzz",
		}),
	}

	c := NewLookupClient()
	resp, err := c.Evaluate(ctx, []string{"foo.bar"}, expressionsiface.EvaluateOpts{Globals: globals})
	require.NoError(err)
	require.Equal("", resp.Results[0].ErrorMsg)
	require.Equal("fuzzbuzz", resp.Results[0].Output)
}
