package logger

import "fmt"

var _ LoggerWithLoader = &MockLogger{}

type MockLogger struct {
}

func (l *MockLogger) Log(msg string, args ...interface{}) {
	fmt.Printf(msg, args...)
	fmt.Println()
}

func (l *MockLogger) Debug(msg string, args ...interface{}) {
	fmt.Printf(msg, args...)
	fmt.Println()
}

func (l *MockLogger) Warning(msg string, args ...interface{}) {
	fmt.Printf(msg, args...)
	fmt.Println()
}

func (l *MockLogger) Error(msg string, args ...interface{}) {
	fmt.Printf(msg, args...)
	fmt.Println()
}

func (l *MockLogger) Step(msg string, args ...interface{}) {
	fmt.Printf(msg, args...)
	fmt.Println()
}

func (l *MockLogger) Suggest(title, command string, args ...interface{}) {
	fmt.Println(title)
	fmt.Printf(command, args...)
	fmt.Println()
}

func (l *MockLogger) SuggestSteps(title string, steps ...string) {
	fmt.Println(title)
	for _, step := range steps {
		fmt.Println(step)
	}
	fmt.Println()
}

func (l *MockLogger) StopLoader() bool {
	return false
}

func (l *MockLogger) StartLoader() {
}
