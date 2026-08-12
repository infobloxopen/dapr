package http

import (
	"testing"

	"github.com/dapr/dapr/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestBuildHTTPPipeline(t *testing.T) {
	t.Run("builds empty pipeline", func(t *testing.T) {
		spec := config.PipelineSpec{}
		pipeline, err := BuildHTTPPipeline(spec)
		require.NoError(t, err)
		assert.Empty(t, pipeline.Handlers)
	})
}

func TestPipelineApply(t *testing.T) {
	t.Run("applies empty pipeline returns same handler", func(t *testing.T) {
		var called bool
		handler := func(ctx *fasthttp.RequestCtx) {
			called = true
		}

		pipeline := Pipeline{Handlers: []Middleware{}}
		result := pipeline.Apply(handler)
		require.NotNil(t, result)

		// Execute the handler
		ctx := &fasthttp.RequestCtx{}
		result(ctx)
		assert.True(t, called)
	})

	t.Run("applies middleware in order", func(t *testing.T) {
		var order []int

		mw1 := func(h fasthttp.RequestHandler) fasthttp.RequestHandler {
			return func(ctx *fasthttp.RequestCtx) {
				order = append(order, 1)
				h(ctx)
			}
		}

		mw2 := func(h fasthttp.RequestHandler) fasthttp.RequestHandler {
			return func(ctx *fasthttp.RequestCtx) {
				order = append(order, 2)
				h(ctx)
			}
		}

		handler := func(ctx *fasthttp.RequestCtx) {
			order = append(order, 3)
		}

		pipeline := Pipeline{Handlers: []Middleware{mw1, mw2}}
		result := pipeline.Apply(handler)

		ctx := &fasthttp.RequestCtx{}
		result(ctx)

		assert.Equal(t, []int{1, 2, 3}, order)
	})
}
