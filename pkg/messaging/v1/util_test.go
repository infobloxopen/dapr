// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package v1

import (
	"context"
	"encoding/base64"
	"sort"
	"strings"
	"testing"

	internalv1pb "github.com/dapr/dapr/pkg/proto/internals/v1"
	"github.com/stretchr/testify/assert"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/propagation"
	epb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestInternalMetadataToHTTPHeader(t *testing.T) {
	testValue := &internalv1pb.ListStringValue{
		Values: []string{"fakeValue"},
	}

	fakeMetadata := map[string]*internalv1pb.ListStringValue{
		"custom-header":  testValue,
		":method":        testValue,
		":scheme":        testValue,
		":path":          testValue,
		":authority":     testValue,
		"grpc-timeout":   testValue,
		"content-type":   testValue, // skip
		"grpc-trace-bin": testValue,
	}

	expectedKeyNames := []string{"custom-header", "dapr-method", "dapr-scheme", "dapr-path", "dapr-authority", "dapr-grpc-timeout"}
	savedHeaderKeyNames := []string{}
	ctx := context.Background()
	InternalMetadataToHTTPHeader(ctx, fakeMetadata, func(k, v string) {
		savedHeaderKeyNames = append(savedHeaderKeyNames, k)
	})

	sort.Strings(expectedKeyNames)
	sort.Strings(savedHeaderKeyNames)

	assert.Equal(t, expectedKeyNames, savedHeaderKeyNames)
}

func TestGrpcMetadataToInternalMetadata(t *testing.T) {
	var keyBinValue = []byte{101, 200}
	testMD := metadata.Pairs(
		"key", "key value",
		"key-bin", string(keyBinValue),
	)
	internalMD := MetadataToInternalMetadata(testMD)

	assert.Equal(t, "key value", internalMD["key"].GetValues()[0])
	assert.Equal(t, 1, len(internalMD["key"].GetValues()))

	assert.Equal(t, base64.StdEncoding.EncodeToString(keyBinValue), internalMD["key-bin"].GetValues()[0], "binary metadata must be saved")
	assert.Equal(t, 1, len(internalMD["key-bin"].GetValues()))
}

func TestIsJSONContentType(t *testing.T) {
	var contentTypeTests = []struct {
		in  string
		out bool
	}{
		{"application/json", true},
		{"text/plains; charset=utf-8", false},
		{"application/json; charset=utf-8", true},
	}

	for _, tt := range contentTypeTests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.out, IsJSONContentType(tt.in))
		})
	}
}

