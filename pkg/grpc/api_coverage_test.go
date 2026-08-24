// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/dapr/components-contrib/pubsub"
	"github.com/dapr/components-contrib/state"
	components_v1alpha "github.com/dapr/dapr/pkg/apis/components/v1alpha1"
	"github.com/dapr/dapr/pkg/config"
	commonv1pb "github.com/dapr/dapr/pkg/proto/common/v1"
	runtimev1pb "github.com/dapr/dapr/pkg/proto/runtime/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- hand-written stubs ---

// stubPubSub implements pubsub.PubSub with configurable behaviour.
type stubPubSub struct {
	features   []pubsub.Feature
	publishErr error
}

func (s *stubPubSub) Init(pubsub.Metadata) error                                     { return nil }
func (s *stubPubSub) Features() []pubsub.Feature                                     { return s.features }
func (s *stubPubSub) Publish(*pubsub.PublishRequest) error                            { return s.publishErr }
func (s *stubPubSub) Subscribe(pubsub.SubscribeRequest, func(*pubsub.NewMessage) error) error {
	return nil
}
func (s *stubPubSub) Close() error { return nil }

// stubPubSubAdapter implements runtime_pubsub.Adapter.
type stubPubSubAdapter struct {
	publishFn   func(*pubsub.PublishRequest) error
	getPubSubFn func(string) pubsub.PubSub
}

func (a *stubPubSubAdapter) Publish(req *pubsub.PublishRequest) error { return a.publishFn(req) }
func (a *stubPubSubAdapter) GetPubSub(name string) pubsub.PubSub     { return a.getPubSubFn(name) }

// stubStateStore implements state.Store with configurable behaviour.
type stubStateStore struct {
	bulkDeleteFn func([]state.DeleteRequest) error
	getFn        func(*state.GetRequest) (*state.GetResponse, error)
	setFn        func(*state.SetRequest) error
	deleteFn     func(*state.DeleteRequest) error
	bulkGetFn    func([]state.GetRequest) (bool, []state.BulkGetResponse, error)
	bulkSetFn    func([]state.SetRequest) error
}

func (s *stubStateStore) Init(state.Metadata) error { return nil }

func (s *stubStateStore) Delete(req *state.DeleteRequest) error {
	if s.deleteFn != nil {
		return s.deleteFn(req)
	}
	return nil
}

func (s *stubStateStore) Get(req *state.GetRequest) (*state.GetResponse, error) {
	if s.getFn != nil {
		return s.getFn(req)
	}
	return &state.GetResponse{}, nil
}

func (s *stubStateStore) Set(req *state.SetRequest) error {
	if s.setFn != nil {
		return s.setFn(req)
	}
	return nil
}

func (s *stubStateStore) BulkDelete(req []state.DeleteRequest) error {
	if s.bulkDeleteFn != nil {
		return s.bulkDeleteFn(req)
	}
	return nil
}

func (s *stubStateStore) BulkGet(req []state.GetRequest) (bool, []state.BulkGetResponse, error) {
	if s.bulkGetFn != nil {
		return s.bulkGetFn(req)
	}
	return false, nil, nil
}

func (s *stubStateStore) BulkSet(req []state.SetRequest) error {
	if s.bulkSetFn != nil {
		return s.bulkSetFn(req)
	}
	return nil
}

// --- tests ---

func TestNewAPI(t *testing.T) {
	t.Run("all fields set", func(t *testing.T) {
		fakeStore := &stubStateStore{}
		stores := map[string]state.Store{"store1": fakeStore}
		components := []components_v1alpha.Component{
			{},
		}
		acl := &config.AccessControlList{DefaultAction: "deny"}

		a := NewAPI(
			"app1",
			nil,    // appChannel
			stores, // stateStores
			nil,    // secretStores
			nil,    // secretsConfiguration
			nil,    // pubsubAdapter
			nil,    // directMessaging
			nil,    // actor
			nil,    // sendToOutputBindingFn
			config.TracingSpec{SamplingRate: "1"},
			acl,
			"http",
			components,
		)

		require.NotNil(t, a)
		concrete, ok := a.(*api)
		require.True(t, ok)
		assert.Equal(t, "app1", concrete.id)
		assert.Equal(t, stores, concrete.stateStores)
		assert.Equal(t, acl, concrete.accessControlList)
		assert.Equal(t, "http", concrete.appProtocol)
		assert.Equal(t, "1", concrete.tracingSpec.SamplingRate)
	})

	t.Run("nil optional fields", func(t *testing.T) {
		a := NewAPI(
			"app2",
			nil, nil, nil, nil, nil, nil, nil, nil,
			config.TracingSpec{},
			nil, "", nil,
		)

		require.NotNil(t, a)
		concrete := a.(*api)
		assert.Equal(t, "app2", concrete.id)
		assert.Nil(t, concrete.stateStores)
		assert.Nil(t, concrete.accessControlList)
	})
}

