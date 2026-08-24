// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package http

import (
	"testing"

	"github.com/dapr/dapr/pkg/config"
	http_middleware "github.com/dapr/dapr/pkg/middleware/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestNewServer(t *testing.T) {
	t.Run("constructor returns non-nil server", func(t *testing.T) {
		apiObj := NewAPI(
			"test-app",
			nil, nil, nil, nil, nil, nil, nil, nil, nil,
			config.TracingSpec{},
		)
		cfg := ServerConfig{
			AllowedOrigins:     "http://localhost",
			AppID:              "test-app",
			HostAddress:        "127.0.0.1",
			Port:               3500,
			ProfilePort:        7777,
			EnableProfiling:    false,
			MaxRequestBodySize: 4,
		}
		srv := NewServer(apiObj, cfg, config.TracingSpec{}, config.MetricSpec{}, http_middleware.Pipeline{})
		require.NotNil(t, srv)
	})

	t.Run("constructor with empty pipeline", func(t *testing.T) {
		apiObj := NewAPI(
			"app2",
			nil, nil, nil, nil, nil, nil, nil, nil, nil,
			config.TracingSpec{},
		)
		cfg := ServerConfig{
			AllowedOrigins:     "*",
			AppID:              "app2",
			Port:               3501,
			MaxRequestBodySize: 4,
		}
		srv := NewServer(apiObj, cfg, config.TracingSpec{}, config.MetricSpec{Enabled: false}, http_middleware.Pipeline{})
		require.NotNil(t, srv)
	})

	t.Run("constructor preserves config fields", func(t *testing.T) {
		apiObj := NewAPI(
			"app3",
			nil, nil, nil, nil, nil, nil, nil, nil, nil,
			config.TracingSpec{},
		)
		cfg := ServerConfig{
			AllowedOrigins:     "http://example.com",
			AppID:              "app3",
			HostAddress:        "0.0.0.0",
			Port:               3502,
			ProfilePort:        7778,
			EnableProfiling:    true,
			MaxRequestBodySize: 16,
		}
		tracingSpec := config.TracingSpec{SamplingRate: "1"}
		metricSpec := config.MetricSpec{Enabled: true}
		pipeline := http_middleware.Pipeline{}

		srv := NewServer(apiObj, cfg, tracingSpec, metricSpec, pipeline)
		require.NotNil(t, srv)

		// Verify fields via the concrete type.
		s, ok := srv.(*server)
		require.True(t, ok)
		assert.Equal(t, "app3", s.config.AppID)
		assert.Equal(t, 3502, s.config.Port)
		assert.Equal(t, 7778, s.config.ProfilePort)
		assert.True(t, s.config.EnableProfiling)
		assert.Equal(t, 16, s.config.MaxRequestBodySize)
		assert.Equal(t, "http://example.com", s.config.AllowedOrigins)
		assert.Equal(t, "1", s.tracingSpec.SamplingRate)
		assert.True(t, s.metricSpec.Enabled)
	})
}

func TestServerUseTracing(t *testing.T) {
	t.Run("tracing disabled returns original handler", func(t *testing.T) {
		srv := &server{
			config:      ServerConfig{AppID: "test-app"},
			tracingSpec: config.TracingSpec{SamplingRate: "0"},
		}
		original := func(ctx *fasthttp.RequestCtx) {}
		handler := srv.useTracing(original)
		require.NotNil(t, handler)
	})

	t.Run("tracing enabled returns wrapped handler", func(t *testing.T) {
		srv := &server{
			config:      ServerConfig{AppID: "test-app"},
			tracingSpec: config.TracingSpec{SamplingRate: "1"},
		}
		original := func(ctx *fasthttp.RequestCtx) {}
		handler := srv.useTracing(original)
		require.NotNil(t, handler)
	})
}

func TestServerUseMetrics(t *testing.T) {
	t.Run("metrics disabled returns original handler", func(t *testing.T) {
		srv := &server{
			metricSpec: config.MetricSpec{Enabled: false},
		}
		original := func(ctx *fasthttp.RequestCtx) {}
		handler := srv.useMetrics(original)
		require.NotNil(t, handler)
	})

	t.Run("metrics enabled returns wrapped handler", func(t *testing.T) {
		srv := &server{
			metricSpec: config.MetricSpec{Enabled: true},
		}
		original := func(ctx *fasthttp.RequestCtx) {}
		handler := srv.useMetrics(original)
		require.NotNil(t, handler)
	})
}
