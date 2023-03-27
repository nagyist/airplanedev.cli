package autopilot

import (
	"context"
	"net/http"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/server/state"
	libhttp "github.com/airplanedev/lib/pkg/api/http"
)

type Action string

const (
	Create Action = "create"
	Insert Action = "insert"
)

type Subject string

const (
	Task Subject = "task"
	View Subject = "view"
)

type SubjectKind string

const (
	SQL       SubjectKind = "sql"
	Component SubjectKind = "component"
)

type GenerateRequest struct {
	Prompt  string          `json:"prompt,omitempty"`
	Context GenerateContext `json:"context,omitempty"`
}

type GenerateContext struct {
	Action              Action               `json:"action"`
	Subject             Subject              `json:"subject"`
	SubjectKind         SubjectKind          `json:"subjectKind"`
	GenerateSQLContext  *GenerateSQLContext  `json:"sql"`
	GenerateViewContext *GenerateViewContext `json:"view"`
}

type GenerateSQLContext struct {
	ResourceID   string `json:"resourceID"`
	ResourceSlug string `json:"resourceSlug"`
}

type GenerateViewContext struct {
	CursorPosition int                   `json:"cursorPosition"`
	Kind           api.ViewComponentKind `json:"kind"`
	Code           string                `json:"code"`
}

type GenerateResponse struct {
	Content string `json:"content"`
}

func GenerateHandler(ctx context.Context, s *state.State, r *http.Request, req GenerateRequest) (GenerateResponse, error) {
	autopilotContext := req.Context
	switch autopilotContext.Action {
	case Create:
		return GenerateResponse{}, create(ctx, s, req.Prompt, autopilotContext)
	case Insert:
		content, err := insert(ctx, s, req.Prompt, autopilotContext)
		if err != nil {
			return GenerateResponse{}, err
		}
		return GenerateResponse{
			Content: content,
		}, nil
	default:
		return GenerateResponse{}, libhttp.NewErrBadRequest("unsupported action")
	}
}
