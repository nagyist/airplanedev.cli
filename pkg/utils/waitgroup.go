package utils

import (
	"sync"
	"time"
)

// WaitTimeout waits until the given WaitGroup has completed running, or until it times out.
// Returns true if the wait has timed out, false otherwise.
// see https://go.dev/play/p/1TdOU_PquM
func WaitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

// WaitUntilTimeout runs check() every pollInterval until check() returns true or the wait times
// out. Returns true if the check timed out, false otherwise.
func WaitUntilTimeout(check func() bool, pollInterval, timeout time.Duration) bool {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0 * time.Second; i < timeout+pollInterval*2; i += pollInterval {
			if check() {
				return
			}
			time.Sleep(pollInterval)
		}
	}()
	return WaitTimeout(&wg, timeout)
}