func TestInternalMetadataToGrpcMetadata(t *testing.T) {
	httpHeaders := map[string]*internalv1pb.ListStringValue{
		"Host": {
			Values: []string{"localhost"},
		},
		"Content-Type": {
			Values: []string{"application/json"},
		},
		"Content-Length": {
			Values: []string{"2000"},
		},
		"Connection": {
			Values: []string{"keep-alive"},
		},
		"Keep-Alive": {
			Values: []string{"timeout=5", "max=1000"},
		},
		"Proxy-Connection": {
			Values: []string{"keep-alive"},
		},
		"Transfer-Encoding": {
			Values: []string{"gzip", "chunked"},
		},
		"Upgrade": {
			Values: []string{"WebSocket"},
		},
		"Accept-Encoding": {
			Values: []string{"gzip, deflate"},
		},
		"User-Agent": {
			Values: []string{"Go-http-client/1.1"},
		},
	}

	ctx := context.Background()

	t.Run("without http header conversion for http headers", func(t *testing.T) {
		convertedMD := InternalMetadataToGrpcMetadata(ctx, httpHeaders, false)
		// always trace header is returned
		assert.Equal(t, 11, convertedMD.Len())

		var testHeaders = []struct {
			key      string
			expected string
		}{
			{"host", "localhost"},
			{"connection", "keep-alive"},
			{"content-length", "2000"},
			{"content-type", "application/json"},
			{"keep-alive", "timeout=5"},
			{"proxy-connection", "keep-alive"},
			{"transfer-encoding", "gzip"},
			{"upgrade", "WebSocket"},
			{"accept-encoding", "gzip, deflate"},
			{"user-agent", "Go-http-client/1.1"},
		}

		for _, ht := range testHeaders {
			assert.Equal(t, ht.expected, convertedMD[ht.key][0])
		}
	})

	t.Run("with http header conversion for http headers", func(t *testing.T) {
		convertedMD := InternalMetadataToGrpcMetadata(ctx, httpHeaders, true)
		// always trace header is returned
		assert.Equal(t, 11, convertedMD.Len())

		var testHeaders = []struct {
			key      string
			expected string
		}{
			{"dapr-host", "localhost"},
			{"dapr-connection", "keep-alive"},
			{"dapr-content-length", "2000"},
			{"dapr-content-type", "application/json"},
			{"dapr-keep-alive", "timeout=5"},
			{"dapr-proxy-connection", "keep-alive"},
			{"dapr-transfer-encoding", "gzip"},
			{"dapr-upgrade", "WebSocket"},
			{"accept-encoding", "gzip, deflate"},
			{"user-agent", "Go-http-client/1.1"},
		}

		for _, ht := range testHeaders {
			assert.Equal(t, ht.expected, convertedMD[ht.key][0])
		}
	})

	var keyBinValue = []byte{100, 50}
	var keyBinEncodedValue = base64.StdEncoding.EncodeToString(keyBinValue)

	traceBinValue := []byte{10, 30, 50, 60}
	traceBinValueEncodedValue := base64.StdEncoding.EncodeToString(traceBinValue)

	grpcMetadata := map[string]*internalv1pb.ListStringValue{
		"content-type": {
			Values: []string{"application/grpc"},
		},
		":authority": {
			Values: []string{"localhost"},
		},
		"grpc-timeout": {
			Values: []string{"1S"},
		},
		"grpc-encoding": {
			Values: []string{"gzip, deflate"},
		},
		"authorization": {
			Values: []string{"bearer token"},
		},
		"grpc-trace-bin": {
			Values: []string{traceBinValueEncodedValue},
		},
		"my-metadata": {
			Values: []string{"value1", "value2", "value3"},
		},
		"key-bin": {
			Values: []string{keyBinEncodedValue, keyBinEncodedValue},
		},
	}

	t.Run("with grpc header conversion for grpc headers", func(t *testing.T) {
		convertedMD := InternalMetadataToGrpcMetadata(ctx, grpcMetadata, true)
		assert.Equal(t, 8, convertedMD.Len())
		assert.Equal(t, "localhost", convertedMD[":authority"][0])
		assert.Equal(t, "1S", convertedMD["grpc-timeout"][0])
		assert.Equal(t, "gzip, deflate", convertedMD["grpc-encoding"][0])
		assert.Equal(t, "bearer token", convertedMD["authorization"][0])
		_, ok := convertedMD["grpc-trace-bin"]
		assert.True(t, ok)
		assert.Equal(t, "value1", convertedMD["my-metadata"][0])
		assert.Equal(t, "value2", convertedMD["my-metadata"][1])
		assert.Equal(t, "value3", convertedMD["my-metadata"][2])
		assert.Equal(t, string(keyBinValue), convertedMD["key-bin"][0])
		assert.Equal(t, string(keyBinValue), convertedMD["key-bin"][1])
		assert.Equal(t, string(traceBinValue), convertedMD["grpc-trace-bin"][0])
	})
}

func TestErrorFromHTTPResponseCode(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		// act
		err := ErrorFromHTTPResponseCode(200, "OK")

		// assert
		assert.NoError(t, err)
	})

	t.Run("Created", func(t *testing.T) {
		// act
		err := ErrorFromHTTPResponseCode(201, "Created")

		// assert
		assert.NoError(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		// act
		err := ErrorFromHTTPResponseCode(404, "Not Found")

		// assert
		s, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.NotFound, s.Code())
		assert.Equal(t, "Not Found", s.Message())
		errInfo := (s.Details()[0]).(*epb.ErrorInfo)
		assert.Equal(t, "404", errInfo.GetMetadata()[errorInfoHTTPCodeMetadata])
		assert.Equal(t, "Not Found", errInfo.GetMetadata()[errorInfoHTTPErrorMetadata])
	})

	t.Run("Internal Server Error", func(t *testing.T) {
		// act
		err := ErrorFromHTTPResponseCode(500, "HTTPExtensions is not given")

		// assert
		s, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Unknown, s.Code())
		assert.Equal(t, "Internal Server Error", s.Message())
		errInfo := (s.Details()[0]).(*epb.ErrorInfo)
		assert.Equal(t, "500", errInfo.GetMetadata()[errorInfoHTTPCodeMetadata])
		assert.Equal(t, "HTTPExtensions is not given", errInfo.GetMetadata()[errorInfoHTTPErrorMetadata])
	})

	t.Run("Truncate error message", func(t *testing.T) {
		longMessage := strings.Repeat("test", 30)

		// act
		err := ErrorFromHTTPResponseCode(500, longMessage)

		// assert
		s, _ := status.FromError(err)
		errInfo := (s.Details()[0]).(*epb.ErrorInfo)
		assert.Equal(t, 63, len(errInfo.GetMetadata()[errorInfoHTTPErrorMetadata]))
	})
}

