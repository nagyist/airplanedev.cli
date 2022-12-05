package config

import (
	_ "embed"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

const (
	FileName = "airplane.yaml"
)

//go:embed schema.json
var schemaStr string

type JavaScriptConfig struct {
	Base        string  `yaml:"base,omitempty" json:"base,omitempty"`
	NodeVersion string  `yaml:"nodeVersion,omitempty" json:"nodeVersion,omitempty"`
	EnvVars     TaskEnv `yaml:"envVars,omitempty" json:"envVars,omitempty"`
	Install     string  `yaml:"install,omitempty" json:"install,omitempty"`
	PreInstall  string  `yaml:"preinstall,omitempty" json:"preinstall,omitempty"`
	PostInstall string  `yaml:"postinstall,omitempty" json:"postinstall,omitempty"`
}

type PythonConfig struct {
	Base        string  `yaml:"base,omitempty" json:"base,omitempty"`
	EnvVars     TaskEnv `yaml:"envVars,omitempty" json:"envVars,omitempty"`
	PreInstall  string  `yaml:"preinstall,omitempty" json:"preinstall,omitempty"`
	PostInstall string  `yaml:"postinstall,omitempty" json:"postinstall,omitempty"`
}

type AirplaneConfig struct {
	Javascript JavaScriptConfig `yaml:"javascript,omitempty" json:"javascript,omitempty"`
	Python     PythonConfig     `yaml:"python,omitempty" json:"python,omitempty"`
}

func NewAirplaneConfigFromFile(fileOrDir string) (AirplaneConfig, error) {
	file := fileOrDir
	fileInfo, err := os.Stat(fileOrDir)
	if err != nil {
		return AirplaneConfig{}, err
	}
	if fileInfo.IsDir() {
		file = filepath.Join(fileOrDir, FileName)
	}

	f, err := os.Open(file)
	if err != nil {
		return AirplaneConfig{}, errors.Wrap(err, "error opening config file")
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return AirplaneConfig{}, errors.Wrap(err, "reading airplane config file")
	}
	c := &AirplaneConfig{}
	if err := c.Unmarshal(b); err != nil {
		return AirplaneConfig{}, err
	}
	return *c, nil
}

func (c *AirplaneConfig) Unmarshal(buf []byte) error {
	buf, err := yaml.YAMLToJSON(buf)
	if err != nil {
		return err
	}

	schemaLoader := gojsonschema.NewStringLoader(schemaStr)
	docLoader := gojsonschema.NewBytesLoader(buf)

	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return errors.Wrap(err, "validating schema")
	}

	if !result.Valid() {
		return errors.WithStack(ErrSchemaValidation{Errors: result.Errors()})
	}

	if err = json.Unmarshal(buf, &c); err != nil {
		return err
	}
	return nil
}
