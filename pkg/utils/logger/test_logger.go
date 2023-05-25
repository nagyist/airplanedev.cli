package logger

import (
	"testing"
)

var _ LoggerWithLoader = &TestLogger{}

type TestLogger struct {
	t testing.TB
}

func NewTestLogger(t testing.TB) TestLogger {
	return TestLogger{t: t}
}

func (l TestLogger) Log(msg string, args ...interface{}) {
	msg += "\n"
	if len(args) > 0 {
		l.t.Logf(msg, args...)
	} else {
		l.t.Log(msg)
	}
}

func (l TestLogger) Debug(msg string, args ...interface{}) {
	l.Log(msg, args...)
}

func (l TestLogger) Warning(msg string, args ...interface{}) {
	l.Log(msg, args...)
}

func (l TestLogger) Error(msg string, args ...interface{}) {
	l.Log(msg, args...)
}

func (l TestLogger) Step(msg string, args ...interface{}) {
	l.Log(msg, args...)
}

func (l TestLogger) Suggest(title, command string, args ...interface{}) {
	l.t.Log(title + "\n")
	l.Log(command, args...)
}

func (l TestLogger) SuggestSteps(title string, steps ...string) {
	l.t.Log(title + "\n")
	for _, step := range steps {
		l.t.Log(step + "\n")
	}
}

func (l TestLogger) StopLoader() bool {
	return false
}

func (l TestLogger) StartLoader() {
	// no-op
}
