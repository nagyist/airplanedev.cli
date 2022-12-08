package params_test

import (
	"context"
	"testing"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/params"
	libapi "github.com/airplanedev/lib/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestStandardizeParams(t *testing.T) {
	require := require.New(t)

	upload := libapi.Upload{
		ID:        "upl1",
		FileName:  "upload1",
		SizeBytes: 10,
	}
	remoteClient := &api.MockClient{
		Uploads: map[string]libapi.Upload{
			"upl1": upload,
		},
	}

	fileParam := libapi.Parameter{
		Name: "File param",
		Slug: "file",
		Type: libapi.TypeUpload,
	}
	stringParam := libapi.Parameter{
		Name: "String param",
		Slug: "string",
		Type: libapi.TypeString,
	}
	dateParam := libapi.Parameter{
		Name: "Date param",
		Slug: "date",
		Type: libapi.TypeDate,
	}
	parameters := libapi.Parameters{fileParam, stringParam, dateParam}

	paramValues := api.Values{
		"file":        "upl1",
		"string":      "hello",
		"date":        "2006-01-02T15:04:05Z",
		"nonexistent": "nonexistent", // should not be included in standardized values
	}

	standardizedValues, err := params.StandardizeParamValues(context.Background(), remoteClient, parameters, paramValues)
	require.NoError(err)
	require.Equal(3, len(standardizedValues))
	require.Equal("hello", standardizedValues["string"]) // should not be affected by standardization
	require.Equal(map[string]interface{}{                // should be converted into a "param upload object"
		"__airplaneType": "upload",
		"id":             upload.ID,
		"url":            "fake-url",
	}, standardizedValues["file"])
	require.Equal("2006-01-02", standardizedValues["date"]) // should be converted into a date string
}
