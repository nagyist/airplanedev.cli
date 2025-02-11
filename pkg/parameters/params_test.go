package parameters_test

import (
	"context"
	"testing"

	libapi "github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/api/cliapi"
	"github.com/airplanedev/cli/pkg/parameters"
	"github.com/stretchr/testify/require"
)

func TestApplyDefaults(t *testing.T) {
	require := require.New(t)
	params := libapi.Parameters{libapi.Parameter{
		Name:    "Param 1",
		Slug:    "p1",
		Type:    libapi.TypeString,
		Default: "Eric",
	}, libapi.Parameter{
		Name:    "Param 2",
		Slug:    "p2",
		Type:    libapi.TypeString,
		Default: "Erica",
	}}
	paramValues := api.Values{
		"p2": "Erie",
	}
	paramValuesWithDefaults := parameters.ApplyDefaults(params, paramValues)
	require.Equal("Eric", paramValuesWithDefaults["p1"])
	require.Equal("Erie", paramValuesWithDefaults["p2"])
}

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
	params := libapi.Parameters{fileParam, stringParam, dateParam}

	paramValues := api.Values{
		"file":        "upl1",
		"string":      "hello",
		"date":        "2006-01-02T15:04:05Z",
		"nonexistent": "nonexistent", // should not be included in standardized values
	}

	standardizedValues, err := parameters.StandardizeParamValues(context.Background(), remoteClient, params, paramValues)
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
