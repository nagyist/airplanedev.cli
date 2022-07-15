package resource

var ResourceKindPostgres ResourceKind = "postgres"

func init() {
	RegisterBaseResourceFactory(ResourceKindPostgres, func() Resource { return PostgresResource{} })
}

type PostgresResource struct {
	BaseResource `mapstructure:",squash"`

	Username string `json:"username" mapstructure:"username"`
	Host     string `json:"host" mapstructure:"host"`
	Port     string `json:"port" mapstructure:"port"`
	Database string `json:"database" mapstructure:"database"`
	Password string `json:"password" mapstructure:"password"`
	SSLMode  string `json:"ssl" mapstructure:"ssl"`
	DSN      string `json:"dsn" mapstructure:"dsn"`

	// Optional SSH tunneling
	SSHHost       string `json:"sshHost" mapstructure:"sshHost"`
	SSHPort       string `json:"sshPort" mapstructure:"sshPort"`
	SSHUsername   string `json:"sshUsername" mapstructure:"sshUsername"`
	SSHPrivateKey string `json:"sshPrivateKey" mapstructure:"sshPrivateKey"`
}

func (r PostgresResource) Kind() ResourceKind {
	return r.BaseResource.Kind
}
