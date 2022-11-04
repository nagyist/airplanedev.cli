package config

import (
	_ "embed"
	"encoding/json"
	"io"
	"os"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/airplanedev/lib/pkg/build"
	"github.com/airplanedev/lib/pkg/deploy/taskdir/definitions"
	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

const (
	FileName = "airplane.yaml"
)

//go:embed schema.json
var schemaStr string

type AirplaneConfig struct {
	NodeVersion build.BuildTypeVersion `yaml:"nodeVersion,omitempty"`
	EnvVars     api.TaskEnv            `yaml:"envVars,omitempty"`
	Base        build.BuildBase        `yaml:"base,omitempty"`
}

func NewAirplaneConfigFromFile(file string) (AirplaneConfig, error) {
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
		return errors.WithStack(definitions.ErrSchemaValidation{Errors: result.Errors()})
	}

	if err = json.Unmarshal(buf, &c); err != nil {
		return err
	}
	return nil
}
