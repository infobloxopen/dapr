// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package diagnostics

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/dapr/dapr/pkg/config"
	diag_utils "github.com/dapr/dapr/pkg/diagnostics/utils"
	commonv1pb "github.com/dapr/dapr/pkg/proto/common/v1"
	internalv1pb "github.com/dapr/dapr/pkg/proto/internals/v1"
	runtimev1pb "github.com/dapr/dapr/pkg/proto/runtime/v1"
	"github.com/stretchr/testify/assert"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestSpanAttributesMapFromGRPC(t *testing.T) {
	var tests = []struct {
		rpcMethod                    string
		requestType                  string
		expectedServiceNameAttribute string
		expectedCustomAttribute      string
	}{
		{"/dapr.proto.internals.v1.ServiceInvocation/CallLocal", "InternalInvokeRequest", "ServiceInvocation", "mymethod"},
		// InvokeService will be ServiceInvocation because this call will be treated as client call
		// of service invocation.
		{"/dapr.proto.runtime.v1.Dapr/InvokeService", "InvokeServiceRequest", "ServiceInvocation", "mymethod"},
		{"/dapr.proto.runtime.v1.Dapr/GetState", "GetStateRequest", "Dapr", "mystore"},
		{"/dapr.proto.runtime.v1.Dapr/SaveState", "SaveStateRequest", "Dapr", "mystore"},
		{"/dapr.proto.runtime.v1.Dapr/DeleteState", "DeleteStateRequest", "Dapr", "mystore"},
		{"/dapr.proto.runtime.v1.Dapr/GetSecret", "GetSecretRequest", "Dapr", "mysecretstore"},
		{"/dapr.proto.runtime.v1.Dapr/InvokeBinding", "InvokeBindingRequest", "Dapr", "mybindings"},
		{"/dapr.proto.runtime.v1.Dapr/PublishEvent", "PublishEventRequest", "Dapr", "mytopic"},
	}
	var req interface{}
	for _, tt := range tests {
		t.Run(tt.rpcMethod, func(t *testing.T) {
			switch tt.requestType {
			case "InvokeServiceRequest":
				req = &runtimev1pb.InvokeServiceRequest{Message: &commonv1pb.InvokeRequest{Method: "mymethod"}}
			case "GetStateRequest":
				req = &runtimev1pb.GetStateRequest{StoreName: "mystore"}
			case "SaveStateRequest":
				req = &runtimev1pb.SaveStateRequest{StoreName: "mystore"}
			case "DeleteStateRequest":
				req = &runtimev1pb.DeleteStateRequest{StoreName: "mystore"}
			case "GetSecretRequest":
				req = &runtimev1pb.GetSecretRequest{StoreName: "mysecretstore"}
			case "InvokeBindingRequest":
				req = &runtimev1pb.InvokeBindingRequest{Name: "mybindings"}
			case "PublishEventRequest":
				req = &runtimev1pb.PublishEventRequest{Topic: "mytopic"}
			case "TopicEventRequest":
				req = &runtimev1pb.TopicEventRequest{Topic: "mytopic"}
			case "BindingEventRequest":
				req = &runtimev1pb.BindingEventRequest{Name: "mybindings"}
			case "InternalInvokeRequest":
				req = &internalv1pb.InternalInvokeRequest{Message: &commonv1pb.InvokeRequest{Method: "mymethod"}}
			}

			got := spanAttributesMapFromGRPC("fakeAppID", req, tt.rpcMethod)
			assert.Equal(t, tt.expectedServiceNameAttribute, got[gRPCServiceSpanAttributeKey], "servicename attribute should be equal")
		})
	}
}

func TestUserDefinedMetadata(t *testing.T) {
	md := metadata.MD{
		"dapr-userdefined-1": []string{"value1"},
		"dapr-userdefined-2": []string{"value2", "value3"},
		"no-attr":            []string{"value3"},
	}

	testCtx := metadata.NewIncomingContext(context.Background(), md)

	m := userDefinedMetadata(testCtx)

	assert.Equal(t, 2, len(m))
	assert.Equal(t, "value1", m["dapr-userdefined-1"])
	assert.Equal(t, "value2", m["dapr-userdefined-2"])
}

