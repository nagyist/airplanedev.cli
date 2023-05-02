package deployutils

import (
	"strings"

	"github.com/airplanedev/cli/pkg/definitions"
)

func IsAirplaneEntity(filepath string) bool {
	return IsDefinitionFileAirplaneEntity(filepath) || IsInlineAirplaneEntity(filepath)
}

func IsDefinitionFileAirplaneEntity(filepath string) bool {
	return definitions.IsTaskDef(filepath) ||
		definitions.IsViewDef(filepath)
}

func IsInlineAirplaneEntity(filepath string) bool {
	return IsNodeInlineAirplaneEntity(filepath) ||
		IsPythonInlineAirplaneEntity(filepath) ||
		IsViewInlineAirplaneEntity(filepath)
}

func IsNodeInlineAirplaneEntity(filepath string) bool {
	return strings.HasSuffix(filepath, ".airplane.ts") || strings.HasSuffix(filepath, ".airplane.js") ||
		IsViewInlineAirplaneEntity(filepath)
}

func IsPythonInlineAirplaneEntity(filepath string) bool {
	return strings.HasSuffix(filepath, "_airplane.py")
}

func IsViewInlineAirplaneEntity(filepath string) bool {
	return strings.HasSuffix(filepath, ".airplane.tsx") || strings.HasSuffix(filepath, ".airplane.jsx") ||
		strings.HasSuffix(filepath, ".view.tsx") || strings.HasSuffix(filepath, ".view.jsx")
}
