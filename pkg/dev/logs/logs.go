package logs

import (
	"sync"

	"github.com/airplanedev/cli/pkg/api/cliapi"
)

// LogBroker represents keeps track of a set of log watchers and streams logs to them. There should only be one log
// broker per run.
type LogBroker interface {
	Record(log api.LogItem)
	Close()
	NewWatcher() LogWatcher
}

// DevLogBroker implements the LogBroker interface for local dev.
type DevLogBroker struct {
	// watchers is a set of all log watchers.
	watchers map[DevLogWatcher]struct{}
	// logs stores the logs from the run so far.
	logs []api.LogItem
	// Flag indicating whether logs have finished streaming or not.
	closed bool
	// Used for synchronizing the methods of the log broker.
	mu sync.Mutex
}

// NewDevLogBroker initializes a new DevLogBroker.
func NewDevLogBroker() *DevLogBroker {
	return &DevLogBroker{
		watchers: make(map[DevLogWatcher]struct{}),
		logs:     make([]api.LogItem, 0),
	}
}

// Record sends a log to all watchers, replaying past logs if necessary. It also appends the log to the log broker.
func (l *DevLogBroker) Record(log api.LogItem) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for watcher := range l.watchers {
		// Send past logs if we haven't already.
		watcher.replay.Do(func() {
			for _, pastLog := range l.logs {
				watcher.logs <- pastLog
			}
		})
		// Send the current log.
		watcher.logs <- log
	}
	// Store the log for future retrieval.
	l.logs = append(l.logs, log)
}

// Close unregisters all log watchers and closes the done channel for all the watchers.
func (l *DevLogBroker) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for watcher := range l.watchers {
		// For runs that have already completed, the log broker will never call Record(...), but we still want to replay
		// past logs.
		watcher.replay.Do(func() {
			for _, pastLog := range l.logs {
				watcher.logs <- pastLog
			}
		})
		close(watcher.Logs())
		delete(l.watchers, watcher) // Unregister the watcher from the broker.
	}
	l.closed = true
}

// NewWatcher instantiates a new log watcher.
func (l *DevLogBroker) NewWatcher() LogWatcher {
	l.mu.Lock()
	defer l.mu.Unlock()

	watcher := DevLogWatcher{
		logs:      make(chan api.LogItem, 100),
		replay:    new(sync.Once),
		logBroker: l,
	}
	l.watchers[watcher] = struct{}{}

	// If logs are done being streamed, immediately execute the log replay + watcher un-registration logic in Close().
	if l.closed {
		go l.Close()
	}

	return watcher
}

// unregisterWatcher unregisters the watcher from the log broker.
func (l *DevLogBroker) unregisterWatcher(watcher DevLogWatcher) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.watchers, watcher)
}

// LogWatcher is a receiver for logs. There can be many log watchers for a given run.
type LogWatcher interface {
	Logs() chan api.LogItem
	Close()
}

// DevLogWatcher implements the LogWatcher interface for local dev.
type DevLogWatcher struct {
	logs chan api.LogItem
	// replay is used to ensure we replay past logs only once.
	replay    *sync.Once
	logBroker *DevLogBroker
}

func (w DevLogWatcher) Logs() chan api.LogItem {
	return w.logs
}

func (w DevLogWatcher) Close() {
	w.logBroker.unregisterWatcher(w)
}
