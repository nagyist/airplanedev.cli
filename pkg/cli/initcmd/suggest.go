package initcmd

import (
	"fmt"

	buildtypes "github.com/airplanedev/cli/pkg/build/types"
	"github.com/airplanedev/cli/pkg/utils/logger"
)

type suggestNextTaskStepsRequest struct {
	logger             logger.Logger
	defnFile           string
	entrypoint         string
	showLocalExecution bool
	kind               buildtypes.TaskKind
	isNew              bool
}

func suggestNextTaskSteps(req suggestNextTaskStepsRequest) {
	// Update the next steps for inline code config
	if req.isNew {
		steps := []string{}
		switch req.kind {
		case buildtypes.TaskKindSQL:
			steps = append(steps, fmt.Sprintf("Add the name of a database resource to %s", req.defnFile))
			steps = append(steps, fmt.Sprintf("Write your query in %s", req.entrypoint))
		case buildtypes.TaskKindREST:
			steps = append(steps, fmt.Sprintf("Add the name of a REST resource to %s", req.defnFile))
			steps = append(steps, fmt.Sprintf("Specify the details of your REST request in %s", req.defnFile))
		case buildtypes.TaskKindBuiltin:
			steps = append(steps, fmt.Sprintf("Add the name of a resource to %s", req.defnFile))
			steps = append(steps, fmt.Sprintf("Specify the details of your request in %s", req.defnFile))
		case buildtypes.TaskKindImage:
			steps = append(steps, fmt.Sprintf("Add the name of a Docker image to %s", req.defnFile))
		default:
			steps = append(steps, fmt.Sprintf("Write your task logic in %s", req.entrypoint))
		}
		if req.defnFile != "" {
			steps = append(steps, fmt.Sprintf("Configure your task with parameters, a description and more in %s", req.defnFile))
		}
		req.logger.SuggestSteps("✅ To complete your task:", steps...)
	}

	file := req.defnFile
	if req.defnFile == "" {
		file = req.entrypoint
	}
	if req.showLocalExecution {
		req.logger.Suggest(
			"⚡ To develop the task locally:",
			"airplane dev %s",
			file,
		)
	}
	req.logger.Suggest(
		"🛫 To deploy your task to Airplane:",
		"airplane deploy %s",
		file,
	)
}

type suggestNextViewStepsRequest struct {
	logger  logger.Logger
	viewDir string
	slug    string
}

func suggestNextViewSteps(req suggestNextViewStepsRequest) {
	if req.viewDir != "" && req.slug != "" {
		req.logger.Suggest("✅ To complete your view:", fmt.Sprintf("Write your view logic in %s", generateViewEntrypointPath(req.slug)))
	}

	req.logger.Suggest(
		"⚡ To develop your view locally:",
		"airplane dev %s",
		req.viewDir,
	)

	var deployDir string
	if req.viewDir != "" {
		deployDir = req.viewDir
	} else {
		deployDir = "."
	}
	req.logger.Suggest(
		"🛫 To deploy your view to Airplane:",
		"airplane deploy %s",
		deployDir,
	)
}
