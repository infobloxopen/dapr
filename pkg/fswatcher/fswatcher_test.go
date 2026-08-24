package fswatcher

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatch(t *testing.T) {
	t.Run("detects file creation", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "fswatcher-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		eventCh := make(chan struct{}, 1)
		errCh := make(chan error, 1)

		go func() {
			errCh <- Watch(ctx, dir, eventCh)
		}()

		// Give watcher time to start
		time.Sleep(200 * time.Millisecond)

		// Create a file to trigger the watcher
		err = ioutil.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)
		require.NoError(t, err)

		select {
		case <-eventCh:
			// Success - event detected
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for file event")
		}

		cancel()
		// Wait for Watch to return
		watchErr := <-errCh
		assert.NoError(t, watchErr)
	})

	t.Run("context cancellation stops watcher", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "fswatcher-cancel")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		ctx, cancel := context.WithCancel(context.Background())
		eventCh := make(chan struct{}, 1)
		errCh := make(chan error, 1)

		go func() {
			errCh <- Watch(ctx, dir, eventCh)
		}()

		time.Sleep(200 * time.Millisecond)
		cancel()

		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("Watch did not return after context cancellation")
		}
	})

	t.Run("invalid directory returns error", func(t *testing.T) {
		ctx := context.Background()
		eventCh := make(chan struct{}, 1)

		err := Watch(ctx, "/nonexistent/path/that/does/not/exist", eventCh)
		assert.Error(t, err)
	})
}