func TestErrorFromInternalStatus(t *testing.T) {
	expected := status.New(codes.Internal, "Internal Service Error")
	expected.WithDetails(
		&epb.DebugInfo{
			StackEntries: []string{
				"first stack",
				"second stack",
			},
		},
	)

	internal := &internalv1pb.Status{
		Code:    expected.Proto().Code,
		Message: expected.Proto().Message,
		Details: expected.Proto().Details,
	}

	expected.Message()

	// act
	statusError := ErrorFromInternalStatus(internal)

	// assert
	actual, ok := status.FromError(statusError)
	assert.True(t, ok)
	assert.Equal(t, expected.Code(), actual.Code())
	assert.Equal(t, expected.Message(), actual.Message())
	assert.Equal(t, expected.Details(), actual.Details())
}

func TestCloneBytes(t *testing.T) {
	t.Run("data is nil", func(t *testing.T) {
		assert.Nil(t, cloneBytes(nil))
	})

	t.Run("data is empty", func(t *testing.T) {
		orig := []byte{}

		assert.Equal(t, orig, cloneBytes(orig))
		assert.NotSame(t, orig, cloneBytes(orig))
	})

	t.Run("data is not empty", func(t *testing.T) {
		orig := []byte("fakedata")

		assert.Equal(t, orig, cloneBytes(orig))
		assert.NotSame(t, orig, cloneBytes(orig))
	})
}

func TestProtobufToJSON(t *testing.T) {
	tpb := &epb.DebugInfo{
		StackEntries: []string{
			"first stack",
			"second stack",
		},
	}

	jsonBody, err := ProtobufToJSON(tpb)
	assert.NoError(t, err)
	t.Log(string(jsonBody))

	// protojson produces different indentation space based on OS
	// For linux
	comp1 := string(jsonBody) == "{\"stackEntries\":[\"first stack\",\"second stack\"]}"
	// For mac and windows
	comp2 := string(jsonBody) == "{\"stackEntries\":[\"first stack\", \"second stack\"]}"
	assert.True(t, comp1 || comp2)
}

func TestHTTPStatusFromCode(t *testing.T) {
	tests := []struct {
		name     string
		code     codes.Code
		expected int
	}{
		{"OK", codes.OK, 200},
		{"Canceled", codes.Canceled, 408},
		{"Unknown", codes.Unknown, 500},
		{"InvalidArgument", codes.InvalidArgument, 400},
		{"DeadlineExceeded", codes.DeadlineExceeded, 504},
		{"NotFound", codes.NotFound, 404},
		{"AlreadyExists", codes.AlreadyExists, 409},
		{"PermissionDenied", codes.PermissionDenied, 403},
		{"Unauthenticated", codes.Unauthenticated, 401},
		{"ResourceExhausted", codes.ResourceExhausted, 429},
		{"FailedPrecondition", codes.FailedPrecondition, 400},
		{"Aborted", codes.Aborted, 409},
		{"OutOfRange", codes.OutOfRange, 400},
		{"Unimplemented", codes.Unimplemented, 501},
		{"Internal", codes.Internal, 500},
		{"Unavailable", codes.Unavailable, 503},
		{"DataLoss", codes.DataLoss, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HTTPStatusFromCode(tt.code))
		})
	}
}

func TestCodeFromHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected codes.Code
	}{
		{"OK 200", 200, codes.OK},
		{"Created 201", 201, codes.OK},
		{"RequestTimeout", 408, codes.Canceled},
		{"InternalServerError", 500, codes.Unknown},
		{"BadRequest", 400, codes.Internal},
		{"GatewayTimeout", 504, codes.DeadlineExceeded},
		{"NotFound", 404, codes.NotFound},
		{"Conflict", 409, codes.AlreadyExists},
		{"Forbidden", 403, codes.PermissionDenied},
		{"Unauthorized", 401, codes.Unauthenticated},
		{"TooManyRequests", 429, codes.ResourceExhausted},
		{"NotImplemented", 501, codes.Unimplemented},
		{"ServiceUnavailable", 503, codes.Unavailable},
		{"Unmapped status", 999, codes.Unknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, CodeFromHTTPStatus(tt.status))
		})
	}
}

func TestIsPermanentHTTPHeader(t *testing.T) {
	tests := []struct {
		header   string
		expected bool
	}{
		{"Accept", true},
		{"Content-Type", true},
		{"Connection", true},
		{"Keep-Alive", true},
		{"Cookie", true},
		{"Host", true},
		{"X-Custom-Header", false},
		{"Authorization", false},
		{"User-Agent", false},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			assert.Equal(t, tt.expected, isPermanentHTTPHeader(tt.header))
		})
	}
}

func TestReservedGRPCMetadataToDaprPrefixHeader(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"method", ":method", "dapr-method"},
		{"scheme", ":scheme", "dapr-scheme"},
		{"path", ":path", "dapr-path"},
		{"authority", ":authority", "dapr-authority"},
		{"grpc prefix", "grpc-timeout", "dapr-grpc-timeout"},
		{"normal key", "custom-key", "custom-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, reservedGRPCMetadataToDaprPrefixHeader(tt.key))
		})
	}
}

func TestIsGRPCProtocol(t *testing.T) {
	t.Run("grpc content type", func(t *testing.T) {
		md := DaprInternalMetadata{
			ContentTypeHeader: &internalv1pb.ListStringValue{Values: []string{GRPCContentType}},
		}
		assert.True(t, IsGRPCProtocol(md))
	})

	t.Run("json content type", func(t *testing.T) {
		md := DaprInternalMetadata{
			ContentTypeHeader: &internalv1pb.ListStringValue{Values: []string{JSONContentType}},
		}
		assert.False(t, IsGRPCProtocol(md))
	})

	t.Run("empty metadata", func(t *testing.T) {
		md := DaprInternalMetadata{}
		assert.False(t, IsGRPCProtocol(md))
	})
}

func TestProcessGRPCToHTTPTraceHeaders(t *testing.T) {
	t.Run("valid base64 grpc-trace-bin value", func(t *testing.T) {
		// Build a valid binary-encoded SpanContext using propagation.Binary
		sc := trace.SpanContext{
			TraceOptions: 1,
		}
		copy(sc.TraceID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		copy(sc.SpanID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8})
		binSC := propagation.Binary(sc)
		encodedTraceCtx := base64.StdEncoding.EncodeToString(binSC)

		ctx := context.Background()
		headers := map[string]string{}
		processGRPCToHTTPTraceHeaders(ctx, encodedTraceCtx, func(k, v string) {
			headers[k] = v
		})

		assert.NotEmpty(t, headers["traceparent"], "traceparent header should be set")
	})

	t.Run("invalid base64 grpc-trace-bin value falls back to context span", func(t *testing.T) {
		ctx := context.Background()
		headers := map[string]string{}
		processGRPCToHTTPTraceHeaders(ctx, "not-valid-base64!!!", func(k, v string) {
			headers[k] = v
		})
		// With a background context, SpanFromContext returns a no-op span with empty SpanContext.
		// SpanContextToHTTPHeaders is a no-op for empty SpanContext, so no headers are set.
		// The key point is this does not panic.
	})

	t.Run("empty grpc-trace-bin value falls back to context span", func(t *testing.T) {
		ctx := context.Background()
		headers := map[string]string{}
		processGRPCToHTTPTraceHeaders(ctx, "", func(k, v string) {
			headers[k] = v
		})
		// Should not panic with empty string
	})

	t.Run("base64 with invalid binary span context falls back to context span", func(t *testing.T) {
		// Valid base64 but not a valid span context binary
		encodedTraceCtx := base64.StdEncoding.EncodeToString([]byte("short"))
		ctx := context.Background()
		headers := map[string]string{}
		processGRPCToHTTPTraceHeaders(ctx, encodedTraceCtx, func(k, v string) {
			headers[k] = v
		})
		// Should not panic
	})
}

