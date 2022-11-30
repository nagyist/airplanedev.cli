package configs

import (
	"strings"

	"github.com/airplanedev/lib/pkg/api"
	"github.com/pkg/errors"
)

var ErrInvalidConfigName = errors.New("invalid config name")

type NameTag struct {
	Name string
	Tag  string
}

func ParseName(nameTag string) (NameTag, error) {
	var res NameTag
	parts := strings.Split(nameTag, ":")
	if len(parts) > 2 {
		return res, ErrInvalidConfigName
	}
	res.Name = parts[0]
	if len(parts) >= 2 {
		res.Tag = parts[1]
	}
	return res, nil
}

func JoinName(nameTag NameTag) string {
	var tagStr string
	if nameTag.Tag != "" {
		tagStr = ":" + nameTag.Tag
	}
	return nameTag.Name + tagStr
}

// MaterializeConfigAttachments returns the configs that are attached to a task
func MaterializeConfigAttachments(attachments []api.ConfigAttachment, configs map[string]string) (map[string]string, error) {
	configAttachments := map[string]string{}
	for _, a := range attachments {
		if _, ok := configs[a.NameTag]; !ok {
			return nil, errors.Errorf("config %s not defined in airplane.dev.yaml", a.NameTag)
		}

		configAttachments[a.NameTag] = configs[a.NameTag]
	}
	return configAttachments, nil
}
