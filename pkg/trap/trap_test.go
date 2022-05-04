package trap

import (
	"context"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func reset() {
	Timeout = 1 * time.Minute
	ForceExit = true
	exit = func(int) {}
}

func TestTrap(t *testing.T) {
	t.Run("cancels the context on signal", func(t *testing.T) {
		var assert = require.New(t)
		var sigc = make(chan os.Signal, 1)

		signal.Notify(sigc, os.Interrupt)

		reset()
		ctx := newContext(sigc)
		sigc <- os.Interrupt

		<-ctx.Done()

		assert.Equal(ctx.Err(), context.Canceled)
	})

	t.Run("exit with 1 when timeout is reached", func(t *testing.T) {
		reset()

		var assert = require.New(t)
		var sigc = make(chan os.Signal, 1)
		var code = -1

		signal.Notify(sigc, os.Interrupt)

		reset()
		Timeout = 10 * time.Millisecond
		exit = func(c int) { code = c }

		ctx := newContext(sigc)
		sigc <- os.Interrupt

		<-ctx.Done()
		time.Sleep(2 * Timeout)

		assert.Equal(1, code)
	})

	t.Run("exit with 1 when ForceExit is true and a second signal is sent", func(t *testing.T) {
		var assert = require.New(t)
		var sigc = make(chan os.Signal, 1)
		var code = -1

		signal.Notify(sigc, os.Interrupt)

		reset()
		ForceExit = true
		exit = func(c int) { code = c }

		ctx := newContext(sigc)
		sigc <- os.Interrupt
		<-ctx.Done()
		sigc <- os.Interrupt

		time.Sleep(10 * time.Millisecond)
		assert.Equal(1, code)
	})
}
