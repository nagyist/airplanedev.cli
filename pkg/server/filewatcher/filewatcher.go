package filewatcher

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/lib/pkg/deploy/discover"
	"github.com/radovskyb/watcher"
)

// FileWatcher will listen for changes to files in a directory
type FileWatcher interface {
	Watch(directory string) error
	Stop()
}

var _ FileWatcher = &AppWatcher{}

// Operations describe what kind of event occured
type Operation int64

// Operations
const (
	NoOp Operation = iota
	Create
	Write
	Remove
	Move
)

// Event describes the change that occured.
type Event struct {
	Op   Operation
	Path string
}

func toEvent(e watcher.Event) Event {
	event := Event{Path: e.Path}
	switch e.Op {
	case watcher.Create:
		event.Op = Create
	case watcher.Write:
		event.Op = Write
	case watcher.Remove:
		event.Op = Remove
	case watcher.Move, watcher.Rename:
		event.Op = Move
	default:
		event.Op = NoOp
	}
	return event
}

// Watches for changes in airplane apps (tasks, views, and dev config file)
type AppWatcher struct {
	watcher      *watcher.Watcher
	pollInterval time.Duration
	callback     func(e Event) error
	isValid      func(path string) bool
}

type AppWatcherOpts struct {
	// IsValid returns true if event is valid and callback should be called
	IsValid func(path string) bool
	// Callback is called on all valid filesystem notifications received
	Callback func(e Event) error
	// PollInterval specifies how often to poll for changes
	PollInterval time.Duration
}

func NewAppWatcher(opts AppWatcherOpts) FileWatcher {
	w := watcher.New()
	w.SetMaxEvents(20)
	if opts.IsValid == nil {
		// set a default filter
		opts.IsValid = IsValidDefinitionFile
	}

	return &AppWatcher{
		watcher:      w,
		callback:     opts.Callback,
		pollInterval: opts.PollInterval,
		isValid:      opts.IsValid,
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
	logger.Log(logger.Green("Changes to tasks, workflows, and views will be applied automatically."))

	// Ignore hidden files and known directories
	f.watcher.IgnoreHiddenFiles(true)
	directoriesToIgnore := make([]string, 0, len(discover.IgnoredDirectories))
	for dir := range discover.IgnoredDirectories {
		directoriesToIgnore = append(directoriesToIgnore, filepath.Join(wd, dir))
	}
	if err := f.watcher.Ignore(directoriesToIgnore...); err != nil {
		return err
	}
	// Watch working directory recursively for changes.
	if err := f.watcher.AddRecursive(wd); err != nil {
		return err
	}
	// Listen for changes
	go func() {
		for {
			select {
			case e := <-f.watcher.Event:
				if !e.IsDir() && f.isValid(e.Path) {
					event := toEvent(e)
					if err := f.callback(event); err != nil {
						logger.Log("Error refreshing app in [%s]: %v", event.Path, err)
					}
				}
			case err := <-f.watcher.Error:
				logger.Error("Watching for changes: ", err)
			case <-f.watcher.Closed:
				logger.Log(" ")
				logger.Log("Stopped watching for changes.")
				return
			}
		}
	}()

	go func() {
		if err := f.watcher.Start(f.pollInterval); err != nil {
			logger.Error("Starting filewatcher: %s", err.Error())
		}
	}()

	return nil
}

func (f *AppWatcher) Stop() {
	f.watcher.Close()
}