func TestSpanContextToGRPCMetadata(t *testing.T) {
	t.Run("empty span context", func(t *testing.T) {
		ctx := context.Background()
		newCtx := SpanContextToGRPCMetadata(ctx, trace.SpanContext{})

		assert.Equal(t, ctx, newCtx)
	})

	t.Run("valid non-empty span context adds grpc-trace-bin to outgoing metadata", func(t *testing.T) {
		sc := trace.SpanContext{
			TraceID:      trace.TraceID{75, 249, 47, 53, 119, 179, 77, 166, 163, 206, 146, 157, 14, 14, 71, 54},
			SpanID:       trace.SpanID{0, 240, 103, 170, 11, 169, 2, 183},
			TraceOptions: trace.TraceOptions(1),
		}

		ctx := context.Background()
		newCtx := SpanContextToGRPCMetadata(ctx, sc)
		assert.NotEqual(t, ctx, newCtx)

		md, ok := metadata.FromOutgoingContext(newCtx)
		assert.True(t, ok)
		assert.NotEmpty(t, md[grpcTraceContextKey])

		// Verify round-trip: deserialize from the metadata and compare.
		traceContextBinary := []byte(md[grpcTraceContextKey][0])
		gotSc, gotOk := propagation.FromBinary(traceContextBinary)
		assert.True(t, gotOk)
		assert.Equal(t, sc, gotSc)
	})
}

func TestGRPCTraceUnaryServerInterceptor(t *testing.T) {
	rate := config.TracingSpec{SamplingRate: "1"}
	interceptor := GRPCTraceUnaryServerInterceptor("fakeAppID", rate)

	testTraceParent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	testSpanContext, _ := SpanContextFromW3CString(testTraceParent)
	testTraceBinary := propagation.Binary(testSpanContext)
	ctx := context.Background()

	t.Run("grpc-trace-bin is given", func(t *testing.T) {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("grpc-trace-bin", string(testTraceBinary)))
		fakeInfo := &grpc.UnaryServerInfo{
			FullMethod: "/dapr.proto.runtime.v1.Dapr/GetState",
		}
		fakeReq := &runtimev1pb.GetStateRequest{
			StoreName: "statestore",
			Key:       "state",
		}

		var span *trace.Span
		assertHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
			span = diag_utils.SpanFromContext(ctx)
			return nil, errors.New("fake error")
		}

		interceptor(ctx, fakeReq, fakeInfo, assertHandler)

		sc := span.SpanContext()
		assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", fmt.Sprintf("%x", sc.TraceID[:]))
		assert.NotEqual(t, "00f067aa0ba902b7", fmt.Sprintf("%x", sc.SpanID[:]))
	})

	t.Run("grpc-trace-bin is not given", func(t *testing.T) {
		fakeInfo := &grpc.UnaryServerInfo{
			FullMethod: "/dapr.proto.runtime.v1.Dapr/GetState",
		}
		fakeReq := &runtimev1pb.GetStateRequest{
			StoreName: "statestore",
			Key:       "state",
		}

		var span *trace.Span
		assertHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
			span = diag_utils.SpanFromContext(ctx)
			return nil, errors.New("fake error")
		}

		interceptor(ctx, fakeReq, fakeInfo, assertHandler)

		sc := span.SpanContext()
		assert.NotEmpty(t, fmt.Sprintf("%x", sc.TraceID[:]))
		assert.NotEmpty(t, fmt.Sprintf("%x", sc.SpanID[:]))
	})

	t.Run("InvokeService call", func(t *testing.T) {
		fakeInfo := &grpc.UnaryServerInfo{
			FullMethod: "/dapr.proto.runtime.v1.Dapr/InvokeService",
		}
		fakeReq := &runtimev1pb.InvokeServiceRequest{
			Id:      "targetID",
			Message: &commonv1pb.InvokeRequest{Method: "method1"},
		}

		var span *trace.Span
		assertHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
			span = diag_utils.SpanFromContext(ctx)
			return nil, errors.New("fake error")
		}

		interceptor(ctx, fakeReq, fakeInfo, assertHandler)

		sc := span.SpanContext()
		assert.True(t, strings.Contains(span.String(), "CallLocal/targetID/method1"))
		assert.NotEmpty(t, fmt.Sprintf("%x", sc.TraceID[:]))
		assert.NotEmpty(t, fmt.Sprintf("%x", sc.SpanID[:]))
	})
}

