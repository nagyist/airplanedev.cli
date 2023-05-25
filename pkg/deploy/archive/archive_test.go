package archive

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/cli/apiclient/mock"
	"github.com/airplanedev/cli/pkg/utils/logger"
	"github.com/stretchr/testify/require"
)

func TestArchive(t *testing.T) {
	require := require.New(t)
	fixturesPath, _ := filepath.Abs("./fixtures")

	testCases := []struct {
		desc        string
		roots       []string
		numUploaded int
	}{
		{
			desc:        "Archives one task",
			roots:       []string{fixturesPath},
			numUploaded: 1,
		},
		{
			desc:        "Archives two tasks, same root",
			roots:       []string{fixturesPath, fixturesPath},
			numUploaded: 1,
		},
		{
			desc:        "Archives two tasks, diff roots",
			roots:       []string{fixturesPath, fixturesPath + "/nested"},
			numUploaded: 2,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			l := &logger.MockLogger{}
			client := &mock.MockClient{}
			uploader := &MockUploader{}
			archiver := NewAPIArchiver(l, client, uploader)

			var numUploaded int
			for _, root := range tC.roots {
				_, sizeBytes, err := archiver.Archive(context.Background(), root)
				require.NoError(err)
				numUploaded++
				if numUploaded > tC.numUploaded {
					require.Empty(sizeBytes)
				}
			}
			require.Equal(tC.numUploaded, uploader.UploadCount)
		})
	}
}