func TestProcessHTTPToHTTPTraceHeaders(t *testing.T) {
	t.Run("empty traceparent falls back to context span", func(t *testing.T) {
		ctx := context.Background()
		headers := map[string]string{}
		processHTTPToHTTPTraceHeaders(ctx, "", "", func(k, v string) {
			headers[k] = v
		})
		// With background context, SpanFromContext returns no-op span; no headers set for empty span context.
	})

	t.Run("non-empty traceparent without tracestate", func(t *testing.T) {
		ctx := context.Background()
		headers := map[string]string{}
		tp := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		processHTTPToHTTPTraceHeaders(ctx, tp, "", func(k, v string) {
			headers[k] = v
		})

		assert.Equal(t, tp, headers["traceparent"])
		_, hasTracestate := headers["tracestate"]
		assert.False(t, hasTracestate, "tracestate should not be set when empty")
	})

	t.Run("non-empty traceparent with tracestate", func(t *testing.T) {
		ctx := context.Background()
		headers := map[string]string{}
		tp := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		ts := "vendor1=value1"
		processHTTPToHTTPTraceHeaders(ctx, tp, ts, func(k, v string) {
			headers[k] = v
		})

		assert.Equal(t, tp, headers["traceparent"])
		assert.Equal(t, ts, headers["tracestate"])
	})
}

func TestProcessGRPCToGRPCTraceHeader(t *testing.T) {
	t.Run("empty grpc-trace-bin falls back to context span", func(t *testing.T) {
		ctx := context.Background()
		md := metadata.MD{}
		processGRPCToGRPCTraceHeader(ctx, md, "")

		vals := md.Get("grpc-trace-bin")
		assert.Equal(t, 1, len(vals), "should set grpc-trace-bin from context span")
	})

	t.Run("valid base64 grpc-trace-bin value", func(t *testing.T) {
		sc := trace.SpanContext{
			TraceOptions: 1,
		}
		copy(sc.TraceID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		copy(sc.SpanID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8})
		binSC := propagation.Binary(sc)
		encodedValue := base64.StdEncoding.EncodeToString(binSC)

		ctx := context.Background()
		md := metadata.MD{}
		processGRPCToGRPCTraceHeader(ctx, md, encodedValue)

		vals := md.Get("grpc-trace-bin")
		assert.Equal(t, 1, len(vals))
		assert.Equal(t, string(binSC), vals[0])
	})

	t.Run("invalid base64 grpc-trace-bin value does not set header", func(t *testing.T) {
		ctx := context.Background()
		md := metadata.MD{}
		processGRPCToGRPCTraceHeader(ctx, md, "not-valid-base64!!!")

		vals := md.Get("grpc-trace-bin")
		assert.Empty(t, vals, "invalid base64 should not set grpc-trace-bin")
	})
}

