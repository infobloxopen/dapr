// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package messaging

import (
	"context"
	"errors"
	"testing"

	nr "github.com/dapr/components-contrib/nameresolution"
	"github.com/dapr/dapr/pkg/config"
	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
	"github.com/dapr/dapr/pkg/modes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newDirectMessaging() *directMessaging {
	return &directMessaging{}
}

func TestDestinationHeaders(t *testing.T) {
	t.Run("destination header present", func(t *testing.T) {
		appID := "test1"
		req := invokev1.NewInvokeMethodRequest("GET")
		req.WithMetadata(map[string][]string{})

		dm := newDirectMessaging()
		dm.addDestinationAppIDHeaderToMetadata(appID, req)
		md := req.Metadata()[invokev1.DestinationIDHeader]
		assert.Equal(t, appID, md.Values[0])
	})
}

func TestForwardedHeaders(t *testing.T) {
	t.Run("forwarded headers present", func(t *testing.T) {
		req := invokev1.NewInvokeMethodRequest("GET")
		req.WithMetadata(map[string][]string{})

		dm := newDirectMessaging()
		dm.hostAddress = "1"
		dm.hostName = "2"

		dm.addForwardedHeadersToMetadata(req)

		md := req.Metadata()[fasthttp.HeaderXForwardedFor]
		assert.Equal(t, "1", md.Values[0])

		md = req.Metadata()[fasthttp.HeaderXForwardedHost]
		assert.Equal(t, "2", md.Values[0])

		md = req.Metadata()[fasthttp.HeaderForwarded]
		assert.Equal(t, "for=1;by=1;host=2", md.Values[0])
	})
}

func TestKubernetesNamespace(t *testing.T) {
	t.Run("no namespace", func(t *testing.T) {
		appID := "app1"

		dm := newDirectMessaging()
		id, ns, err := dm.requestAppIDAndNamespace(appID)

		assert.NoError(t, err)
		assert.Empty(t, ns)
		assert.Equal(t, appID, id)
	})

	t.Run("with namespace", func(t *testing.T) {
		appID := "app1.ns1"

		dm := newDirectMessaging()
		id, ns, err := dm.requestAppIDAndNamespace(appID)

		assert.NoError(t, err)
		assert.Equal(t, "ns1", ns)
		assert.Equal(t, "app1", id)
	})

	t.Run("invalid namespace", func(t *testing.T) {
		appID := "app1.ns1.ns2"

		dm := newDirectMessaging()
		_, _, err := dm.requestAppIDAndNamespace(appID)

		assert.Error(t, err)
	})
}

type mockAppChannel struct {
	invokeResponse *invokev1.InvokeMethodResponse
	invokeError    error
}

func (m *mockAppChannel) GetBaseAddress() string {
	return "localhost"
}

func (m *mockAppChannel) InvokeMethod(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
	return m.invokeResponse, m.invokeError
}

type mockResolver struct {
	resolveAddress string
	resolveError   error
}

func (m *mockResolver) Init(metadata nr.Metadata) error {
	return nil
}

func (m *mockResolver) ResolveID(req nr.ResolveRequest) (string, error) {
	return m.resolveAddress, m.resolveError
}

func TestNewDirectMessaging(t *testing.T) {
	t.Run("creates instance", func(t *testing.T) {
		dm := NewDirectMessaging(
			"app1", "ns1", 50001, modes.StandaloneMode,
			nil, nil, nil, config.TracingSpec{}, 4)
		require.NotNil(t, dm)
	})
}

func TestInvokeLocal(t *testing.T) {
	t.Run("calls app channel", func(t *testing.T) {
		resp := invokev1.NewInvokeMethodResponse(200, "OK", nil)
		ch := &mockAppChannel{invokeResponse: resp}

		dm := &directMessaging{appChannel: ch}
		req := invokev1.NewInvokeMethodRequest("test")
		req.WithMetadata(map[string][]string{})

		result, err := dm.invokeLocal(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int32(200), result.Status().GetCode())
	})

	t.Run("nil channel returns error", func(t *testing.T) {
		dm := &directMessaging{appChannel: nil}
		req := invokev1.NewInvokeMethodRequest("test")

		_, err := dm.invokeLocal(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "app channel not initialized")
	})

	t.Run("channel error propagated", func(t *testing.T) {
		ch := &mockAppChannel{invokeError: errors.New("channel error")}
		dm := &directMessaging{appChannel: ch}
		req := invokev1.NewInvokeMethodRequest("test")
		req.WithMetadata(map[string][]string{})

		_, err := dm.invokeLocal(context.Background(), req)
		assert.Error(t, err)
	})
}

func TestGetRemoteApp(t *testing.T) {
	t.Run("resolves app address", func(t *testing.T) {
		resolver := &mockResolver{resolveAddress: "10.0.0.1:50001"}
		dm := &directMessaging{
			resolver:  resolver,
			namespace: "default",
			grpcPort:  50001,
		}

		app, err := dm.getRemoteApp("remote-app")
		require.NoError(t, err)
		assert.Equal(t, "remote-app", app.id)
		assert.Equal(t, "default", app.namespace)
		assert.Equal(t, "10.0.0.1:50001", app.address)
	})

	t.Run("resolver error", func(t *testing.T) {
		resolver := &mockResolver{resolveError: errors.New("resolve failed")}
		dm := &directMessaging{
			resolver:  resolver,
			namespace: "default",
			grpcPort:  50001,
		}

		_, err := dm.getRemoteApp("remote-app")
		assert.Error(t, err)
	})

	t.Run("with namespace in appID", func(t *testing.T) {
		resolver := &mockResolver{resolveAddress: "10.0.0.1:50001"}
		dm := &directMessaging{
			resolver:  resolver,
			namespace: "default",
			grpcPort:  50001,
		}

		app, err := dm.getRemoteApp("remote-app.custom-ns")
		require.NoError(t, err)
		assert.Equal(t, "remote-app", app.id)
		assert.Equal(t, "custom-ns", app.namespace)
	})
}

