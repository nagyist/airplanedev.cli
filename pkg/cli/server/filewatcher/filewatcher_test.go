package filewatcher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilter(tt *testing.T) {
	tests := []struct {
		desc   string
		path   string
		expect bool
	}{
		{
			desc: ".airplane directory",
			path: ".airplane",
		},
		{
			desc: ".airplane-view directory",
			path: ".airplane-view",
		},
		{
			desc: ".gitignore",
			path: ".gitignore",
		},
		{
			desc: "file inside pycache",
			path: "/folder/__pycache__/something_airplane.py",
		},
		{
			desc: "file inside ignored .airplane directory",
			path: "root/.airplane/what/views_2.view.js",
		},
		{
			desc: "file inside ignored .airplane-view directory",
			path: "root/.airplane-view/myviews/my_inline_view.view.jsx",
		},
		{
			desc: "file inside ignored node modules directory",
			path: "node_modules/what/my_view.view.js",
		},
		{
			desc: "non inline view is ignored",
			path: "root/what/views_2.view.js",
		},
		{
			desc: "non inline JS task is ignored",
			path: "root/what/my_inline_task.ts",
		},
		{
			desc:   "inline JS task",
			path:   "root/what/my_inline_task.airplane.ts",
			expect: true,
		},
		{
			desc:   "inline view tsx",
			path:   "root/what/my_inline_view.airplane.tsx",
			expect: true,
		},

		{
			desc:   "task config yaml",
			path:   "capitalize.task.yaml",
			expect: true,
		},
		{
			desc:   "nested task config",
			path:   "/airplane/capitalize.task.yaml",
			expect: true,
		},
		{
			desc:   "SQL file",
			path:   "/airplane/query1.sql",
			expect: true,
		},
	}
	for _, test := range tests {
		tt.Run(test.desc, func(t *testing.T) {
			res := IsValidDefinitionFile(test.path)
			require.Equal(t, test.expect, res)
		})
	}
}
