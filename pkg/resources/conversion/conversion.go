package conversion

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

func ConvertToInternalResource(r resources.Resource) (kind_configs.InternalResource, error) {
	var internal kind_configs.InternalResource
	var err error
	switch resource := r.(type) {
	case *kinds.BigQueryResource:
		internal, err = ConvertBigQueryResource(resource)
	case *kinds.MailgunResource:
		internal, err = ConvertMailgunResource(resource)
	case *kinds.MongoDBResource:
		internal, err = ConvertMongoDBResource(resource)
	case *kinds.MySQLResource:
		internal, err = ConvertMySQLResource(resource)
	case *kinds.PostgresResource:
		internal, err = ConvertPostgresResource(resource)
	case *kinds.RedshiftResource:
		internal, err = ConvertRedshiftResource(resource)
	case *kinds.RESTResource:
		internal, err = ConvertRESTResource(resource)
	case *kinds.SendGridResource:
		internal, err = ConvertSendGridResource(resource)
	case *kinds.SlackResource:
		internal, err = ConvertSlackResource(resource)
	case *kinds.SMTPResource:
		internal, err = ConvertSMTPResource(resource)
	case *kinds.SnowflakeResource:
		internal, err = ConvertSnowflakeResource(resource)
	case *kinds.SQLServerResource:
		internal, err = ConvertSQLServerResource(resource)
	default:
		return kind_configs.InternalResource{}, errors.Errorf("Unkonwn resource type %T", resource)
	}
	if err != nil {
		return kind_configs.InternalResource{}, errors.Wrap(err, "error converting resource")
	}
	internal.ExportResource = r
	return internal, nil
}
