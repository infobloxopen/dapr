// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package api

import (
	"context"
	"encoding/json"
	"testing"

	componentsapi "github.com/dapr/dapr/pkg/apis/components/v1alpha1"
	configurationapi "github.com/dapr/dapr/pkg/apis/configuration/v1alpha1"
	subscriptionsapi "github.com/dapr/dapr/pkg/apis/subscriptions/v1alpha1"
	operatorv1pb "github.com/dapr/dapr/pkg/proto/operator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fake_client "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, configurationapi.AddToScheme(s))
	require.NoError(t, componentsapi.AddToScheme(s))
	require.NoError(t, subscriptionsapi.AddToScheme(s))
	return s
}

func newTestServer(cl client.Client) *apiServer {
	return &apiServer{
		Client:     cl,
		updateChan: make(chan *componentsapi.Component, 1),
	}
}

// stubComponentUpdateServer implements operatorv1pb.Operator_ComponentUpdateServer
// for testing the streaming ComponentUpdate method.
type stubComponentUpdateServer struct {
	SentEvents []*operatorv1pb.ComponentUpdateEvent
	ctx        context.Context
}

func (s *stubComponentUpdateServer) Send(event *operatorv1pb.ComponentUpdateEvent) error {
	s.SentEvents = append(s.SentEvents, event)
	return nil
}

func (s *stubComponentUpdateServer) SetHeader(metadata.MD) error  { return nil }
func (s *stubComponentUpdateServer) SendHeader(metadata.MD) error { return nil }
func (s *stubComponentUpdateServer) SetTrailer(metadata.MD)       {}
func (s *stubComponentUpdateServer) Context() context.Context     { return s.ctx }
func (s *stubComponentUpdateServer) SendMsg(interface{}) error    { return nil }
func (s *stubComponentUpdateServer) RecvMsg(interface{}) error    { return nil }

func TestNewAPIServer(t *testing.T) {
	t.Run("returns non-nil server with client set", func(t *testing.T) {
		scheme := newTestScheme(t)
		cl := fake_client.NewClientBuilder().WithScheme(scheme).Build()

		srv := NewAPIServer(cl)
		require.NotNil(t, srv)

		// Verify the underlying type and client field.
		concrete, ok := srv.(*apiServer)
		require.True(t, ok)
		assert.Equal(t, cl, concrete.Client)
		assert.NotNil(t, concrete.updateChan)
	})
}

func TestGetConfiguration(t *testing.T) {
	scheme := newTestScheme(t)

	t.Run("config found returns marshaled JSON", func(t *testing.T) {
		conf := &configurationapi.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-config",
				Namespace: "test-ns",
			},
			Spec: configurationapi.ConfigurationSpec{
				MTLSSpec: configurationapi.MTLSSpec{
					Enabled: true,
				},
			},
		}

		cl := fake_client.NewClientBuilder().WithScheme(scheme).WithObjects(conf).Build()
		srv := newTestServer(cl)

		resp, err := srv.GetConfiguration(context.Background(), &operatorv1pb.GetConfigurationRequest{
			Name:      "my-config",
			Namespace: "test-ns",
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.Configuration)

		// Unmarshal and verify the returned configuration.
		var got configurationapi.Configuration
		require.NoError(t, json.Unmarshal(resp.Configuration, &got))
		assert.Equal(t, "my-config", got.Name)
		assert.Equal(t, "test-ns", got.Namespace)
		assert.True(t, got.Spec.MTLSSpec.Enabled)
	})

	t.Run("config not found returns error", func(t *testing.T) {
		cl := fake_client.NewClientBuilder().WithScheme(scheme).Build()
		srv := newTestServer(cl)

		resp, err := srv.GetConfiguration(context.Background(), &operatorv1pb.GetConfigurationRequest{
			Name:      "nonexistent",
			Namespace: "test-ns",
		})
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "error getting configuration")
	})
}

