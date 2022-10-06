package kinds

import (
	"encoding/base64"
	"testing"

	"github.com/airplanedev/lib/pkg/resources"
	"github.com/stretchr/testify/require"
)

func TestBigQueryResource(t *testing.T) {
	require := require.New(t)

	resource := BigQueryResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindBigQuery,
		},
		RawCredentials: `{"credentials": "foobar"}`,
		ProjectID:      "my_project_id",
		Location:       "my_location",
		DataSet:        "my_dataset",
	}
	err := resource.Calculate()
	require.NoError(err)
	err = resource.Validate()
	require.NoError(err)

	// Update without sensitive info, sensitive info should still be there.
	err = resource.Update(&BigQueryResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindBigQuery,
		},
		ProjectID: "my_new_project_id",
		Location:  "my_new_location",
		DataSet:   "my_new_dataset",
	})
	require.NoError(err)
	require.Equal("my_new_project_id", resource.ProjectID)
	require.Equal("my_new_location", resource.Location)
	require.Equal("my_new_dataset", resource.DataSet)
	require.Equal(`{"credentials": "foobar"}`, resource.RawCredentials)
	require.Equal(base64.StdEncoding.EncodeToString([]byte(resource.RawCredentials)), resource.Credentials)
	require.NotEmpty(resource.DSN)
	err = resource.Validate()
	require.NoError(err)

	// Update with sensitive info, sensitive info should be changed.
	err = resource.Update(&BigQueryResource{
		BaseResource: resources.BaseResource{
			Kind: ResourceKindBigQuery,
		},
		RawCredentials: `{"credentials": "my_new_credentials"}`,
		ProjectID:      "my_new_project_id",
		Location:       "my_new_location",
		DataSet:        "my_new_dataset",
	})
	require.NoError(err)
	require.Equal("my_new_project_id", resource.ProjectID)
	require.Equal("my_new_location", resource.Location)
	require.Equal("my_new_dataset", resource.DataSet)
	require.Equal(`{"credentials": "my_new_credentials"}`, resource.RawCredentials)
	require.Equal(base64.StdEncoding.EncodeToString([]byte(resource.RawCredentials)), resource.Credentials)
	require.NotEmpty(resource.DSN)
	err = resource.Validate()
	require.NoError(err)
}

func TestCredentialUpdating(t *testing.T) {
	rawCredentials1 := `{"foo": "bar"}`
	credentials1 := base64.StdEncoding.EncodeToString([]byte(rawCredentials1))
	rawCredentials2 := `{"foo": "baz"}`
	credentials2 := base64.StdEncoding.EncodeToString([]byte(rawCredentials2))

	for _, test := range []struct {
		name                   string
		original               *BigQueryResource
		update                 *BigQueryResource
		expectedCredentials    string
		expectedRawCredentials string
	}{
		{
			name: "no-op",
			original: &BigQueryResource{
				ProjectID:   "projectID",
				Location:    "location",
				DataSet:     "dataset",
				Credentials: credentials1,
			},
			update: &BigQueryResource{
				ProjectID: "projectID",
				Location:  "location",
				DataSet:   "dataset",
			},
			expectedRawCredentials: rawCredentials1,
			expectedCredentials:    credentials1,
		},
		{
			name: "Legacy update",
			original: &BigQueryResource{
				ProjectID:   "projectID",
				Location:    "location",
				DataSet:     "dataset",
				Credentials: credentials1,
			},
			update: &BigQueryResource{
				ProjectID:   "projectID",
				Location:    "location",
				DataSet:     "dataset",
				Credentials: rawCredentials2,
			},
			expectedRawCredentials: rawCredentials2,
			expectedCredentials:    credentials2,
		},
		{
			name: "update raw credentials",
			original: &BigQueryResource{
				ProjectID:   "projectID",
				Location:    "location",
				DataSet:     "dataset",
				Credentials: credentials1,
			},
			update: &BigQueryResource{
				ProjectID:      "projectID",
				Location:       "location",
				DataSet:        "dataset",
				RawCredentials: rawCredentials2,
			},
			expectedRawCredentials: rawCredentials2,
			expectedCredentials:    credentials2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)
			err := test.original.Update(test.update)
			require.NoError(err)
			require.Equal(test.expectedRawCredentials, test.original.RawCredentials)
			require.Equal(test.expectedCredentials, test.original.Credentials)
		})
	}
}
