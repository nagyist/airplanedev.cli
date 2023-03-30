package sql

import (
	"context"
	"os"
	"testing"

	"github.com/airplanedev/cli/pkg/deploy/taskdir"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/stretchr/testify/require"
)

func TestUpdateQuery(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	l := &logger.MockLogger{}

	d, err := os.MkdirTemp("", "runtime-sql-update-*.")
	require.NoError(err)
	t.Cleanup(func() {
		require.NoError(os.RemoveAll(d))
	})

	{
		contents, err := os.ReadFile("./fixtures/update/sql.task.yaml")
		require.NoError(err)
		require.NoError(os.WriteFile(d+"/sql.task.yaml", contents, 0655))
	}
	{
		contents, err := os.ReadFile("./fixtures/update/entrypoint.sql")
		require.NoError(err)
		require.NoError(os.WriteFile(d+"/entrypoint.sql", contents, 0655))
	}

	r := Runtime{}
	canUpdate, err := r.CanUpdate(ctx, l, d+"/sql.task.yaml", "my_task")
	require.NoError(err)
	require.True(canUpdate)

	td, err := taskdir.Open(d + "/sql.task.yaml")
	require.NoError(err)
	def, err := td.ReadDefinition()
	require.NoError(err)

	def.SQL.TestingOnlySetQuery("SELECT TRUE;")

	err = r.Update(ctx, l, d+"/sql.task.yaml", "my_task", def)
	require.NoError(err)

	def, err = td.ReadDefinition()
	require.NoError(err)

	query, err := def.SQL.GetQuery()
	require.NoError(err)
	require.Equal("SELECT TRUE;", query)
}