func TestListComponents(t *testing.T) {
	scheme := newTestScheme(t)

	t.Run("components exist returns list", func(t *testing.T) {
		comp1 := &componentsapi.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "statestore",
				Namespace: "default",
			},
			Spec: componentsapi.ComponentSpec{
				Type:    "state.redis",
				Version: "v1",
			},
		}
		comp2 := &componentsapi.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pubsub",
				Namespace: "default",
			},
			Spec: componentsapi.ComponentSpec{
				Type:    "pubsub.nats",
				Version: "v1",
			},
		}

		cl := fake_client.NewClientBuilder().WithScheme(scheme).WithObjects(comp1, comp2).Build()
		srv := newTestServer(cl)

		resp, err := srv.ListComponents(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Components, 2)

		// Verify one of the marshaled components.
		var got componentsapi.Component
		require.NoError(t, json.Unmarshal(resp.Components[0], &got))
		assert.NotEmpty(t, got.Name)
	})

	t.Run("no components returns empty list", func(t *testing.T) {
		cl := fake_client.NewClientBuilder().WithScheme(scheme).Build()
		srv := newTestServer(cl)

		resp, err := srv.ListComponents(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Empty(t, resp.Components)
	})
}

func TestListSubscriptions(t *testing.T) {
	scheme := newTestScheme(t)

	t.Run("subscriptions exist returns list", func(t *testing.T) {
		sub1 := &subscriptionsapi.Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub1",
				Namespace: "default",
			},
			Spec: subscriptionsapi.SubscriptionSpec{
				Topic:      "orders",
				Route:      "/orders",
				Pubsubname: "pubsub",
			},
		}
		sub2 := &subscriptionsapi.Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub2",
				Namespace: "default",
			},
			Spec: subscriptionsapi.SubscriptionSpec{
				Topic:      "events",
				Route:      "/events",
				Pubsubname: "pubsub",
			},
		}

		cl := fake_client.NewClientBuilder().WithScheme(scheme).WithObjects(sub1, sub2).Build()
		srv := newTestServer(cl)

		resp, err := srv.ListSubscriptions(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Subscriptions, 2)

		// Verify one of the marshaled subscriptions.
		var got subscriptionsapi.Subscription
		require.NoError(t, json.Unmarshal(resp.Subscriptions[0], &got))
		assert.NotEmpty(t, got.Name)
	})

	t.Run("no subscriptions returns empty list", func(t *testing.T) {
		cl := fake_client.NewClientBuilder().WithScheme(scheme).Build()
		srv := newTestServer(cl)

		resp, err := srv.ListSubscriptions(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Empty(t, resp.Subscriptions)
	})
}

func TestComponentUpdate(t *testing.T) {
	scheme := newTestScheme(t)

	t.Run("receives component update and sends event", func(t *testing.T) {
		cl := fake_client.NewClientBuilder().WithScheme(scheme).Build()
		srv := newTestServer(cl)

		stub := &stubComponentUpdateServer{
			ctx: context.Background(),
		}

		comp := &componentsapi.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "statestore",
				Namespace: "default",
			},
			Spec: componentsapi.ComponentSpec{
				Type:    "state.redis",
				Version: "v1",
			},
		}

		// Send a component and then close the channel to end the loop.
		srv.updateChan <- comp
		close(srv.updateChan)

		err := srv.ComponentUpdate(&emptypb.Empty{}, stub)
		require.NoError(t, err)

		// The Send is dispatched in a goroutine; give it a moment to complete.
		// Since the channel is closed after one item, the for-range exits and
		// the goroutine should complete quickly.
		require.Eventually(t, func() bool {
			return len(stub.SentEvents) == 1
		}, 2*1e9, 10*1e6) // 2s timeout, 10ms poll

		var got componentsapi.Component
		require.NoError(t, json.Unmarshal(stub.SentEvents[0].Component, &got))
		assert.Equal(t, "statestore", got.Name)
		assert.Equal(t, "state.redis", got.Spec.Type)
	})

	t.Run("closed channel with no items returns immediately", func(t *testing.T) {
		cl := fake_client.NewClientBuilder().WithScheme(scheme).Build()
		srv := newTestServer(cl)

		stub := &stubComponentUpdateServer{
			ctx: context.Background(),
		}

		close(srv.updateChan)

		err := srv.ComponentUpdate(&emptypb.Empty{}, stub)
		require.NoError(t, err)
		assert.Empty(t, stub.SentEvents)
	})
}

func TestOnComponentUpdated(t *testing.T) {
	t.Run("is a no-op", func(t *testing.T) {
		scheme := newTestScheme(t)
		cl := fake_client.NewClientBuilder().WithScheme(scheme).Build()
		srv := newTestServer(cl)

		// Should not panic or error — currently a no-op (TODO in source).
		comp := &componentsapi.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "statestore",
				Namespace: "default",
			},
		}
		srv.OnComponentUpdated(comp)
	})
}
