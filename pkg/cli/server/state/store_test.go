//go:build !race

package state

import (
	"errors"
	"sync"
	"testing"

	"github.com/airplanedev/cli/pkg/cli/dev"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	c := NewStore[string, int](nil)

	v, ok := c.Get("doesn't exist")
	require.False(t, ok)
	require.Equal(t, v, 0)
	c.Add("val1", 123)
	v, ok = c.Get("val1")
	require.True(t, ok)
	require.Equal(t, v, 123)

	c.Delete("doesn't exist shouldn't error'")
	c.Delete("val1")
	_, ok = c.Get("val1")
	require.False(t, ok)
}

func TestStoreUpdate(t *testing.T) {
	runStore := NewStore[string, dev.LocalRun](nil)
	runStore.Add("run_1", dev.LocalRun{RunID: "run_1", ParentID: "original_parent_id"})
	v1, ok := runStore.Get("run_1")
	require.True(t, ok)
	require.Equal(t, v1.RunID, "run_1")
	require.Equal(t, v1.ParentID, "original_parent_id")

	_, err := runStore.Update("run_1", func(run *dev.LocalRun) error {
		run.ParentID = "new_parent_id"
		return nil
	})
	require.NoError(t, err)
	v1, ok = runStore.Get("run_1")
	require.True(t, ok)
	require.Equal(t, v1.RunID, "run_1")
	require.Equal(t, v1.ParentID, "new_parent_id")

	_, err = runStore.Update("run_1", func(run *dev.LocalRun) error {
		return errors.New("test throwing an error")
	})
	require.Error(t, err)
}

func TestStoreReplace(t *testing.T) {
	c := NewStore[string, int](nil)
	newItems := map[string]int{"item1": 1}
	require.Equal(t, 0, c.Len())
	c.ReplaceItems(newItems)
	require.Equal(t, 1, c.Len())
	v, ok := c.Get("item1")
	require.True(t, ok)
	require.Equal(t, v, 1)
}

func TestConcurrentStore(t *testing.T) {
	s := NewStore[string, int](nil)
	wg := sync.WaitGroup{}
	wg.Add(10 * 3)
	for i := 1; i <= 10; i++ {
		go func() {
			defer wg.Done()
			s.Add("val1", 1)
			s.Get("val1")
		}()
		go func() {
			defer wg.Done()
			s.Get("val1")
			s.Items()
			s.Add("val1", 2)
		}()
		go func() {
			defer wg.Done()
			s.Len()
			s.Get("val2")
			s.Delete("val1")
			s.Add("val1", 3)
		}()
	}
	// checks that this does not panic
	wg.Wait()
}
