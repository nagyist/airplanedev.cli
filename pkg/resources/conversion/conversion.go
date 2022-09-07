package conversion

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

func ConvertToInternalResource(r resources.Resource) (kind_configs.InternalResource, error) {
	switch resource := r.(type) {
	case *kinds.BigQueryResource:
		return ConvertBigQueryResource(resource)
	case *kinds.MailgunResource:
		return ConvertMailgunResource(resource)
	case *kinds.MongoDBResource:
		return ConvertMongoDBResource(resource)
	case *kinds.MySQLResource:
		return ConvertMySQLResource(resource)
	case *kinds.PostgresResource:
		return ConvertPostgresResource(resource)
	case *kinds.RedshiftResource:
		return ConvertRedshiftResource(resource)
	case *kinds.RESTResource:
		return ConvertRESTResource(resource)
	case *kinds.SendGridResource:
		return ConvertSendGridResource(resource)
	case *kinds.SlackResource:
		return ConvertSlackResource(resource)
	case *kinds.SMTPResource:
		return ConvertSMTPResource(resource)
	case *kinds.SnowflakeResource:
		return ConvertSnowflakeResource(resource)
	case *kinds.SQLServerResource:
		return ConvertSQLServerResource(resource)
	default:
		return kind_configs.InternalResource{}, errors.Errorf("Unkonwn resource type %T", resource)
	}
}
