package concurrency

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLimiter(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		expectedLimit int
	}{
		{"positive limit", 5, 5},
		{"zero limit defaults to 100", 0, DefaultLimit},
		{"negative limit defaults to 100", -1, DefaultLimit},
		{"limit of 1", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLimiter(tt.limit)
			require.NotNil(t, l)
			assert.Equal(t, tt.expectedLimit, l.limit)
		})
	}
}

func TestExecute(t *testing.T) {
	t.Run("executes function and returns ticket", func(t *testing.T) {
		l := NewLimiter(5)
		var called int32
		l.Execute(func(param interface{}) {
			atomic.AddInt32(&called, 1)
		}, nil)
		l.Wait()
		assert.Equal(t, int32(1), atomic.LoadInt32(&called))
	})

	t.Run("respects concurrency limit", func(t *testing.T) {
		limit := 3
		l := NewLimiter(limit)
		var maxConcurrent int32
		var current int32
		done := make(chan struct{})

		for i := 0; i < 10; i++ {
			l.Execute(func(param interface{}) {
				cur := atomic.AddInt32(&current, 1)
				for {
					old := atomic.LoadInt32(&maxConcurrent)
					if cur <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, cur) {
						break
					}
				}
				// small busy wait to let concurrency build up
				for j := 0; j < 1000; j++ {
				}
				atomic.AddInt32(&current, -1)
			}, nil)
		}
		l.Wait()
		close(done)
		assert.LessOrEqual(t, int(atomic.LoadInt32(&maxConcurrent)), limit)
	})

	t.Run("passes parameter to job", func(t *testing.T) {
		l := NewLimiter(1)
		var received interface{}
		l.Execute(func(param interface{}) {
			received = param
		}, "hello")
		l.Wait()
		assert.Equal(t, "hello", received)
	})
}

func TestWait(t *testing.T) {
	t.Run("blocks until all jobs complete", func(t *testing.T) {
		l := NewLimiter(5)
		var count int32
		for i := 0; i < 10; i++ {
			l.Execute(func(param interface{}) {
				atomic.AddInt32(&count, 1)
			}, nil)
		}
		l.Wait()
		assert.Equal(t, int32(10), atomic.LoadInt32(&count))
	})
}
