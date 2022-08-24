package conversion

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/airplanedev/lib/pkg/utils/logger"
	"github.com/pkg/errors"
)

func ConvertToInternalResource(r resources.Resource, l logger.Logger) (kind_configs.InternalResource, error) {
	switch resource := r.(type) {
	case kinds.BigQueryResource:
		return ConvertBigQueryResource(resource, l)
	case kinds.MailgunResource:
		return ConvertMailgunResource(resource, l)
	case kinds.MongoDBResource:
		return ConvertMongoDBResource(resource, l)
	case kinds.MySQLResource:
		return ConvertMySQLResource(resource, l)
	case kinds.PostgresResource:
		return ConvertPostgresResource(resource, l)
	case kinds.RedshiftResource:
		return ConvertRedshiftResource(resource, l)
	case kinds.RESTResource:
		return ConvertRESTResource(resource, l)
	case kinds.SendGridResource:
		return ConvertSendGridResource(resource, l)
	case kinds.SlackResource:
		return ConvertSlackResource(resource, l)
	case kinds.SMTPResource:
		return ConvertSMTPResource(resource, l)
	case kinds.SnowflakeResource:
		return ConvertSnowflakeResource(resource, l)
	case kinds.SQLServerResource:
		return ConvertSQLServerResource(resource, l)
	default:
		return kind_configs.InternalResource{}, errors.Errorf("Unkonwn resource type %T", resource)
	}
}
