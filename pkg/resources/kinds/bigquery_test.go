package kinds

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

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
