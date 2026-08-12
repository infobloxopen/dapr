// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"go.opencensus.io/trace"
)

func TestSpanFromContext(t *testing.T) {
	t.Run("fasthttp.RequestCtx, not nil span", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		SpanToFastHTTPContext(ctx, &trace.Span{})

		assert.NotNil(t, SpanFromContext(ctx))
	})

	t.Run("fasthttp.RequestCtx, nil span", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		SpanToFastHTTPContext(ctx, nil)

		assert.Nil(t, SpanFromContext(ctx))
	})

	t.Run("not nil span for context", func(t *testing.T) {
		ctx := context.Background()
		newCtx := trace.NewContext(ctx, &trace.Span{})

		assert.NotNil(t, SpanFromContext(newCtx))
	})

	t.Run("nil span for context", func(t *testing.T) {
		ctx := context.Background()
		newCtx := trace.NewContext(ctx, nil)

		assert.Nil(t, SpanFromContext(newCtx))
	})

	t.Run("nil", func(t *testing.T) {
		ctx := context.Background()

		assert.Nil(t, SpanFromContext(ctx))
	})
}

func TestGetTraceSamplingRate(t *testing.T) {
	tests := []struct {
		name     string
		rate     string
		expected float64
	}{
		{"valid rate", "0.5", 0.5},
		{"zero rate", "0", 0},
		{"full rate", "1", 1},
		{"empty string returns default", "", defaultSamplingRate},
		{"invalid string returns default", "invalid", defaultSamplingRate},
		{"negative rate", "-0.1", -0.1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTraceSamplingRate(tt.rate)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTracingEnabled(t *testing.T) {
	tests := []struct {
		name     string
		rate     string
		expected bool
	}{
		{"enabled with rate 1", "1", true},
		{"enabled with rate 0.5", "0.5", true},
		{"disabled with rate 0", "0", false},
		{"enabled with empty string", "", true},
		{"enabled with invalid string", "invalid", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTracingEnabled(tt.rate)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTraceSampler(t *testing.T) {
	t.Run("returns start option", func(t *testing.T) {
		opt := TraceSampler("0.5")
		assert.NotNil(t, opt)
	})
}

func TestStdoutExporter(t *testing.T) {
	t.Run("implements Exporter interface", func(t *testing.T) {
		e := &StdoutExporter{}
		// Just verify it doesn't panic on a nil-safe call
		e.ExportSpan(&trace.SpanData{
			SpanContext: trace.SpanContext{},
		})
	})
}
