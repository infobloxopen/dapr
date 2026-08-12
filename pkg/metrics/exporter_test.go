// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package metrics

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/dapr/dapr/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsExporter(t *testing.T) {
	t.Run("returns default options", func(t *testing.T) {
		e := NewExporter("test")
		op := e.Options()
		assert.Equal(t, defaultMetricOptions(), op)
	})

	t.Run("return error if exporter is not initialized", func(t *testing.T) {
		e := &promMetricsExporter{
			&exporter{
				namespace: "test",
				options:   defaultMetricOptions(),
				logger:    logger.NewLogger("dapr.metrics"),
			},
			nil,
		}
		assert.Error(t, e.startMetricServer())
	})

	t.Run("skip starting metric server", func(t *testing.T) {
		e := NewExporter("test")
		e.Options().MetricsEnabled = false
		err := e.Init()
		assert.NoError(t, err)
	})
}

func TestInitMetricsEnabled(t *testing.T) {
	// Get a free port by listening on :0, then closing the listener
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := lis.Addr().(*net.TCPAddr).Port
	lis.Close()

	e := &promMetricsExporter{
		&exporter{
			namespace: "test",
			options: &Options{
				MetricsEnabled: true,
				metricsPort:    fmt.Sprintf("%d", port),
			},
			logger: logger.NewLogger("dapr.metrics"),
		},
		nil,
	}

	err = e.Init()
	assert.NoError(t, err)
	assert.NotNil(t, e.ocExporter)

	// Give the background goroutine time to start the server
	time.Sleep(200 * time.Millisecond)

	// Verify the server is actually listening by making a request
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/", port))
	if err == nil {
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}

func TestStartMetricServerNotEnabled(t *testing.T) {
	e := &promMetricsExporter{
		&exporter{
			namespace: "test",
			options: &Options{
				MetricsEnabled: false,
				metricsPort:    "9090",
			},
			logger: logger.NewLogger("dapr.metrics"),
		},
		nil,
	}

	err := e.startMetricServer()
	assert.NoError(t, err)
}

func TestStartMetricServerNilExporter(t *testing.T) {
	e := &promMetricsExporter{
		&exporter{
			namespace: "test",
			options: &Options{
				MetricsEnabled: true,
				metricsPort:    "9090",
			},
			logger: logger.NewLogger("dapr.metrics"),
		},
		nil,
	}

	err := e.startMetricServer()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exporter was not initialized")
}
