package configs

import (
	"errors"
	"strings"

	"github.com/airplanedev/lib/pkg/api"
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

// MaterializeConfigs returns the configs that are attached to a task
func MaterializeConfigs(attached []api.ConfigAttachment, allConfigs map[string]string) map[string]string {
	configAttachments := map[string]string{}
	for _, a := range attached {
		configAttachments[a.NameTag] = allConfigs[a.NameTag]
	}
	return configAttachments
}
