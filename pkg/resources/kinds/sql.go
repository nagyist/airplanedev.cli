package kinds

import "github.com/airplanedev/cli/pkg/resources"

type SSHConfig struct {
	Host       string
	Port       string
	Username   string
	PrivateKey []byte
}

type SQLDriver string

type SQLResourceInterface interface {
	resources.Resource
	GetDSN() string
	GetSSHConfig() *SSHConfig
	GetSQLDriver() SQLDriver
}
