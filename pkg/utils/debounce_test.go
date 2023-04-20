package utils

import (
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"
)

func TestDebounce(t *testing.T) {
	require := require.New(t)

	called := 0
	clock := clock.NewMock()
	f := DebounceWithOpts(DebounceOpts{
		Delay: time.Second,
		clock: clock,
	}, func() {
		called++
	})

	// The function has not been called yet.
	require.Equal(0, called)

	f()
	f()
	f()

	// The function has still not been called because delay hasn't elapsed.
	require.Equal(called, 0)

	// Advance 1s. This will cause the function to get invoked.
	clock.Add(time.Second)
	require.Equal(called, 1)
}

func TestDebounceLeading(t *testing.T) {
	require := require.New(t)

	called := 0
	clock := clock.NewMock()
	f := DebounceWithOpts(DebounceOpts{
		Delay:   time.Second,
		Leading: true,
		clock:   clock,
	}, func() {
		called++
	})

	// The function should be called immediately.
	f()
	f()
	f()
	require.Equal(1, called)

	// The function should not be called again if <delay.
	clock.Add(time.Second / 2)
	require.Equal(1, called)

	// Once delay time has passed, it should be called like normal.
	clock.Add(time.Second / 2)
	require.Equal(2, called)

	// If called only once, it should only be called on the leading edge.
	clock.Add(time.Second)
	f()
	require.Equal(3, called)
	clock.Add(time.Second)
	require.Equal(3, called)

	// If called more than once, it should be called on both the leading and trailing edges.
	clock.Add(time.Second)
	f()
	require.Equal(4, called)
	f()
	f()
	require.Equal(4, called)
	clock.Add(time.Second)
	require.Equal(5, called)
}

func TestDebounceMaxWait(t *testing.T) {
	require := require.New(t)

	called := 0
	clock := clock.NewMock()
	f := DebounceWithOpts(DebounceOpts{
		Delay:   2 * time.Second,
		MaxWait: 4 * time.Second,
		clock:   clock,
	}, func() {
		called++
	})

	// Keep invoking the function before the delay can elapse. The underlying function will not
	// get called until we hit maxWait.
	f()
	require.Equal(0, called)
	clock.Add(time.Second) // 1s
	f()
	require.Equal(0, called)
	clock.Add(time.Second) // 2s
	f()
	require.Equal(0, called)
	clock.Add(time.Second) // 3s
	f()
	require.Equal(0, called)
	clock.Add(time.Second) // 4s
	// The function should get called automatically due to maxWait.
	require.Equal(1, called)

	// The function should not get called again (e.g. for the trailing edge).
	clock.Add(10 * time.Second)
	require.Equal(1, called)

	// But if we do another function call, it should get called again.
	f()
	require.Equal(1, called)
	clock.Add(2 * time.Second)
	require.Equal(2, called)
}

func TestDebounceMaxWaitLeading(t *testing.T) {
	require := require.New(t)

	called := 0
	clock := clock.NewMock()
	f := DebounceWithOpts(DebounceOpts{
		Delay:   2 * time.Second,
		Leading: true,
		MaxWait: 4 * time.Second,
		clock:   clock,
	}, func() {
		called++
	})

	// Keep invoking the function before the delay can elapse. The underlying function will get
	// called immediately because leading is true, but otherwise won't get called until we hit maxWait.
	f()
	require.Equal(1, called)
	clock.Add(time.Second) // 1s
	f()
	require.Equal(1, called)
	clock.Add(time.Second) // 2s
	f()
	require.Equal(1, called)
	clock.Add(time.Second) // 3s
	f()
	require.Equal(1, called)
	clock.Add(time.Second) // 4s
	// The function should get called automatically due to maxWait.
	require.Equal(2, called)

	// If we do another function call, it's been <delay since the function was invoked (due to maxWait),
	// so it should not get called immediately.
	f()
	require.Equal(2, called)
	clock.Add(10 * time.Second)
	// But it should eventually fire:
	require.Equal(3, called)
}

func TestDebounceMaxWaitEqualDelay(t *testing.T) {
	require := require.New(t)

	called := 0
	clock := clock.NewMock()
	f := DebounceWithOpts(DebounceOpts{
		Delay:   time.Millisecond,
		MaxWait: time.Millisecond,
		clock:   clock,
	}, func() {
		called++
	})

	// Should only be called once, even though delay and maxWait elapse at the same time.
	f()
	require.Equal(0, called)
	clock.Add(time.Millisecond)
	require.Equal(1, called)
}
