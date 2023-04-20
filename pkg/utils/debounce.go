package utils

import (
	"sync"
	"time"

	"github.com/benbjohnson/clock"
)

type DebounceOpts struct {
	Delay   time.Duration
	Leading bool
	MaxWait time.Duration

	clock clock.Clock
}

// Debounce creates a debounced function with reasonable defaults. See DebounceWithOpts.
func Debounce(delay time.Duration, f func()) func() {
	return DebounceWithOpts(DebounceOpts{
		Delay:   delay,
		Leading: true,
		MaxWait: delay * 5,
	}, f)
}

// DebounceWithOpts creates a debounced function that delays invoking `f` until after
// `opts.Delay` seconds have elapsed since the last time the debounced function was
// invoked.
//
// If `opts.Leading`, the function will be called at the beginning of the delay timeout.
//
// If `opts.MaxWait` is <=0 and function is continuously called with < opts.Delay seconds
// between invocations, then the function will never be called. Otherwise, the debounced
// function will ensure that the function is called at least once every `opts.MaxWait`
// seconds.
//
// To learn more about debounce, see: https://llu.is/throttle-and-debounce-visualized
//
// This implementation is modeled after lodash's debounce method.
func DebounceWithOpts(opts DebounceOpts, f func()) func() {
	if opts.clock == nil {
		opts.clock = clock.New()
	}
	if opts.Delay < 0 {
		opts.Delay = 0
	}

	var mu sync.Mutex
	var timer *clock.Timer
	var maxTimer *clock.Timer
	var lastCalled time.Time
	var functionCalls int
	var lastFunctionCall int

	invoke := func() {
		if timer != nil {
			timer.Stop()
			timer = nil
		}
		if maxTimer != nil {
			maxTimer.Stop()
			maxTimer = nil
		}

		// Do not invoke unless there has been a debounce call since the last one we invoked:
		if lastFunctionCall >= functionCalls {
			return
		}

		// If we just invoked the function (e.g. a call happened just after maxWait fired), don't
		// invoke again.
		now := opts.clock.Now()
		if lastCalled.Add(opts.Delay).After(now) {
			return
		}
		lastFunctionCall = functionCalls
		lastCalled = now

		f()
	}

	return func() {
		mu.Lock()
		defer mu.Unlock()

		functionCalls++

		if opts.Leading && timer == nil {
			invoke()
		}

		if timer != nil {
			timer.Stop()
		}
		timer = opts.clock.AfterFunc(opts.Delay, func() {
			mu.Lock()
			defer mu.Unlock()
			invoke()
		})

		if opts.MaxWait > 0 && maxTimer == nil {
			maxTimer = opts.clock.AfterFunc(opts.MaxWait, func() {
				mu.Lock()
				defer mu.Unlock()
				invoke()
			})
		}
	}
}
