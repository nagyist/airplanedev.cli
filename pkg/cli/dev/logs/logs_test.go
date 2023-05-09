package logs

import (
	"strconv"
	"testing"
	"time"

	"github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestLogging(t *testing.T) {
	require := require.New(t)

	logBroker := NewDevLogBroker()
	watcher := logBroker.NewWatcher()
	watcherLogs := make([]api.LogItem, 0)

	var g errgroup.Group
	g.Go(func() error {
		for {
			select {
			case log, open := <-watcher.Logs():
				if !open {
					return nil
				}
				watcherLogs = append(watcherLogs, log)
			case <-time.After(time.Second * 30):
				return errors.New("Timed out waiting for message on log or done channel")
			}
		}
	})

	for i := 0; i < 10; i++ {
		logBroker.Record(api.LogItem{
			Text: strconv.Itoa(i),
		})
	}
	logBroker.Close()

	err := g.Wait()
	require.NoError(err)

	expectedText := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	require.Equal(len(expectedText), len(watcherLogs))
	for i, log := range watcherLogs {
		require.Equal(expectedText[i], log.Text)
	}
}

func TestMultipleLogWatchers(t *testing.T) {
	require := require.New(t)

	logBroker := NewDevLogBroker()

	type watcherAndLogs struct {
		watcher LogWatcher
		logs    []api.LogItem
	}
	watchers := make([]watcherAndLogs, 3)

	var g errgroup.Group
	for i := range watchers {
		i := i // Re-create i with closure
		watchers[i] = watcherAndLogs{
			watcher: logBroker.NewWatcher(),
			logs:    make([]api.LogItem, 0),
		}

		g.Go(func() error {
			for {
				select {
				case log, open := <-watchers[i].watcher.Logs():
					if !open {
						return nil
					}
					watchers[i].logs = append(watchers[i].logs, log)
				case <-time.After(time.Second * 30):
					return errors.New("Timed out waiting for message on log or done channel")
				}
			}
		})
	}

	for i := 0; i < 10; i++ {
		logBroker.Record(api.LogItem{
			Text: strconv.Itoa(i),
		})
	}
	logBroker.Close()

	err := g.Wait()
	require.NoError(err)

	expectedText := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	for _, w := range watchers {
		require.Equal(len(expectedText), len(w.logs))
		for i, log := range w.logs {
			require.Equal(expectedText[i], log.Text)
		}
	}
}

func TestPastLogs(t *testing.T) {
	require := require.New(t)

	logBroker := NewDevLogBroker()
	// Initialize log broker with logs
	for i := 0; i < 5; i++ {
		logBroker.logs = append(logBroker.logs, api.LogItem{
			Text: strconv.Itoa(i),
		})
	}

	watcher := logBroker.NewWatcher()
	watcherLogs := make([]api.LogItem, 0)

	var g errgroup.Group
	g.Go(func() error {
		for {
			select {
			case log, open := <-watcher.Logs():
				if !open {
					return nil
				}
				watcherLogs = append(watcherLogs, log)
			case <-time.After(time.Second * 30):
				return errors.New("Timed out waiting for message on log or done channel")
			}
		}
	})

	for i := 5; i < 10; i++ {
		logBroker.Record(api.LogItem{
			Text: strconv.Itoa(i),
		})
	}
	logBroker.Close()

	err := g.Wait()
	require.NoError(err)

	// Verify that the watcher received both past logs and new logs.
	expectedText := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	require.Equal(len(expectedText), len(watcherLogs))
	for i, log := range watcherLogs {
		require.Equal(expectedText[i], log.Text)
	}
}

func TestLogBrokerClosed(t *testing.T) {
	require := require.New(t)

	logBroker := NewDevLogBroker()
	for i := 0; i < 10; i++ {
		logBroker.logs = append(logBroker.logs, api.LogItem{
			Text: strconv.Itoa(i),
		})
	}
	logBroker.closed = true

	watcher := logBroker.NewWatcher()
	watcherLogs := make([]api.LogItem, 0)

	var g errgroup.Group
	g.Go(func() error {
		for {
			select {
			case log, open := <-watcher.Logs():
				if !open {
					return nil
				}
				watcherLogs = append(watcherLogs, log)
			case <-time.After(time.Second * 30):
				return errors.New("Timed out waiting for message on log or done channel")
			}
		}
	})

	err := g.Wait()
	require.NoError(err)

	expectedText := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	require.Equal(len(expectedText), len(watcherLogs))
	for i, log := range watcherLogs {
		require.Equal(expectedText[i], log.Text)
	}
}
