// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package diagnostics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestGRPCMetricsInit(t *testing.T) {
	g := newGRPCMetrics()
	assert.False(t, g.IsEnabled())

	err := g.Init("test-grpc-app")
	// Duplicate view registration is expected when other tests run first
	if err != nil {
		assert.Contains(t, err.Error(), "cannot register view")
	}
	assert.True(t, g.IsEnabled())
	assert.Equal(t, "test-grpc-app", g.appID)
}

func TestGRPCMetricsAllMethods(t *testing.T) {
	g := newGRPCMetrics()
	// Manually enable to avoid duplicate view registration errors
	g.appID = "test-grpc-all"
	g.enabled = true

	ctx := context.Background()

	t.Run("ServerRequestReceived", func(t *testing.T) {
		start := g.ServerRequestReceived(ctx, "/dapr.proto.runtime.v1.Dapr/GetState", 128)
		assert.False(t, start.IsZero())
	})

	t.Run("ServerRequestSent", func(t *testing.T) {
		start := time.Now()
		assert.NotPanics(t, func() {
			g.ServerRequestSent(ctx, "/dapr.proto.runtime.v1.Dapr/GetState", "OK", 256, start)
		})
	})

	t.Run("ClientRequestSent", func(t *testing.T) {
		start := g.ClientRequestSent(ctx, "/dapr.proto.runtime.v1.Dapr/SaveState", 64)
		assert.False(t, start.IsZero())
	})

	t.Run("ClientRequestRecieved", func(t *testing.T) {
		start := time.Now()
		assert.NotPanics(t, func() {
			g.ClientRequestRecieved(ctx, "/dapr.proto.runtime.v1.Dapr/SaveState", "OK", 128, start)
		})
	})

	t.Run("getPayloadSize", func(t *testing.T) {
		msg := &emptypb.Empty{}
		size := g.getPayloadSize(msg)
		assert.GreaterOrEqual(t, size, 0)
	})

	t.Run("UnaryServerInterceptor", func(t *testing.T) {
		interceptor := g.UnaryServerInterceptor()
		require.NotNil(t, interceptor)

		fakeReq := &emptypb.Empty{}
		fakeResp := &emptypb.Empty{}
		fakeHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return fakeResp, nil
		}
		info := &grpc.UnaryServerInfo{
			FullMethod: "/dapr.proto.runtime.v1.Dapr/GetState",
		}

		resp, err := interceptor(context.Background(), fakeReq, info, fakeHandler)
		assert.NoError(t, err)
		assert.Equal(t, fakeResp, resp)
	})

	t.Run("UnaryClientInterceptor", func(t *testing.T) {
		interceptor := g.UnaryClientInterceptor()
		require.NotNil(t, interceptor)

		fakeReq := &emptypb.Empty{}
		fakeReply := &emptypb.Empty{}
		fakeInvoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			return nil
		}

		err := interceptor(context.Background(), "/dapr.proto.runtime.v1.Dapr/SaveState", fakeReq, fakeReply, nil, fakeInvoker)
		assert.NoError(t, err)
	})
}

func TestGRPCMetricsDisabledNoPanic(t *testing.T) {
	g := newGRPCMetrics()
	assert.False(t, g.IsEnabled())

	ctx := context.Background()
	start := g.ServerRequestReceived(ctx, "/method", 100)
	assert.False(t, start.IsZero())

	g.ServerRequestSent(ctx, "/method", "OK", 100, start)
	clientStart := g.ClientRequestSent(ctx, "/method", 100)
	assert.False(t, clientStart.IsZero())

	g.ClientRequestRecieved(ctx, "/method", "OK", 100, clientStart)
}
