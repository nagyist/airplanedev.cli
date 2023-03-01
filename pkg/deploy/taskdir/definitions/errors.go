package definitions

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

const taskDefDocURL = "https://docs.airplane.dev/tasks/task-definition"

var ErrNoEntrypoint = errors.New("No entrypoint")
var ErrNoAbsoluteEntrypoint = errors.New("No absolute entrypoint")

type errReadDefinition struct {
	msg       string
	errorMsgs []string
}

func NewErrReadDefinition(msg string, errorMsgs ...string) error {
	return errors.WithStack(errReadDefinition{
		msg:       msg,
		errorMsgs: errorMsgs,
	})
}

func (err errReadDefinition) Error() string {
	return err.msg
}

// Implements ErrorExplained
func (err errReadDefinition) ExplainError() string {
	msgs := []string{}
	msgs = append(msgs, err.errorMsgs...)
	if len(err.errorMsgs) > 0 {
		msgs = append(msgs, "")
	}
	msgs = append(msgs, fmt.Sprintf("For more information on the task definition format, see the docs:\n%s", taskDefDocURL))
	return strings.Join(msgs, "\n")
}

type ErrSchemaValidation struct {
	Errors []gojsonschema.ResultError
}

func (err ErrSchemaValidation) Error() string {
	return fmt.Sprintf("invalid format: %v", err.Errors)
}