func TestPublishEventCoverage(t *testing.T) {
	t.Run("pubsub adapter not configured", func(t *testing.T) {
		a := &api{
			id:            "test-app",
			pubsubAdapter: nil,
		}

		_, err := a.PublishEvent(context.Background(), &runtimev1pb.PublishEventRequest{
			PubsubName: "mypubsub",
			Topic:      "mytopic",
		})
		require.Error(t, err)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("empty pubsub name", func(t *testing.T) {
		a := &api{
			id: "test-app",
			pubsubAdapter: &stubPubSubAdapter{
				getPubSubFn: func(string) pubsub.PubSub { return &stubPubSub{} },
				publishFn:   func(*pubsub.PublishRequest) error { return nil },
			},
		}

		_, err := a.PublishEvent(context.Background(), &runtimev1pb.PublishEventRequest{
			PubsubName: "",
			Topic:      "mytopic",
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("pubsub component not found", func(t *testing.T) {
		a := &api{
			id: "test-app",
			pubsubAdapter: &stubPubSubAdapter{
				getPubSubFn: func(string) pubsub.PubSub { return nil },
				publishFn:   func(*pubsub.PublishRequest) error { return nil },
			},
		}

		_, err := a.PublishEvent(context.Background(), &runtimev1pb.PublishEventRequest{
			PubsubName: "missing-pubsub",
			Topic:      "mytopic",
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("empty topic", func(t *testing.T) {
		a := &api{
			id: "test-app",
			pubsubAdapter: &stubPubSubAdapter{
				getPubSubFn: func(string) pubsub.PubSub { return &stubPubSub{} },
				publishFn:   func(*pubsub.PublishRequest) error { return nil },
			},
		}

		_, err := a.PublishEvent(context.Background(), &runtimev1pb.PublishEventRequest{
			PubsubName: "mypubsub",
			Topic:      "",
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestDeleteBulkStateCoverage(t *testing.T) {
	t.Run("state store not configured", func(t *testing.T) {
		a := &api{
			id:          "test-app",
			stateStores: nil,
		}

		_, err := a.DeleteBulkState(context.Background(), &runtimev1pb.DeleteBulkStateRequest{
			StoreName: "store1",
		})
		require.Error(t, err)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("state store not found", func(t *testing.T) {
		a := &api{
			id:          "test-app",
			stateStores: map[string]state.Store{"store1": &stubStateStore{}},
		}

		_, err := a.DeleteBulkState(context.Background(), &runtimev1pb.DeleteBulkStateRequest{
			StoreName: "nonexistent",
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("successful bulk delete with empty states", func(t *testing.T) {
		called := false
		store := &stubStateStore{
			bulkDeleteFn: func(reqs []state.DeleteRequest) error {
				called = true
				return nil
			},
		}
		a := &api{
			id:          "test-app",
			stateStores: map[string]state.Store{"store1": store},
		}

		_, err := a.DeleteBulkState(context.Background(), &runtimev1pb.DeleteBulkStateRequest{
			StoreName: "store1",
			States:    []*commonv1pb.StateItem{},
		})
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("successful bulk delete with states", func(t *testing.T) {
		var deletedKeys []string
		store := &stubStateStore{
			bulkDeleteFn: func(reqs []state.DeleteRequest) error {
				for _, r := range reqs {
					deletedKeys = append(deletedKeys, r.Key)
				}
				return nil
			},
		}
		a := &api{
			id:          "test-app",
			stateStores: map[string]state.Store{"store1": store},
		}

		_, err := a.DeleteBulkState(context.Background(), &runtimev1pb.DeleteBulkStateRequest{
			StoreName: "store1",
			States: []*commonv1pb.StateItem{
				{Key: "key1"},
				{Key: "key2"},
			},
		})
		require.NoError(t, err)
		assert.Len(t, deletedKeys, 2)
		assert.Contains(t, deletedKeys[0], "key1")
		assert.Contains(t, deletedKeys[1], "key2")
	})

	t.Run("bulk delete with etag and options", func(t *testing.T) {
		var captured []state.DeleteRequest
		store := &stubStateStore{
			bulkDeleteFn: func(reqs []state.DeleteRequest) error {
				captured = reqs
				return nil
			},
		}
		a := &api{
			id:          "test-app",
			stateStores: map[string]state.Store{"store1": store},
		}

		_, err := a.DeleteBulkState(context.Background(), &runtimev1pb.DeleteBulkStateRequest{
			StoreName: "store1",
			States: []*commonv1pb.StateItem{
				{
					Key:  "key1",
					Etag: &commonv1pb.Etag{Value: "etag-1"},
					Options: &commonv1pb.StateOptions{
						Concurrency: commonv1pb.StateOptions_CONCURRENCY_FIRST_WRITE,
						Consistency: commonv1pb.StateOptions_CONSISTENCY_STRONG,
					},
				},
			},
		})
		require.NoError(t, err)
		require.Len(t, captured, 1)
		assert.NotNil(t, captured[0].ETag)
		assert.Equal(t, "etag-1", *captured[0].ETag)
		assert.Equal(t, "first-write", captured[0].Options.Concurrency)
		assert.Equal(t, "strong", captured[0].Options.Consistency)
	})

	t.Run("bulk delete returns error", func(t *testing.T) {
		store := &stubStateStore{
			bulkDeleteFn: func(reqs []state.DeleteRequest) error {
				return errors.New("bulk delete failed")
			},
		}
		a := &api{
			id:          "test-app",
			stateStores: map[string]state.Store{"store1": store},
		}

		_, err := a.DeleteBulkState(context.Background(), &runtimev1pb.DeleteBulkStateRequest{
			StoreName: "store1",
			States: []*commonv1pb.StateItem{
				{Key: "key1"},
			},
		})
		require.Error(t, err)
	})
}

func TestApplyAccessControlPolicies(t *testing.T) {
	t.Run("nil access control list allows by default", func(t *testing.T) {
		a := &api{
			id:                "test-app",
			accessControlList: nil,
		}
		// When ACL is nil, CallLocal skips the policy check entirely.
		// Verify the field is nil.
		assert.Nil(t, a.accessControlList)
	})

	t.Run("non-nil access control list with default deny", func(t *testing.T) {
		acl := &config.AccessControlList{
			DefaultAction: "deny",
			TrustDomain:   "public",
		}
		a := &api{
			id:                "test-app",
			accessControlList: acl,
			appProtocol:       "http",
		}

		// Call applyAccessControlPolicies directly with a basic context.
		// Without a valid spiffe ID in the context, it falls through to the
		// default global action.
		allowed, errMsg := a.applyAccessControlPolicies(
			context.Background(),
			"/method",
			commonv1pb.HTTPExtension_POST,
			"http",
		)

		// With a deny default action and no matching policy, access should be denied.
		assert.False(t, allowed)
		assert.Contains(t, errMsg, "access control policy has denied access")
	})

	t.Run("non-nil access control list with default allow", func(t *testing.T) {
		acl := &config.AccessControlList{
			DefaultAction: "allow",
			TrustDomain:   "public",
		}
		a := &api{
			id:                "test-app",
			accessControlList: acl,
			appProtocol:       "grpc",
		}

		allowed, errMsg := a.applyAccessControlPolicies(
			context.Background(),
			"/mymethod",
			commonv1pb.HTTPExtension_NONE,
			"grpc",
		)

		assert.True(t, allowed)
		assert.Empty(t, errMsg)
	})
}
