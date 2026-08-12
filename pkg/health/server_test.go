package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/dapr/dapr/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getFreePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func TestNewServer(t *testing.T) {
	t.Run("creates non-nil server", func(t *testing.T) {
		log := logger.NewLogger("test.health")
		s := NewServer(log)
		require.NotNil(t, s)
	})
}

func TestReadyNotReady(t *testing.T) {
	t.Run("ready sets server to ready", func(t *testing.T) {
		log := logger.NewLogger("test.health.ready")
		s := NewServer(log).(*server)
		assert.False(t, s.ready)
		s.Ready()
		assert.True(t, s.ready)
	})

	t.Run("not ready sets server to not ready", func(t *testing.T) {
		log := logger.NewLogger("test.health.notready")
		s := NewServer(log).(*server)
		s.Ready()
		assert.True(t, s.ready)
		s.NotReady()
		assert.False(t, s.ready)
	})
}

func TestHealthzEndpoint(t *testing.T) {
	t.Run("returns 200 when ready", func(t *testing.T) {
		log := logger.NewLogger("test.health.200")
		port, err := getFreePort()
		require.NoError(t, err)

		s := NewServer(log)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- s.Run(ctx, port)
		}()

		// Wait for server to start
		time.Sleep(500 * time.Millisecond)

		s.Ready()
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", port))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		cancel()
	})

	t.Run("returns 503 when not ready", func(t *testing.T) {
		log := logger.NewLogger("test.health.503")
		port, err := getFreePort()
		require.NoError(t, err)

		s := NewServer(log)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			s.Run(ctx, port)
		}()

		// Wait for server to start
		time.Sleep(500 * time.Millisecond)

		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", port))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		cancel()
	})
}