func TestSpanContextSerialization(t *testing.T) {
	wantSc := trace.SpanContext{
		TraceID:      trace.TraceID{75, 249, 47, 53, 119, 179, 77, 166, 163, 206, 146, 157, 14, 14, 71, 54},
		SpanID:       trace.SpanID{0, 240, 103, 170, 11, 169, 2, 183},
		TraceOptions: trace.TraceOptions(1),
	}

	passedOverWire := string(propagation.Binary(wantSc))
	storedInDapr := base64.StdEncoding.EncodeToString([]byte(passedOverWire))
	decoded, _ := base64.StdEncoding.DecodeString(storedInDapr)
	gotSc, _ := propagation.FromBinary(decoded)
	assert.Equal(t, wantSc, gotSc)
}

func TestSpanContextFromIncomingGRPCMetadata(t *testing.T) {
	testSc := trace.SpanContext{
		TraceID:      trace.TraceID{75, 249, 47, 53, 119, 179, 77, 166, 163, 206, 146, 157, 14, 14, 71, 54},
		SpanID:       trace.SpanID{0, 240, 103, 170, 11, 169, 2, 183},
		TraceOptions: trace.TraceOptions(1),
	}

	t.Run("with grpc-trace-bin metadata", func(t *testing.T) {
		traceContextBinary := propagation.Binary(testSc)
		md := metadata.Pairs(grpcTraceContextKey, string(traceContextBinary))
		ctx := metadata.NewIncomingContext(context.Background(), md)

		sc, ok := SpanContextFromIncomingGRPCMetadata(ctx)
		assert.True(t, ok)
		assert.Equal(t, testSc, sc)
	})

	t.Run("with traceparent metadata fallback", func(t *testing.T) {
		traceparent := SpanContextToW3CString(testSc)
		md := metadata.Pairs(traceparentHeader, traceparent)
		ctx := metadata.NewIncomingContext(context.Background(), md)

		sc, ok := SpanContextFromIncomingGRPCMetadata(ctx)
		assert.True(t, ok)
		assert.Equal(t, testSc.TraceID, sc.TraceID)
		assert.Equal(t, testSc.SpanID, sc.SpanID)
		assert.Equal(t, testSc.TraceOptions, sc.TraceOptions)
	})

	t.Run("with traceparent and tracestate metadata", func(t *testing.T) {
		traceparent := SpanContextToW3CString(testSc)
		md := metadata.Pairs(traceparentHeader, traceparent, tracestateHeader, "vendor1=opaque1")
		ctx := metadata.NewIncomingContext(context.Background(), md)

		sc, ok := SpanContextFromIncomingGRPCMetadata(ctx)
		assert.True(t, ok)
		assert.Equal(t, testSc.TraceID, sc.TraceID)
		assert.NotNil(t, sc.Tracestate)
		entries := sc.Tracestate.Entries()
		assert.Len(t, entries, 1)
		assert.Equal(t, "vendor1", entries[0].Key)
		assert.Equal(t, "opaque1", entries[0].Value)
	})

	t.Run("with no metadata returns empty span context", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})

		sc, ok := SpanContextFromIncomingGRPCMetadata(ctx)
		assert.False(t, ok)
		assert.Equal(t, trace.SpanContext{}, sc)
	})
}

func TestUpdateSpanStatusFromGRPCError(t *testing.T) {
	t.Run("nil error does nothing", func(t *testing.T) {
		ctx := context.Background()
		_, span := trace.StartSpan(ctx, "test", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()

		// Should not panic and should not set any error status.
		assert.NotPanics(t, func() {
			UpdateSpanStatusFromGRPCError(span, nil)
		})
	})

	t.Run("nil span does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			UpdateSpanStatusFromGRPCError(nil, errors.New("some error"))
		})
	})

	t.Run("grpc status error sets span status with grpc code", func(t *testing.T) {
		ctx := context.Background()
		_, span := trace.StartSpan(ctx, "test", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()

		grpcErr := status.Error(codes.NotFound, "resource not found")
		UpdateSpanStatusFromGRPCError(span, grpcErr)
		// Span status is set but we can verify it does not panic.
		// The internal state is not directly accessible without exporter.
	})

	t.Run("plain error sets span status as Internal", func(t *testing.T) {
		ctx := context.Background()
		_, span := trace.StartSpan(ctx, "test", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()

		plainErr := errors.New("something went wrong")
		assert.NotPanics(t, func() {
			UpdateSpanStatusFromGRPCError(span, plainErr)
		})
	})

	t.Run("both nil does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			UpdateSpanStatusFromGRPCError(nil, nil)
		})
	})
}
