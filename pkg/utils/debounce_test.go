package utils

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"
)

func TestDebounce(t *testing.T) {
	require := require.New(t)

	called := atomic.Int32{}
	clock := clock.NewMock()
	f := DebounceWithOpts(DebounceOpts{
		Delay: time.Second,
		clock: clock,
	}, func() {
		called.Add(1)
	})

	// The function has not been called yet.
	require.EqualValues(0, called.Load())

	f()
	f()
	f()

	// The function has still not been called because delay hasn't elapsed.
	require.EqualValues(0, called.Load())

	// Advance 1s. This will cause the function to get invoked.
	clock.Add(time.Second)
	require.EqualValues(1, called.Load())
}

func TestDebounceLeading(t *testing.T) {
	require := require.New(t)

	called := atomic.Int32{}
	clock := clock.NewMock()
	f := DebounceWithOpts(DebounceOpts{
		Delay:   time.Second,
		Leading: true,
		clock:   clock,
	}, func() {
		called.Add(1)
	})

	// The function should be called immediately.
	f()
	f()
	f()
	require.EqualValues(1, called.Load())

	// The function should not be called again if <delay.
	clock.Add(time.Second / 2)
	require.EqualValues(1, called.Load())

	// Once delay time has passed, it should be called like normal.
	clock.Add(time.Second / 2)
	require.EqualValues(2, called.Load())

	// If called only once, it should only be called on the leading edge.
	clock.Add(time.Second)
	f()
	require.EqualValues(3, called.Load())
	clock.Add(time.Second)
	require.EqualValues(3, called.Load())

	// If called more than once, it should be called on both the leading and trailing edges.
	clock.Add(time.Second)
	f()
	require.EqualValues(4, called.Load())
	f()
	f()
	require.EqualValues(4, called.Load())
	clock.Add(time.Second)
	require.EqualValues(5, called.Load())
}

func TestDebounceMaxWait(t *testing.T) {
	require := require.New(t)

	called := atomic.Int32{}
	clock := clock.NewMock()
	f := DebounceWithOpts(DebounceOpts{
		Delay:   2 * time.Second,
		MaxWait: 4 * time.Second,
		clock:   clock,
	}, func() {
		called.Add(1)
	})

	// Keep invoking the function before the delay can elapse. The underlying function will not
	// get called until we hit maxWait.
	f()
	require.EqualValues(0, called.Load())
	clock.Add(time.Second) // 1s
	f()
	require.EqualValues(0, called.Load())
	clock.Add(time.Second) // 2s
	f()
	require.EqualValues(0, called.Load())
	clock.Add(time.Second) // 3s
	f()
	require.EqualValues(0, called.Load())
	clock.Add(time.Second) // 4s
	// The function should get called automatically due to maxWait.
	require.EqualValues(1, called.Load())

	// The function should not get called again (e.g. for the trailing edge).
	clock.Add(10 * time.Second)
	require.EqualValues(1, called.Load())

	// But if we do another function call, it should get called again.
	f()
	require.EqualValues(1, called.Load())
	clock.Add(2 * time.Second)
	require.EqualValues(2, called.Load())
}

func TestDebounceMaxWaitLeading(t *testing.T) {
	require := require.New(t)

	called := atomic.Int32{}
	clock := clock.NewMock()
	f := DebounceWithOpts(DebounceOpts{
		Delay:   2 * time.Second,
		Leading: true,
		MaxWait: 4 * time.Second,
		clock:   clock,
	}, func() {
		called.Add(1)
	})

	// Keep invoking the function before the delay can elapse. The underlying function will get
	// called immediately because leading is true, but otherwise won't get called until we hit maxWait.
	f()
	require.EqualValues(1, called.Load())
	clock.Add(time.Second) // 1s
	f()
	require.EqualValues(1, called.Load())
	clock.Add(time.Second) // 2s
	f()
	require.EqualValues(1, called.Load())
	clock.Add(time.Second) // 3s
	f()
	require.EqualValues(1, called.Load())
	clock.Add(time.Second) // 4s
	// The function should get called automatically due to maxWait.
	require.EqualValues(2, called.Load())

	// If we do another function call, it's been <delay since the function was invoked (due to maxWait),
	// so it should not get called immediately.
	f()
	require.EqualValues(2, called.Load())
	clock.Add(10 * time.Second)
	// But it should eventually fire:
	require.EqualValues(3, called.Load())
}

func TestDebounceMaxWaitEqualDelay(t *testing.T) {
	require := require.New(t)

	called := atomic.Int32{}
	clock := clock.NewMock()
	f := DebounceWithOpts(DebounceOpts{
		Delay:   time.Millisecond,
		MaxWait: time.Millisecond,
		clock:   clock,
	}, func() {
		called.Add(1)
	})

	// Should only be called once, even though delay and maxWait elapse at the same time.
	f()
	require.EqualValues(0, called.Load())
	clock.Add(time.Millisecond)
	require.EqualValues(1, called.Load())
}
