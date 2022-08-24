package kind_configs

type SqlBaseConfig struct {
	Host       string `json:"host" yaml:"host"`
	Port       string `json:"port" yaml:"port"`
	Database   string `json:"database" yaml:"database"`
	Username   string `json:"username" yaml:"username"`
	Password   string `json:"password" yaml:"password"`
	DisableSSL bool   `json:"disableSSL" yaml:"disableSSL"`

	// Optional SSH tunneling
	SSHHost       string `json:"sshHost" yaml:"sshHost"`
	SSHPort       string `json:"sshPort" yaml:"sshPort"`
	SSHUsername   string `json:"sshUsername" yaml:"sshUsername"`
	SSHPrivateKey string `json:"sshPrivateKey" yaml:"sshPrivateKey"`
}

func (this *SqlBaseConfig) update(c SqlBaseConfig) {
	this.Host = c.Host
	this.Port = c.Port
	this.Database = c.Database
	this.Username = c.Username
	// Don't clobber over password if empty
	if c.Password != "" {
		this.Password = c.Password
	}
	this.DisableSSL = c.DisableSSL

	this.SSHHost = c.SSHHost
	this.SSHPort = c.SSHPort
	this.SSHUsername = c.SSHUsername
	// Don't clobber over SSH private key if empty
	if c.SSHPrivateKey != "" {
		this.SSHPrivateKey = c.SSHPrivateKey
	}
}
