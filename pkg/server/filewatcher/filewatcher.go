package filewatcher

import (
	"path/filepath"
	"strings"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/rjeczalik/notify"
)

// FileWatcher will listen for changes to files in a directory
type FileWatcher interface {
	Watch(directory string) error
	Stop()
}

var _ FileWatcher = &AppWatcher{}

// Watches for changes in airplane apps (tasks, views, and dev config file)
type AppWatcher struct {
	changes  chan notify.EventInfo
	isValid  func(path string) bool
	callback func(e notify.EventInfo) error
}

type AppWatcherOpts struct {
	// IsValid returns true if event is valid and callback should be called
	IsValid func(path string) bool
	// Callback is called on all valid filesystem notifications received
	Callback func(e notify.EventInfo) error
}

func NewAppWatcher(opts AppWatcherOpts) FileWatcher {
	if opts.IsValid == nil {
		// set a default filter
		opts.IsValid = IsValidDefinitionFile
	}
	return &AppWatcher{
		changes:  make(chan notify.EventInfo, 20),
		callback: opts.Callback,
		isValid:  opts.IsValid,
	}
}

func IsValidDefinitionFile(path string) bool {
	// ignore directories
	if discover.IgnoredDirectories[filepath.Base(path)] {
		return false
	}
	// ignore files inside ignored directories
	for dir := range discover.IgnoredDirectories {
		fileDir := filepath.Dir(path)
		if strings.Contains(fileDir, dir) {
			return false
		}
	}
	// only allow supported file extensions that can contain task/view definitions
	for _, ext := range discover.DefinitionFileExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func (f *AppWatcher) Watch(wd string) error {
	logger.Log(" ")
	logger.Log(logger.Green("Watching for changes in: %s", wd))
	logger.Log(logger.Green("Changes to tasks, views, and workflows will be applied automatically."))

	if err := notify.Watch(wd+"/...", f.changes, notify.All); err != nil {
		return err
	}
	go func() {
		for event := range f.changes {
			if f.callback != nil && f.isValid != nil {
				if f.isValid(event.Path()) {
					err := f.callback(event)
					if err != nil {
						logger.Log("Error refreshing app in [%s]: %v", event.Path(), err)
					}
				}
			}
		}
	}()
	return nil
}

func (f *AppWatcher) Stop() {
	logger.Log(" ")
	notify.Stop(f.changes)
	close(f.changes)
	logger.Log("Stopped watching for changes.")
}
