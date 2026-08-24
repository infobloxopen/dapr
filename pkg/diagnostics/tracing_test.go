// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package diagnostics

import (
	"context"
	"testing"

	"github.com/dapr/dapr/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/tracestate"
)

func TestSpanContextToW3CString(t *testing.T) {
	t.Run("empty SpanContext", func(t *testing.T) {
		expected := "00-00000000000000000000000000000000-0000000000000000-00"
		sc := trace.SpanContext{}
		got := SpanContextToW3CString(sc)
		assert.Equal(t, expected, got)
	})
	t.Run("valid SpanContext", func(t *testing.T) {
		expected := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		sc := trace.SpanContext{
			TraceID:      trace.TraceID{75, 249, 47, 53, 119, 179, 77, 166, 163, 206, 146, 157, 14, 14, 71, 54},
			SpanID:       trace.SpanID{0, 240, 103, 170, 11, 169, 2, 183},
			TraceOptions: trace.TraceOptions(1)}
		got := SpanContextToW3CString(sc)
		assert.Equal(t, expected, got)
	})
}

func TestTraceStateToW3CString(t *testing.T) {
	t.Run("empty Tracestate", func(t *testing.T) {
		sc := trace.SpanContext{}
		got := TraceStateToW3CString(sc)
		assert.Empty(t, got)
	})
	t.Run("valid Tracestate", func(t *testing.T) {
		entry := tracestate.Entry{Key: "key", Value: "value"}
		ts, _ := tracestate.New(nil, entry)
		sc := trace.SpanContext{}
		sc.Tracestate = ts
		got := TraceStateToW3CString(sc)
		assert.Equal(t, "key=value", got)
	})
}

func TestTraceStateFromW3CString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		wantKV  map[string]string
	}{
		{
			name:    "valid tracestate with two pairs",
			input:   "key1=value1,key2=value2",
			wantNil: false,
			wantKV:  map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:    "empty string returns nil",
			input:   "",
			wantNil: true,
		},
		{
			name:    "malformed pair missing value",
			input:   "key1value1",
			wantNil: true,
		},
		{
			name:    "single valid pair",
			input:   "vendor1=opaqueValue1",
			wantNil: false,
			wantKV:  map[string]string{"vendor1": "opaqueValue1"},
		},
		{
			name:    "pair with too many equals signs",
			input:   "key1=val=ue",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TraceStateFromW3CString(tt.input)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				entries := got.Entries()
				gotKV := make(map[string]string, len(entries))
				for _, e := range entries {
					gotKV[e.Key] = e.Value
				}
				assert.Equal(t, tt.wantKV, gotKV)
			}
		})
	}
}

func TestSpanContextFromW3CString(t *testing.T) {
	tests := []struct {
		name   string
		header string
		wantSc trace.SpanContext
		wantOk bool
	}{
		{
			name:   "valid traceparent with sampled flag",
			header: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			wantSc: trace.SpanContext{
				TraceID:      trace.TraceID{75, 249, 47, 53, 119, 179, 77, 166, 163, 206, 146, 157, 14, 14, 71, 54},
				SpanID:       trace.SpanID{0, 240, 103, 170, 11, 169, 2, 183},
				TraceOptions: trace.TraceOptions(1),
			},
			wantOk: true,
		},
		{
			name:   "valid traceparent with not-sampled flag",
			header: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00",
			wantSc: trace.SpanContext{
				TraceID:      trace.TraceID{75, 249, 47, 53, 119, 179, 77, 166, 163, 206, 146, 157, 14, 14, 71, 54},
				SpanID:       trace.SpanID{0, 240, 103, 170, 11, 169, 2, 183},
				TraceOptions: trace.TraceOptions(0),
			},
			wantOk: true,
		},
		{
			name:   "empty string",
			header: "",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "too few sections",
			header: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "invalid version hex",
			header: "zz-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "version too high (255)",
			header: "ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "trace-id wrong length",
			header: "00-4bf92f35-00f067aa0ba902b7-01",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "span-id wrong length",
			header: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067-01",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "all-zero trace ID",
			header: "00-00000000000000000000000000000000-00f067aa0ba902b7-01",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "all-zero span ID",
			header: "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "invalid trace-id hex characters",
			header: "00-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx-00f067aa0ba902b7-01",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "invalid span-id hex characters",
			header: "00-4bf92f3577b34da6a3ce929d0e0e4736-xxxxxxxxxxxxxxxx-01",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "invalid trace-options hex",
			header: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-zz",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "version 0 with extra sections",
			header: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-extra",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "version field wrong length (3 chars)",
			header: "000-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			wantSc: trace.SpanContext{},
			wantOk: false,
		},
		{
			name:   "future version with extra sections is valid",
			header: "02-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-extra",
			wantSc: trace.SpanContext{
				TraceID:      trace.TraceID{75, 249, 47, 53, 119, 179, 77, 166, 163, 206, 146, 157, 14, 14, 71, 54},
				SpanID:       trace.SpanID{0, 240, 103, 170, 11, 169, 2, 183},
				TraceOptions: trace.TraceOptions(1),
			},
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, ok := SpanContextFromW3CString(tt.header)
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.wantSc, sc)
		})
	}
}

