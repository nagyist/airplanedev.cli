package conversion

import (
	"github.com/airplanedev/lib/pkg/resources"
	"github.com/airplanedev/lib/pkg/resources/kind_configs"
	"github.com/airplanedev/lib/pkg/resources/kinds"
	"github.com/pkg/errors"
)

func ConvertToInternalResource(r resources.Resource) (kind_configs.InternalResource, error) {
	var base resources.BaseResource
	switch resource := r.(type) {
	case *kinds.BigQueryResource:
		base = resource.BaseResource
	case *kinds.MailgunResource:
		base = resource.BaseResource
	case *kinds.MongoDBResource:
		base = resource.BaseResource
	case *kinds.MySQLResource:
		base = resource.BaseResource
	case *kinds.PostgresResource:
		base = resource.BaseResource
	case *kinds.RedshiftResource:
		base = resource.BaseResource
	case *kinds.RESTResource:
		base = resource.BaseResource
	case *kinds.SendGridResource:
		base = resource.BaseResource
	case *kinds.SlackResource:
		base = resource.BaseResource
	case *kinds.SMTPResource:
		base = resource.BaseResource
	case *kinds.SnowflakeResource:
		base = resource.BaseResource
	case *kinds.SQLServerResource:
		base = resource.BaseResource
	default:
		return kind_configs.InternalResource{}, errors.Errorf("Unkonwn resource type %T", resource)
	}
	return kind_configs.InternalResource{
		ID:             base.ID,
		Slug:           base.Slug,
		Name:           base.Name,
		Kind:           base.Kind,
		ExportResource: r,
	}, nil
}