func TestInvokeWithRetry(t *testing.T) {
	t.Run("succeeds on first try", func(t *testing.T) {
		resp := invokev1.NewInvokeMethodResponse(200, "OK", nil)
		callCount := 0
		fn := func(ctx context.Context, appID, namespace, appAddress string, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
			callCount++
			return resp, nil
		}

		dm := &directMessaging{}
		req := invokev1.NewInvokeMethodRequest("test")
		req.WithMetadata(map[string][]string{})

		app := remoteApp{id: "test", namespace: "default", address: "localhost:50001"}
		result, err := dm.invokeWithRetry(context.Background(), 3, 0, app, fn, req)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, callCount)
	})

	t.Run("returns error after max retries", func(t *testing.T) {
		callCount := 0
		fn := func(ctx context.Context, appID, namespace, appAddress string, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
			callCount++
			return nil, errors.New("non-transient error")
		}

		dm := &directMessaging{}
		req := invokev1.NewInvokeMethodRequest("test")
		req.WithMetadata(map[string][]string{})

		app := remoteApp{id: "test", namespace: "default", address: "localhost:50001"}
		_, err := dm.invokeWithRetry(context.Background(), 1, 0, app, fn, req)
		assert.Error(t, err)
		assert.Equal(t, 1, callCount)
	})
}

func TestInvokeWithRetryTransient(t *testing.T) {
	t.Run("retries on unavailable and reconnects", func(t *testing.T) {
		callCount := 0
		connCreateCount := 0
		fn := func(ctx context.Context, appID, namespace, appAddress string, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
			callCount++
			if callCount == 1 {
				return nil, status.Error(codes.Unavailable, "unavailable")
			}
			return invokev1.NewInvokeMethodResponse(200, "OK", nil), nil
		}

		dm := &directMessaging{
			connectionCreatorFn: func(address, id string, namespace string, skipTLS, recreateIfExists, enableSSL bool) (*grpc.ClientConn, error) {
				connCreateCount++
				return nil, nil
			},
		}
		req := invokev1.NewInvokeMethodRequest("test")
		req.WithMetadata(map[string][]string{})

		app := remoteApp{id: "test", namespace: "default", address: "localhost:50001"}
		result, err := dm.invokeWithRetry(context.Background(), 3, 0, app, fn, req)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 2, callCount)
		assert.Equal(t, 1, connCreateCount)
	})

	t.Run("connection recreation failure", func(t *testing.T) {
		fn := func(ctx context.Context, appID, namespace, appAddress string, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
			return nil, status.Error(codes.Unavailable, "unavailable")
		}

		dm := &directMessaging{
			connectionCreatorFn: func(address, id string, namespace string, skipTLS, recreateIfExists, enableSSL bool) (*grpc.ClientConn, error) {
				return nil, errors.New("connection failed")
			},
		}
		req := invokev1.NewInvokeMethodRequest("test")
		req.WithMetadata(map[string][]string{})

		app := remoteApp{id: "test", namespace: "default", address: "localhost:50001"}
		_, err := dm.invokeWithRetry(context.Background(), 3, 0, app, fn, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection failed")
	})

	t.Run("exhausts all retries on transient error", func(t *testing.T) {
		callCount := 0
		fn := func(ctx context.Context, appID, namespace, appAddress string, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
			callCount++
			return nil, status.Error(codes.Unavailable, "unavailable")
		}

		dm := &directMessaging{
			connectionCreatorFn: func(address, id string, namespace string, skipTLS, recreateIfExists, enableSSL bool) (*grpc.ClientConn, error) {
				return nil, nil
			},
		}
		req := invokev1.NewInvokeMethodRequest("test")
		req.WithMetadata(map[string][]string{})

		app := remoteApp{id: "test", namespace: "default", address: "localhost:50001"}
		_, err := dm.invokeWithRetry(context.Background(), 3, 0, app, fn, req)
		assert.Error(t, err)
		assert.Equal(t, 3, callCount)
	})
}

func TestInvoke(t *testing.T) {
	t.Run("invokes local when target is self", func(t *testing.T) {
		resp := invokev1.NewInvokeMethodResponse(200, "OK", nil)
		ch := &mockAppChannel{invokeResponse: resp}
		resolver := &mockResolver{resolveAddress: "10.0.0.1:50001"}

		dm := &directMessaging{
			appID:      "myapp",
			namespace:  "default",
			appChannel: ch,
			resolver:   resolver,
			grpcPort:   50001,
		}

		req := invokev1.NewInvokeMethodRequest("test")
		req.WithMetadata(map[string][]string{})

		result, err := dm.Invoke(context.Background(), "myapp", req)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("resolver error in Invoke", func(t *testing.T) {
		resolver := &mockResolver{resolveError: errors.New("resolve failed")}
		dm := &directMessaging{
			appID:     "myapp",
			namespace: "default",
			resolver:  resolver,
			grpcPort:  50001,
		}

		req := invokev1.NewInvokeMethodRequest("test")
		req.WithMetadata(map[string][]string{})

		_, err := dm.Invoke(context.Background(), "other-app", req)
		assert.Error(t, err)
	})
}