func TestAddAttributesToSpan(t *testing.T) {
	t.Run("nil span does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			AddAttributesToSpan(nil, map[string]string{"key": "value"})
		})
	})

	t.Run("adds normal attributes and skips internal prefix and empty values", func(t *testing.T) {
		ctx := context.Background()
		_, span := trace.StartSpan(ctx, "testspan", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()

		attrs := map[string]string{
			"normalKey":                          "normalValue",
			"anotherKey":                         "anotherValue",
			daprInternalSpanAttrPrefix + "inner": "should_skip",
			"emptyValueKey":                      "",
		}
		// Should not panic and should silently skip internal/empty attributes.
		assert.NotPanics(t, func() {
			AddAttributesToSpan(span, attrs)
		})
	})

	t.Run("nil attributes map does not panic", func(t *testing.T) {
		_, span := trace.StartSpan(context.Background(), "testspan", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()

		assert.NotPanics(t, func() {
			AddAttributesToSpan(span, nil)
		})
	})
}

func TestConstructInputBindingSpanAttributes(t *testing.T) {
	m := ConstructInputBindingSpanAttributes("myBinding", "http://localhost:3500")
	assert.Equal(t, "myBinding", m[dbNameSpanAttributeKey])
	assert.Equal(t, daprGRPCDaprService, m[gRPCServiceSpanAttributeKey])
	assert.Equal(t, bindingBuildingBlockType, m[dbSystemSpanAttributeKey])
	assert.Equal(t, "http://localhost:3500", m[dbConnectionStringSpanAttributeKey])
	assert.Len(t, m, 4)
}

func TestConstructSubscriptionSpanAttributes(t *testing.T) {
	m := ConstructSubscriptionSpanAttributes("myTopic")
	assert.Equal(t, pubsubBuildingBlockType, m[messagingSystemSpanAttributeKey])
	assert.Equal(t, "myTopic", m[messagingDestinationSpanAttributeKey])
	assert.Equal(t, messagingDestinationTopicKind, m[messagingDestinationKindSpanAttributeKey])
	assert.Len(t, m, 3)
}

func TestStartInternalCallbackSpan(t *testing.T) {
	t.Run("tracing enabled creates non-nil span", func(t *testing.T) {
		spec := config.TracingSpec{SamplingRate: "1"}
		parentSc := trace.SpanContext{
			TraceID:      trace.TraceID{75, 249, 47, 53, 119, 179, 77, 166, 163, 206, 146, 157, 14, 14, 71, 54},
			SpanID:       trace.SpanID{0, 240, 103, 170, 11, 169, 2, 183},
			TraceOptions: trace.TraceOptions(1),
		}

		ctx, span := StartInternalCallbackSpan("testSpan", parentSc, spec)
		require.NotNil(t, ctx)
		require.NotNil(t, span)
		defer span.End()

		// The span should carry the same trace ID as the parent.
		assert.Equal(t, parentSc.TraceID, span.SpanContext().TraceID)
	})

	t.Run("tracing disabled returns nil span", func(t *testing.T) {
		spec := config.TracingSpec{SamplingRate: "0"}
		parentSc := trace.SpanContext{
			TraceID:      trace.TraceID{75, 249, 47, 53, 119, 179, 77, 166, 163, 206, 146, 157, 14, 14, 71, 54},
			SpanID:       trace.SpanID{0, 240, 103, 170, 11, 169, 2, 183},
			TraceOptions: trace.TraceOptions(1),
		}

		ctx, span := StartInternalCallbackSpan("testSpan", parentSc, spec)
		require.NotNil(t, ctx)
		assert.Nil(t, span)
	})
}

// This test would allow us to know when the span attribute keys are
// modified in go.opentelemetry.io/otel/semconv library, and thus in
// the spec.
func TestOtelConventionStrings(t *testing.T) {
	assert.Equal(t, "db.system", dbSystemSpanAttributeKey)
	assert.Equal(t, "db.name", dbNameSpanAttributeKey)
	assert.Equal(t, "db.statement", dbStatementSpanAttributeKey)
	assert.Equal(t, "db.connection_string", dbConnectionStringSpanAttributeKey)
	assert.Equal(t, "topic", messagingDestinationTopicKind)
	assert.Equal(t, "messaging.system", messagingSystemSpanAttributeKey)
	assert.Equal(t, "messaging.destination", messagingDestinationSpanAttributeKey)
	assert.Equal(t, "messaging.destination_kind", messagingDestinationKindSpanAttributeKey)
	assert.Equal(t, "rpc.service", gRPCServiceSpanAttributeKey)
	assert.Equal(t, "net.peer.name", netPeerNameSpanAttributeKey)
}
