package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsInit(t *testing.T) {
	t.Run("init with metrics enabled uses free port", func(t *testing.T) {
		e := NewExporter("test_init")
		e.Options().MetricsEnabled = true
		// Use a random high port to avoid conflicts
		e.Options().metricsPort = "0"
		err := e.Init()
		// Port 0 may not work for ListenAndServe, but Init should not return error
		// because the server is started in a goroutine
		require.NoError(t, err)
	})

	t.Run("init with metrics disabled", func(t *testing.T) {
		e := NewExporter("test_disabled")
		e.Options().MetricsEnabled = false
		err := e.Init()
		assert.NoError(t, err)
	})

	t.Run("startMetricServer with metrics disabled", func(t *testing.T) {
		e := &promMetricsExporter{
			&exporter{
				namespace: "test",
				options:   defaultMetricOptions(),
				logger:    nil,
			},
			nil,
		}
		e.Options().MetricsEnabled = false
		err := e.startMetricServer()
		assert.NoError(t, err)
	})
}
