package resource

var ResourceKindREST ResourceKind = "rest"

func init() {
	RegisterBaseResourceFactory(ResourceKindREST, func() Resource { return RESTResource{} })
}

type RESTResource struct {
	BaseResource `mapstructure:",squash"`

	BaseURL       string            `json:"baseURL" mapstructure:"baseURL"`
	Headers       map[string]string `json:"headers" mapstructure:"headers"`
	SecretHeaders []string          `json:"secretHeaders" mapstructure:"secretHeaders"`
}

func (r RESTResource) Kind() ResourceKind {
	return r.BaseResource.Kind
}
