package logger

type NoopLogger struct {
}

var _ Logger = &NoopLogger{}

// NewNoopLogger creates a new logger that drops all messages.
func NewNoopLogger() Logger {
	return &NoopLogger{}
}

func (l NoopLogger) Log(msg string, args ...interface{})                {}
func (l NoopLogger) Warning(msg string, args ...interface{})            {}
func (l NoopLogger) Step(msg string, args ...interface{})               {}
func (l NoopLogger) Suggest(title, command string, args ...interface{}) {}
func (l NoopLogger) SuggestSteps(title string, steps ...string)         {}
func (l NoopLogger) Debug(msg string, args ...interface{})              {}
func (l NoopLogger) Error(msg string, args ...interface{})              {}
