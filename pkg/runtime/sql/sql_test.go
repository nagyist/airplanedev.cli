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

	d, err := os.MkdirTemp("", "runtime-sql-update-*")
	require.NoError(err)
	t.Cleanup(func() {
		require.NoError(os.RemoveAll(d))
	})

	require.NoError(os.Mkdir(d+"/folder", 0755))
	// Add an `airplane.yaml` file, which defines the root directory for SQL tasks, so that
	// the root != dir that contains the entrypoint file.
	require.NoError(os.WriteFile(d+"/airplane.yaml", []byte(""), 0655))

	{
		contents, err := os.ReadFile("./fixtures/update/sql.task.yaml")
		require.NoError(err)
		require.NoError(os.WriteFile(d+"/folder/sql.task.yaml", contents, 0655))
	}
	{
		contents, err := os.ReadFile("./fixtures/update/entrypoint.sql")
		require.NoError(err)
		require.NoError(os.WriteFile(d+"/folder/entrypoint.sql", contents, 0655))
	}

	r := Runtime{}
	canUpdate, err := r.CanUpdate(ctx, l, d+"/folder/sql.task.yaml", "my_task")
	require.NoError(err)
	require.True(canUpdate)

	td, err := taskdir.Open(d + "/folder/sql.task.yaml")
	require.NoError(err)
	def, err := td.ReadDefinition()
	require.NoError(err)

	def.SQL.TestingOnlySetQuery("SELECT TRUE;")

	err = r.Update(ctx, l, d+"/folder/sql.task.yaml", "my_task", def)
	require.NoError(err)

	def, err = td.ReadDefinition()
	require.NoError(err)

	query, err := def.SQL.GetQuery()
	require.NoError(err)
	require.Equal("SELECT TRUE;", query)
}
