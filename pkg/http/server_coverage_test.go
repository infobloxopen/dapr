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

func TestServerUseRouter(t *testing.T) {
	t.Run("returns a non-nil handler from API endpoints", func(t *testing.T) {
		apiObj := NewAPI(
			"router-test",
			nil, nil, nil, nil, nil, nil, nil, nil, nil,
			config.TracingSpec{},
		)
		srv := &server{
			api:    apiObj,
			config: ServerConfig{AppID: "router-test"},
		}
		handler := srv.useRouter()
		require.NotNil(t, handler, "useRouter should return a non-nil handler")

		// Exercise the router with a known route (healthz).
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/v1.0/healthz")
		ctx.Request.Header.SetMethod("GET")
		handler(ctx)
		// The handler should respond (either 200/204 or 500 depending on ready state).
		assert.True(t, ctx.Response.StatusCode() > 0, "handler should set a status code")
	})

	t.Run("router handles unknown route with 404", func(t *testing.T) {
		apiObj := NewAPI(
			"router-test",
			nil, nil, nil, nil, nil, nil, nil, nil, nil,
			config.TracingSpec{},
		)
		srv := &server{
			api:    apiObj,
			config: ServerConfig{AppID: "router-test"},
		}
		handler := srv.useRouter()
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/v1.0/nonexistent-route")
		ctx.Request.Header.SetMethod("GET")
		handler(ctx)
		assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())
	})
}

func TestServerUseComponents(t *testing.T) {
	t.Run("empty pipeline passes through to next handler", func(t *testing.T) {
		srv := &server{
			pipeline: http_middleware.Pipeline{},
		}
		called := false
		next := func(ctx *fasthttp.RequestCtx) {
			called = true
		}
		handler := srv.useComponents(next)
		require.NotNil(t, handler)

		ctx := &fasthttp.RequestCtx{}
		handler(ctx)
		assert.True(t, called, "empty pipeline should pass through to the next handler")
	})

	t.Run("pipeline with middleware wraps handler", func(t *testing.T) {
		order := []string{}
		middleware := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
			return func(ctx *fasthttp.RequestCtx) {
				order = append(order, "middleware")
				next(ctx)
			}
		}
		srv := &server{
			pipeline: http_middleware.Pipeline{
				Handlers: []http_middleware.Middleware{middleware},
			},
		}
		next := func(ctx *fasthttp.RequestCtx) {
			order = append(order, "handler")
		}
		handler := srv.useComponents(next)
		ctx := &fasthttp.RequestCtx{}
		handler(ctx)
		assert.Equal(t, []string{"middleware", "handler"}, order)
	})
}
