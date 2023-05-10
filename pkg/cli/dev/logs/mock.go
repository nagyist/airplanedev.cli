package logs

import api "github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"

// MockLogWatcher is a mock log watcher.
type MockLogWatcher struct{}

func (w MockLogWatcher) Logs() chan api.LogItem {
	return nil
}

func (w MockLogWatcher) Close() {}

// MockLogBroker is a mock log broker.
type MockLogBroker struct{}

func (l *MockLogBroker) Record(_ api.LogItem) {}

func (l *MockLogBroker) Close() {}

func (l *MockLogBroker) NewWatcher() LogWatcher {
	return MockLogWatcher{}
}
