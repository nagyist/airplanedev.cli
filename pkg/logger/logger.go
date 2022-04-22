package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"golang.org/x/term"
)

var (
	// EnableDebug determines if debug logs are emitted.
	EnableDebug bool
)

type Logger interface {
	Log(msg string, args ...interface{})
	Warning(msg string, args ...interface{})
	Step(msg string, args ...interface{})
	Suggest(title, command string, args ...interface{})
	SuggestSteps(title string, steps ...string)
	Debug(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

type LoggerWithLoader interface {
	Logger
	StopLoader() bool
	StartLoader()
}

var _ LoggerWithLoader = &StdErrLogger{}

type StdErrLogger struct {
	loader Loader
}

type StdErrLoggerOpts struct {
	WithLoader bool
}

// NewStdErrLogger creates a new logger that logs to stderr.
// If created WithLoader, the loader must be stopped in a defer.
func NewStdErrLogger(opts StdErrLoggerOpts) LoggerWithLoader {
	var loader Loader
	if opts.WithLoader {
		loader = NewLoader()
	} else {
		loader = &NoopLoader{}
	}
	loader.Start()
	return &StdErrLogger{
		loader: loader,
	}
}

func (l *StdErrLogger) Log(msg string, args ...interface{}) {
	wasActive := l.StopLoader()
	if wasActive {
		defer l.StartLoader()
	}
	Log(msg, args...)
}

func (l *StdErrLogger) Debug(msg string, args ...interface{}) {
	wasActive := l.StopLoader()
	if wasActive {
		defer l.StartLoader()
	}
	Debug(msg, args...)
}

func (l *StdErrLogger) Warning(msg string, args ...interface{}) {
	wasActive := l.StopLoader()
	if wasActive {
		defer l.StartLoader()
	}
	Warning(msg, args...)
}

func (l *StdErrLogger) Error(msg string, args ...interface{}) {
	wasActive := l.StopLoader()
	if wasActive {
		defer l.StartLoader()
	}
	Error(msg, args...)
}

func (l *StdErrLogger) Step(msg string, args ...interface{}) {
	wasActive := l.StopLoader()
	if wasActive {
		defer l.StartLoader()
	}
	Step(msg, args...)
}

func (l *StdErrLogger) Suggest(title, command string, args ...interface{}) {
	wasActive := l.StopLoader()
	if wasActive {
		defer l.StartLoader()
	}
	Suggest(title, command, args...)
}

func (l *StdErrLogger) StopLoader() bool {
	if l.loader != nil {
		isActive := l.loader.IsActive()
		l.loader.Stop()
		return isActive
	}
	return false
}

func (l *StdErrLogger) StartLoader() {
	if l.loader != nil {
		l.loader.Start()
	}
}

func (l *StdErrLogger) SuggestSteps(title string, steps ...string) {
	SuggestSteps(title, steps...)
}

// Log writes a log message to stderr, followed by a newline. Printf-style
// formatting is applied to msg using args.
func Log(msg string, args ...interface{}) {
	if len(args) == 0 {
		// Use Fprint if no args - avoids treating msg like a format string
		fmt.Fprint(os.Stderr, msg+"\n")
	} else {
		fmt.Fprintf(os.Stderr, msg+"\n", args...)
	}
}

// Step prints a step that was performed.
func Step(msg string, args ...interface{}) {
	Log("- "+msg, args...)
}

// Suggest suggests a command with title and args.
func Suggest(title, command string, args ...interface{}) {
	Log("\n"+Gray(title)+"\n  "+command, args...)
}

func SuggestSteps(title string, steps ...string) {
	if len(steps) > 0 {
		Log("\n" + Gray(title) + "\n- " + strings.Join(steps, "\n- "))
	}
}

// Error logs an error message.
func Error(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, Red("Error: ")+msg+"\n", args...)
}

// Warning logs a warning message.
func Warning(msg string, args ...interface{}) {
	fmt.Fprint(os.Stderr, Yellow("[warning] "+msg+"\n", args...))
}

// Debug writes a log message to stderr, followed by a newline, if the CLI
// is executing in debug mode. Printf-style formatting is applied to msg
// using args.
func Debug(msg string, args ...interface{}) {
	if !EnableDebug {
		return
	}

	msgf := msg
	if len(args) > 0 {
		msgf = fmt.Sprintf(msg, args...)
	}

	debugPrefix := "[" + Blue("debug") + "] "
	msgf = debugPrefix + strings.Join(strings.Split(msgf, "\n"), "\n"+debugPrefix)

	fmt.Fprint(os.Stderr, msgf+"\n")
}

type Loader interface {
	Start()
	Stop()
	IsActive() bool
}

// SpinnerLoader adds a spinner / progress indicator to stderr.
type SpinnerLoader struct {
	sync.Mutex
	spin *spinner.Spinner
}

// NoopLoader doesn't do anything.
type NoopLoader struct {
}

func NewLoader() Loader {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return &NoopLoader{}
	}
	return &SpinnerLoader{
		spin: spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(os.Stderr)),
	}
}

// Start starts a new loader. The loader should be stopped
// before writing additional output to stderr.
func (sp *SpinnerLoader) Start() {
	sp.Lock()
	defer sp.Unlock()
	sp.spin.Start()
}

// Stop stops the loader and removes it from stderr.
func (sp *SpinnerLoader) Stop() {
	sp.Lock()
	defer sp.Unlock()
	sp.spin.Stop()
}

// Returns whether the spinner is active.
func (sp *SpinnerLoader) IsActive() bool {
	return sp.spin.Active()
}

func (sp *NoopLoader) Start() {
}
func (sp *NoopLoader) Stop() {
}
func (sp *NoopLoader) IsActive() bool {
	return false
}