func TestInternalMetadataToHTTPHeaderBinaryAndEdgeCases(t *testing.T) {
	t.Run("binary headers are skipped", func(t *testing.T) {
		fakeMetadata := DaprInternalMetadata{
			"custom-header": {Values: []string{"value1"}},
			"my-data-bin":   {Values: []string{"binaryvalue"}},
		}

		ctx := context.Background()
		headers := map[string]string{}
		InternalMetadataToHTTPHeader(ctx, fakeMetadata, func(k, v string) {
			headers[k] = v
		})

		assert.Equal(t, "value1", headers["custom-header"])
		_, hasBin := headers["my-data-bin"]
		assert.False(t, hasBin, "binary headers should be skipped")
	})

	t.Run("empty values list is skipped", func(t *testing.T) {
		fakeMetadata := DaprInternalMetadata{
			"empty-header": {Values: []string{}},
			"valid-header": {Values: []string{"valid"}},
		}

		ctx := context.Background()
		headers := map[string]string{}
		InternalMetadataToHTTPHeader(ctx, fakeMetadata, func(k, v string) {
			headers[k] = v
		})

		_, hasEmpty := headers["empty-header"]
		assert.False(t, hasEmpty, "headers with empty values should be skipped")
		assert.Equal(t, "valid", headers["valid-header"])
	})

	t.Run("destination-app-id header is skipped", func(t *testing.T) {
		fakeMetadata := DaprInternalMetadata{
			DestinationIDHeader: {Values: []string{"myapp"}},
			"custom-header":     {Values: []string{"value1"}},
		}

		ctx := context.Background()
		headers := map[string]string{}
		InternalMetadataToHTTPHeader(ctx, fakeMetadata, func(k, v string) {
			headers[k] = v
		})

		_, hasDest := headers[DestinationIDHeader]
		assert.False(t, hasDest, "destination-app-id should be skipped")
		assert.Equal(t, "value1", headers["custom-header"])
	})

	t.Run("grpc protocol converts grpc-trace-bin to http trace headers", func(t *testing.T) {
		sc := trace.SpanContext{
			TraceOptions: 1,
		}
		copy(sc.TraceID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		copy(sc.SpanID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8})
		binSC := propagation.Binary(sc)
		encodedTraceCtx := base64.StdEncoding.EncodeToString(binSC)

		fakeMetadata := DaprInternalMetadata{
			"content-type":   {Values: []string{GRPCContentType}},
			"grpc-trace-bin": {Values: []string{encodedTraceCtx}},
			"custom-header":  {Values: []string{"value1"}},
		}

		ctx := context.Background()
		headers := map[string]string{}
		InternalMetadataToHTTPHeader(ctx, fakeMetadata, func(k, v string) {
			headers[k] = v
		})

		assert.Equal(t, "value1", headers["custom-header"])
		assert.NotEmpty(t, headers["traceparent"], "traceparent should be set from grpc-trace-bin")
	})

	t.Run("http protocol with traceparent and tracestate", func(t *testing.T) {
		tp := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		ts := "vendor1=value1"
		fakeMetadata := DaprInternalMetadata{
			"content-type":  {Values: []string{JSONContentType}},
			"traceparent":   {Values: []string{tp}},
			"tracestate":    {Values: []string{ts}},
			"custom-header": {Values: []string{"value1"}},
		}

		ctx := context.Background()
		headers := map[string]string{}
		InternalMetadataToHTTPHeader(ctx, fakeMetadata, func(k, v string) {
			headers[k] = v
		})

		assert.Equal(t, "value1", headers["custom-header"])
		assert.Equal(t, tp, headers["traceparent"])
		assert.Equal(t, ts, headers["tracestate"])
	})

	t.Run("http protocol with traceparent only", func(t *testing.T) {
		tp := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		fakeMetadata := DaprInternalMetadata{
			"content-type":  {Values: []string{JSONContentType}},
			"traceparent":   {Values: []string{tp}},
			"custom-header": {Values: []string{"value1"}},
		}

		ctx := context.Background()
		headers := map[string]string{}
		InternalMetadataToHTTPHeader(ctx, fakeMetadata, func(k, v string) {
			headers[k] = v
		})

		assert.Equal(t, tp, headers["traceparent"])
		_, hasTracestate := headers["tracestate"]
		assert.False(t, hasTracestate, "tracestate should not be set when not in metadata")
	})
}

func TestInternalMetadataToGrpcMetadataEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("destination-app-id header is skipped", func(t *testing.T) {
		md := DaprInternalMetadata{
			DestinationIDHeader: {Values: []string{"myapp"}},
			"custom-header":     {Values: []string{"value1"}},
		}

		convertedMD := InternalMetadataToGrpcMetadata(ctx, md, false)
		_, hasDest := convertedMD[DestinationIDHeader]
		assert.False(t, hasDest, "destination-app-id should be skipped")
		assert.Equal(t, "value1", convertedMD["custom-header"][0])
	})

	t.Run("invalid base64 in binary metadata is skipped", func(t *testing.T) {
		md := DaprInternalMetadata{
			"data-bin": {Values: []string{"not-valid-base64!!!"}},
		}

		convertedMD := InternalMetadataToGrpcMetadata(ctx, md, false)
		_, hasBin := convertedMD["data-bin"]
		assert.False(t, hasBin, "invalid base64 binary metadata should be skipped")
	})

	t.Run("valid base64 in binary metadata is decoded", func(t *testing.T) {
		original := []byte{100, 200, 50}
		encoded := base64.StdEncoding.EncodeToString(original)
		md := DaprInternalMetadata{
			"data-bin": {Values: []string{encoded}},
		}

		convertedMD := InternalMetadataToGrpcMetadata(ctx, md, false)
		assert.Equal(t, string(original), convertedMD["data-bin"][0])
	})

	t.Run("grpc protocol with empty grpc-trace-bin falls back to context span", func(t *testing.T) {
		md := DaprInternalMetadata{
			"content-type":   {Values: []string{GRPCContentType}},
			"grpc-trace-bin": {Values: []string{""}},
			"custom-header":  {Values: []string{"value1"}},
		}

		convertedMD := InternalMetadataToGrpcMetadata(ctx, md, false)
		vals := convertedMD.Get("grpc-trace-bin")
		assert.Equal(t, 1, len(vals), "grpc-trace-bin should be set from context span fallback")
	})

	t.Run("grpc protocol with valid grpc-trace-bin", func(t *testing.T) {
		sc := trace.SpanContext{
			TraceOptions: 1,
		}
		copy(sc.TraceID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		copy(sc.SpanID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8})
		binSC := propagation.Binary(sc)
		encodedTraceCtx := base64.StdEncoding.EncodeToString(binSC)

		md := DaprInternalMetadata{
			"content-type":   {Values: []string{GRPCContentType}},
			"grpc-trace-bin": {Values: []string{encodedTraceCtx}},
		}

		convertedMD := InternalMetadataToGrpcMetadata(ctx, md, false)
		vals := convertedMD.Get("grpc-trace-bin")
		assert.Equal(t, 1, len(vals))
		assert.Equal(t, string(binSC), vals[0])
	})

	t.Run("grpc protocol with invalid base64 grpc-trace-bin", func(t *testing.T) {
		md := DaprInternalMetadata{
			"content-type":   {Values: []string{GRPCContentType}},
			"grpc-trace-bin": {Values: []string{"not-valid-base64!!!"}},
		}

		convertedMD := InternalMetadataToGrpcMetadata(ctx, md, false)
		// When base64 decode fails, grpc-trace-bin should not be set
		vals := convertedMD.Get("grpc-trace-bin")
		assert.Empty(t, vals, "invalid base64 should not set grpc-trace-bin")
	})

	t.Run("http protocol with traceparent converts to grpc-trace-bin", func(t *testing.T) {
		tp := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		ts := "vendor1=value1"
		md := DaprInternalMetadata{
			"content-type": {Values: []string{JSONContentType}},
			"traceparent":  {Values: []string{tp}},
			"tracestate":   {Values: []string{ts}},
		}

		convertedMD := InternalMetadataToGrpcMetadata(ctx, md, false)
		vals := convertedMD.Get("grpc-trace-bin")
		assert.Equal(t, 1, len(vals), "grpc-trace-bin should be set from HTTP traceparent")
	})

	t.Run("http protocol without traceparent falls back to context span", func(t *testing.T) {
		md := DaprInternalMetadata{
			"content-type":  {Values: []string{JSONContentType}},
			"custom-header": {Values: []string{"value1"}},
		}

		convertedMD := InternalMetadataToGrpcMetadata(ctx, md, false)
		vals := convertedMD.Get("grpc-trace-bin")
		assert.Equal(t, 1, len(vals), "grpc-trace-bin should be set from context span fallback")
	})

	t.Run("multiple values in binary metadata decoded individually", func(t *testing.T) {
		val1 := []byte{10, 20}
		val2 := []byte{30, 40}
		md := DaprInternalMetadata{
			"payload-bin": {Values: []string{
				base64.StdEncoding.EncodeToString(val1),
				base64.StdEncoding.EncodeToString(val2),
			}},
		}

		convertedMD := InternalMetadataToGrpcMetadata(ctx, md, false)
		assert.Equal(t, string(val1), convertedMD["payload-bin"][0])
		assert.Equal(t, string(val2), convertedMD["payload-bin"][1])
	})
}
