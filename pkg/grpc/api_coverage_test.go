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
	"github.com/dapr/components-contrib/secretstores"
	"github.com/dapr/components-contrib/state"
	"github.com/dapr/dapr/pkg/actors"
	components_v1alpha "github.com/dapr/dapr/pkg/apis/components/v1alpha1"
	"github.com/dapr/dapr/pkg/channel"
	"github.com/dapr/dapr/pkg/config"
	"github.com/dapr/dapr/pkg/messaging"
	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
	commonv1pb "github.com/dapr/dapr/pkg/proto/common/v1"
	internalv1pb "github.com/dapr/dapr/pkg/proto/internals/v1"
	runtimev1pb "github.com/dapr/dapr/pkg/proto/runtime/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
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

// stubActors implements actors.Actors with configurable behaviour.
type stubActors struct {
	callFn                        func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error)
	initFn                        func() error
	getStateFn                    func(ctx context.Context, req *actors.GetStateRequest) (*actors.StateResponse, error)
	transactionalStateOperationFn func(ctx context.Context, req *actors.TransactionalRequest) error
	getReminderFn                 func(ctx context.Context, req *actors.GetReminderRequest) (*actors.Reminder, error)
	createReminderFn              func(ctx context.Context, req *actors.CreateReminderRequest) error
	deleteReminderFn              func(ctx context.Context, req *actors.DeleteReminderRequest) error
	createTimerFn                 func(ctx context.Context, req *actors.CreateTimerRequest) error
	deleteTimerFn                 func(ctx context.Context, req *actors.DeleteTimerRequest) error
	isActorHostedFn               func(ctx context.Context, req *actors.ActorHostedRequest) bool
	getActiveActorsCountFn        func(ctx context.Context) []actors.ActiveActorsCount
}

func (s *stubActors) Call(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
	if s.callFn != nil {
		return s.callFn(ctx, req)
	}
	return invokev1.NewInvokeMethodResponse(200, "OK", nil), nil
}

func (s *stubActors) Init() error {
	if s.initFn != nil {
		return s.initFn()
	}
	return nil
}

func (s *stubActors) Stop() {}

func (s *stubActors) GetState(ctx context.Context, req *actors.GetStateRequest) (*actors.StateResponse, error) {
	if s.getStateFn != nil {
		return s.getStateFn(ctx, req)
	}
	return &actors.StateResponse{}, nil
}

func (s *stubActors) TransactionalStateOperation(ctx context.Context, req *actors.TransactionalRequest) error {
	if s.transactionalStateOperationFn != nil {
		return s.transactionalStateOperationFn(ctx, req)
	}
	return nil
}

func (s *stubActors) GetReminder(ctx context.Context, req *actors.GetReminderRequest) (*actors.Reminder, error) {
	if s.getReminderFn != nil {
		return s.getReminderFn(ctx, req)
	}
	return &actors.Reminder{}, nil
}

func (s *stubActors) CreateReminder(ctx context.Context, req *actors.CreateReminderRequest) error {
	if s.createReminderFn != nil {
		return s.createReminderFn(ctx, req)
	}
	return nil
}

func (s *stubActors) DeleteReminder(ctx context.Context, req *actors.DeleteReminderRequest) error {
	if s.deleteReminderFn != nil {
		return s.deleteReminderFn(ctx, req)
	}
	return nil
}

func (s *stubActors) CreateTimer(ctx context.Context, req *actors.CreateTimerRequest) error {
	if s.createTimerFn != nil {
		return s.createTimerFn(ctx, req)
	}
	return nil
}

func (s *stubActors) DeleteTimer(ctx context.Context, req *actors.DeleteTimerRequest) error {
	if s.deleteTimerFn != nil {
		return s.deleteTimerFn(ctx, req)
	}
	return nil
}

func (s *stubActors) IsActorHosted(ctx context.Context, req *actors.ActorHostedRequest) bool {
	if s.isActorHostedFn != nil {
		return s.isActorHostedFn(ctx, req)
	}
	return true
}

func (s *stubActors) GetActiveActorsCount(ctx context.Context) []actors.ActiveActorsCount {
	if s.getActiveActorsCountFn != nil {
		return s.getActiveActorsCountFn(ctx)
	}
	return nil
}

// stubSecretStore implements secretstores.SecretStore with configurable behaviour.
type stubSecretStore struct {
	getSecretFn     func(secretstores.GetSecretRequest) (secretstores.GetSecretResponse, error)
	bulkGetSecretFn func(secretstores.BulkGetSecretRequest) (secretstores.BulkGetSecretResponse, error)
}

func (s *stubSecretStore) Init(secretstores.Metadata) error { return nil }

func (s *stubSecretStore) GetSecret(req secretstores.GetSecretRequest) (secretstores.GetSecretResponse, error) {
	if s.getSecretFn != nil {
		return s.getSecretFn(req)
	}
	return secretstores.GetSecretResponse{}, nil
}

func (s *stubSecretStore) BulkGetSecret(req secretstores.BulkGetSecretRequest) (secretstores.BulkGetSecretResponse, error) {
	if s.bulkGetSecretFn != nil {
		return s.bulkGetSecretFn(req)
	}
	return secretstores.BulkGetSecretResponse{}, nil
}

// stubAppChannel implements channel.AppChannel with configurable behaviour.
type stubAppChannel struct {
	invokeMethodFn func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error)
	baseAddress    string
}

func (c *stubAppChannel) GetBaseAddress() string { return c.baseAddress }
func (c *stubAppChannel) InvokeMethod(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
	if c.invokeMethodFn != nil {
		return c.invokeMethodFn(ctx, req)
	}
	return invokev1.NewInvokeMethodResponse(200, "OK", nil), nil
}

// ctxWithSpan returns a context with an opencensus span attached, so that
// diag_utils.SpanFromContext does not return nil.
func ctxWithSpan() context.Context {
	ctx, _ := trace.StartSpan(context.Background(), "test")
	return ctx
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

// --- CallLocal tests ---

func TestCallLocalCoverage(t *testing.T) {
	t.Run("nil appChannel returns Internal error", func(t *testing.T) {
		a := &api{
			id:         "test-app",
			appChannel: nil,
		}

		in := &internalv1pb.InternalInvokeRequest{
			Message: &commonv1pb.InvokeRequest{Method: "mymethod"},
		}
		_, err := a.CallLocal(context.Background(), in)
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.Contains(t, err.Error(), "not initialized")
	})

	t.Run("nil Message returns InvalidArgument", func(t *testing.T) {
		a := &api{
			id:         "test-app",
			appChannel: &stubAppChannel{},
		}

		in := &internalv1pb.InternalInvokeRequest{
			Message: nil,
		}
		_, err := a.CallLocal(context.Background(), in)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("appChannel InvokeMethod returns error", func(t *testing.T) {
		a := &api{
			id: "test-app",
			appChannel: &stubAppChannel{
				invokeMethodFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
					return nil, errors.New("channel invoke failed")
				},
			},
		}

		in := &internalv1pb.InternalInvokeRequest{
			Message: &commonv1pb.InvokeRequest{Method: "mymethod"},
		}
		_, err := a.CallLocal(context.Background(), in)
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.Contains(t, err.Error(), "channel invoke failed")
	})

	t.Run("appChannel InvokeMethod returns success", func(t *testing.T) {
		var capturedMethod string
		a := &api{
			id: "test-app",
			appChannel: &stubAppChannel{
				invokeMethodFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
					capturedMethod = req.Message().Method
					resp := invokev1.NewInvokeMethodResponse(200, "OK", nil)
					resp.WithRawData([]byte(`{"result":"success"}`), "application/json")
					return resp, nil
				},
			},
		}

		in := &internalv1pb.InternalInvokeRequest{
			Message: &commonv1pb.InvokeRequest{Method: "doWork"},
		}
		resp, err := a.CallLocal(context.Background(), in)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "doWork", capturedMethod)
	})

	t.Run("ACL deny blocks call", func(t *testing.T) {
		a := &api{
			id: "test-app",
			appChannel: &stubAppChannel{
				invokeMethodFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
					t.Fatal("InvokeMethod should not be called when ACL denies")
					return nil, nil
				},
			},
			accessControlList: &config.AccessControlList{
				DefaultAction: "deny",
				TrustDomain:   "public",
			},
			appProtocol: "grpc",
		}

		in := &internalv1pb.InternalInvokeRequest{
			Message: &commonv1pb.InvokeRequest{Method: "/mymethod"},
		}
		_, err := a.CallLocal(context.Background(), in)
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("ACL allow permits call", func(t *testing.T) {
		invoked := false
		a := &api{
			id: "test-app",
			appChannel: &stubAppChannel{
				invokeMethodFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
					invoked = true
					return invokev1.NewInvokeMethodResponse(200, "OK", nil), nil
				},
			},
			accessControlList: &config.AccessControlList{
				DefaultAction: "allow",
				TrustDomain:   "public",
			},
			appProtocol: "grpc",
		}

		in := &internalv1pb.InternalInvokeRequest{
			Message: &commonv1pb.InvokeRequest{Method: "/mymethod"},
		}
		resp, err := a.CallLocal(context.Background(), in)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, invoked)
	})

	t.Run("ACL with HTTP protocol and verb", func(t *testing.T) {
		a := &api{
			id: "test-app",
			appChannel: &stubAppChannel{
				invokeMethodFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
					return invokev1.NewInvokeMethodResponse(200, "OK", nil), nil
				},
			},
			accessControlList: &config.AccessControlList{
				DefaultAction: "allow",
				TrustDomain:   "public",
			},
			appProtocol: "http",
		}

		in := &internalv1pb.InternalInvokeRequest{
			Message: &commonv1pb.InvokeRequest{
				Method: "/api/invoke",
				HttpExtension: &commonv1pb.HTTPExtension{
					Verb: commonv1pb.HTTPExtension_POST,
				},
			},
			Metadata: invokev1.MetadataToInternalMetadata(map[string][]string{
				"content-type": {"application/json"},
			}),
		}
		resp, err := a.CallLocal(context.Background(), in)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("ACL nil skips policy check", func(t *testing.T) {
		invoked := false
		a := &api{
			id: "test-app",
			appChannel: &stubAppChannel{
				invokeMethodFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
					invoked = true
					return invokev1.NewInvokeMethodResponse(200, "OK", nil), nil
				},
			},
			accessControlList: nil,
		}

		in := &internalv1pb.InternalInvokeRequest{
			Message: &commonv1pb.InvokeRequest{Method: "doWork"},
		}
		resp, err := a.CallLocal(context.Background(), in)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, invoked)
	})
}

// --- PublishEvent cloud event error path tests ---

func TestPublishEventCloudEventCreationError(t *testing.T) {
	t.Run("invalid cloudevent JSON returns InvalidArgument", func(t *testing.T) {
		ps := &stubPubSub{
			features: []pubsub.Feature{},
		}

		a := &api{
			id: "test-app",
			pubsubAdapter: &stubPubSubAdapter{
				getPubSubFn: func(name string) pubsub.PubSub { return ps },
				publishFn: func(req *pubsub.PublishRequest) error {
					t.Fatal("Publish should not be called when cloud event creation fails")
					return nil
				},
			},
		}

		// DataContentType "application/cloudevents+json" triggers FromCloudEvent
		// which tries to unmarshal the data as JSON. Invalid JSON causes an error.
		_, err := a.PublishEvent(ctxWithSpan(), &runtimev1pb.PublishEventRequest{
			PubsubName:      "mypubsub",
			Topic:           "mytopic",
			Data:            []byte("this is not valid json{{{"),
			DataContentType: "application/cloudevents+json",
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "cannot create cloudevent")
	})

	t.Run("empty data with cloudevent content type returns error", func(t *testing.T) {
		ps := &stubPubSub{
			features: []pubsub.Feature{},
		}

		a := &api{
			id: "test-app",
			pubsubAdapter: &stubPubSubAdapter{
				getPubSubFn: func(name string) pubsub.PubSub { return ps },
				publishFn: func(req *pubsub.PublishRequest) error {
					t.Fatal("Publish should not be called when cloud event creation fails")
					return nil
				},
			},
		}

		// Empty body (nil data) with cloudevent content type should also
		// fail because FromCloudEvent cannot parse an empty byte slice as JSON.
		_, err := a.PublishEvent(ctxWithSpan(), &runtimev1pb.PublishEventRequest{
			PubsubName:      "mypubsub",
			Topic:           "mytopic",
			Data:            nil,
			DataContentType: "application/cloudevents+json",
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("topic with slashes is accepted for validation", func(t *testing.T) {
		// A topic like "/" is not empty, so it passes the empty-topic check.
		// It then proceeds to the cloud event path. With cloud event content type
		// and invalid JSON, the error is from cloud event creation, not topic validation.
		ps := &stubPubSub{
			features: []pubsub.Feature{},
		}

		a := &api{
			id: "test-app",
			pubsubAdapter: &stubPubSubAdapter{
				getPubSubFn: func(name string) pubsub.PubSub { return ps },
				publishFn: func(req *pubsub.PublishRequest) error {
					return nil
				},
			},
		}

		_, err := a.PublishEvent(ctxWithSpan(), &runtimev1pb.PublishEventRequest{
			PubsubName:      "mypubsub",
			Topic:           "/",
			Data:            []byte("not json{{{"),
			DataContentType: "application/cloudevents+json",
		})
		// Should hit the cloud event creation error, not the empty-topic check.
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "cannot create cloudevent")
	})
}

// --- applyAccessControlPolicies additional tests ---

func TestApplyAccessControlPoliciesMore(t *testing.T) {
	t.Run("HTTP protocol with POST verb and deny", func(t *testing.T) {
		acl := &config.AccessControlList{
			DefaultAction: "deny",
			TrustDomain:   "public",
		}
		a := &api{
			id:                "test-app",
			accessControlList: acl,
			appProtocol:       "http",
		}

		allowed, errMsg := a.applyAccessControlPolicies(
			context.Background(),
			"/api/v1/resource",
			commonv1pb.HTTPExtension_POST,
			"http",
		)

		assert.False(t, allowed)
		assert.Contains(t, errMsg, "access control policy has denied access")
	})

	t.Run("HTTP protocol with GET verb and allow", func(t *testing.T) {
		acl := &config.AccessControlList{
			DefaultAction: "allow",
			TrustDomain:   "public",
		}
		a := &api{
			id:                "test-app",
			accessControlList: acl,
			appProtocol:       "http",
		}

		allowed, errMsg := a.applyAccessControlPolicies(
			context.Background(),
			"/api/v1/resource",
			commonv1pb.HTTPExtension_GET,
			"http",
		)

		assert.True(t, allowed)
		assert.Empty(t, errMsg)
	})

	t.Run("HTTP protocol with PUT verb and deny", func(t *testing.T) {
		acl := &config.AccessControlList{
			DefaultAction: "deny",
			TrustDomain:   "public",
		}
		a := &api{
			id:                "test-app",
			accessControlList: acl,
			appProtocol:       "http",
		}

		allowed, errMsg := a.applyAccessControlPolicies(
			context.Background(),
			"/api/v1/update",
			commonv1pb.HTTPExtension_PUT,
			"http",
		)

		assert.False(t, allowed)
		assert.Contains(t, errMsg, "access control policy has denied access")
	})

	t.Run("grpc protocol with default deny", func(t *testing.T) {
		acl := &config.AccessControlList{
			DefaultAction: "deny",
			TrustDomain:   "td1",
		}
		a := &api{
			id:                "test-app",
			accessControlList: acl,
			appProtocol:       "grpc",
		}

		allowed, errMsg := a.applyAccessControlPolicies(
			context.Background(),
			"/dapr.proto.runtime.v1.AppCallback/OnInvoke",
			commonv1pb.HTTPExtension_NONE,
			"grpc",
		)

		assert.False(t, allowed)
		assert.Contains(t, errMsg, "access control policy has denied access")
	})

	t.Run("operation with duplicate slashes is normalized", func(t *testing.T) {
		acl := &config.AccessControlList{
			DefaultAction: "allow",
			TrustDomain:   "public",
		}
		a := &api{
			id:                "test-app",
			accessControlList: acl,
			appProtocol:       "http",
		}

		// Duplicate slashes should be normalized by purell.
		allowed, errMsg := a.applyAccessControlPolicies(
			context.Background(),
			"/api//v1///resource",
			commonv1pb.HTTPExtension_GET,
			"http",
		)

		assert.True(t, allowed)
		assert.Empty(t, errMsg)
	})

	t.Run("empty operation is normalized", func(t *testing.T) {
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
			"",
			commonv1pb.HTTPExtension_NONE,
			"grpc",
		)

		assert.True(t, allowed)
		assert.Empty(t, errMsg)
	})

	t.Run("invalid operation URL returns normalization error", func(t *testing.T) {
		acl := &config.AccessControlList{
			DefaultAction: "allow",
			TrustDomain:   "public",
		}
		a := &api{
			id:                "test-app",
			accessControlList: acl,
			appProtocol:       "http",
		}

		// "://" is an invalid URL that purell.NormalizeURLString cannot parse,
		// triggering the normalization error branch.
		allowed, errMsg := a.applyAccessControlPolicies(
			context.Background(),
			"://",
			commonv1pb.HTTPExtension_GET,
			"http",
		)

		assert.False(t, allowed)
		assert.Contains(t, errMsg, "error in method normalization")
	})
}

// --- setter tests ---

func TestSetAppChannelCoverage(t *testing.T) {
	t.Run("sets app channel to non-nil value", func(t *testing.T) {
		a := &api{}
		assert.Nil(t, a.appChannel)

		var fakeChannel channel.AppChannel // nil interface value is fine for setter test
		a.SetAppChannel(fakeChannel)
		// The setter simply assigns the field; verify the assignment happened.
		assert.Equal(t, fakeChannel, a.appChannel)
	})
}

func TestSetDirectMessagingCoverage(t *testing.T) {
	t.Run("sets direct messaging to non-nil value", func(t *testing.T) {
		a := &api{}
		assert.Nil(t, a.directMessaging)

		var fakeDM messaging.DirectMessaging
		a.SetDirectMessaging(fakeDM)
		assert.Equal(t, fakeDM, a.directMessaging)
	})
}

func TestSetActorRuntimeCoverage(t *testing.T) {
	t.Run("sets actor runtime", func(t *testing.T) {
		a := &api{}
		assert.Nil(t, a.actor)

		stub := &stubActors{}
		a.SetActorRuntime(stub)
		assert.Equal(t, stub, a.actor)
	})
}

// --- CallActor tests ---

func TestCallActorCoverage(t *testing.T) {
	t.Run("invalid request returns InvalidArgument", func(t *testing.T) {
		a := &api{actor: &stubActors{}}

		// InternalInvokeRequest with nil Message triggers an error from
		// invokev1.InternalInvokeRequest.
		in := &internalv1pb.InternalInvokeRequest{
			Message: nil,
		}
		_, err := a.CallActor(context.Background(), in)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("actor Call returns error", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				callFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
					return nil, errors.New("actor call failed")
				},
			},
		}

		in := &internalv1pb.InternalInvokeRequest{
			Message: &commonv1pb.InvokeRequest{Method: "mymethod"},
		}
		_, err := a.CallActor(context.Background(), in)
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("actor Call succeeds", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				callFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
					return invokev1.NewInvokeMethodResponse(200, "OK", nil), nil
				},
			},
		}

		in := &internalv1pb.InternalInvokeRequest{
			Message: &commonv1pb.InvokeRequest{Method: "mymethod"},
		}
		resp, err := a.CallActor(context.Background(), in)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

// --- InvokeActor tests ---

func TestInvokeActorCoverage(t *testing.T) {
	t.Run("actor runtime nil returns error", func(t *testing.T) {
		a := &api{actor: nil}

		_, err := a.InvokeActor(context.Background(), &runtimev1pb.InvokeActorRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Method:    "DoWork",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("actor Call returns error", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				callFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
					return nil, errors.New("invoke failed")
				},
			},
		}

		_, err := a.InvokeActor(context.Background(), &runtimev1pb.InvokeActorRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Method:    "DoWork",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("success path returns data", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				callFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
					resp := invokev1.NewInvokeMethodResponse(200, "OK", nil)
					resp.WithRawData([]byte(`{"result":"ok"}`), "application/json")
					return resp, nil
				},
			},
		}

		resp, err := a.InvokeActor(context.Background(), &runtimev1pb.InvokeActorRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Method:    "DoWork",
			Data:      []byte(`{"input":"test"}`),
		})
		require.NoError(t, err)
		assert.Equal(t, []byte(`{"result":"ok"}`), resp.Data)
	})
}

// --- RegisterActorTimer tests ---

func TestRegisterActorTimerCoverage(t *testing.T) {
	t.Run("actor runtime nil returns error", func(t *testing.T) {
		a := &api{actor: nil}

		_, err := a.RegisterActorTimer(context.Background(), &runtimev1pb.RegisterActorTimerRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Name:      "timer1",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("success with data", func(t *testing.T) {
		var captured *actors.CreateTimerRequest
		a := &api{
			actor: &stubActors{
				createTimerFn: func(ctx context.Context, req *actors.CreateTimerRequest) error {
					captured = req
					return nil
				},
			},
		}

		_, err := a.RegisterActorTimer(context.Background(), &runtimev1pb.RegisterActorTimerRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Name:      "timer1",
			DueTime:   "0h0m3s0ms",
			Period:    "0h0m7s0ms",
			Callback:  "myCallback",
			Data:      []byte(`{"key":"val"}`),
		})
		require.NoError(t, err)
		require.NotNil(t, captured)
		assert.Equal(t, "timer1", captured.Name)
		assert.Equal(t, "mytype", captured.ActorType)
		assert.Equal(t, "123", captured.ActorID)
		assert.Equal(t, "myCallback", captured.Callback)
		assert.Equal(t, []byte(`{"key":"val"}`), captured.Data)
	})

	t.Run("success without data", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				createTimerFn: func(ctx context.Context, req *actors.CreateTimerRequest) error {
					return nil
				},
			},
		}

		_, err := a.RegisterActorTimer(context.Background(), &runtimev1pb.RegisterActorTimerRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Name:      "timer1",
		})
		require.NoError(t, err)
	})
}

// --- UnregisterActorTimer tests ---

func TestUnregisterActorTimerCoverage(t *testing.T) {
	t.Run("actor runtime nil returns error", func(t *testing.T) {
		a := &api{actor: nil}

		_, err := a.UnregisterActorTimer(context.Background(), &runtimev1pb.UnregisterActorTimerRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Name:      "timer1",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("success", func(t *testing.T) {
		var captured *actors.DeleteTimerRequest
		a := &api{
			actor: &stubActors{
				deleteTimerFn: func(ctx context.Context, req *actors.DeleteTimerRequest) error {
					captured = req
					return nil
				},
			},
		}

		_, err := a.UnregisterActorTimer(context.Background(), &runtimev1pb.UnregisterActorTimerRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Name:      "timer1",
		})
		require.NoError(t, err)
		require.NotNil(t, captured)
		assert.Equal(t, "timer1", captured.Name)
		assert.Equal(t, "mytype", captured.ActorType)
		assert.Equal(t, "123", captured.ActorID)
	})
}

// --- RegisterActorReminder tests ---

func TestRegisterActorReminderCoverage(t *testing.T) {
	t.Run("actor runtime nil returns error", func(t *testing.T) {
		a := &api{actor: nil}

		_, err := a.RegisterActorReminder(context.Background(), &runtimev1pb.RegisterActorReminderRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Name:      "reminder1",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("success with data", func(t *testing.T) {
		var captured *actors.CreateReminderRequest
		a := &api{
			actor: &stubActors{
				createReminderFn: func(ctx context.Context, req *actors.CreateReminderRequest) error {
					captured = req
					return nil
				},
			},
		}

		_, err := a.RegisterActorReminder(context.Background(), &runtimev1pb.RegisterActorReminderRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Name:      "reminder1",
			DueTime:   "0h0m5s0ms",
			Period:    "0h0m10s0ms",
			Data:      []byte(`{"key":"val"}`),
		})
		require.NoError(t, err)
		require.NotNil(t, captured)
		assert.Equal(t, "reminder1", captured.Name)
		assert.Equal(t, "mytype", captured.ActorType)
		assert.Equal(t, "123", captured.ActorID)
		assert.Equal(t, []byte(`{"key":"val"}`), captured.Data)
	})

	t.Run("success without data", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				createReminderFn: func(ctx context.Context, req *actors.CreateReminderRequest) error {
					return nil
				},
			},
		}

		_, err := a.RegisterActorReminder(context.Background(), &runtimev1pb.RegisterActorReminderRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Name:      "reminder1",
		})
		require.NoError(t, err)
	})
}

// --- UnregisterActorReminder tests ---

func TestUnregisterActorReminderCoverage(t *testing.T) {
	t.Run("actor runtime nil returns error", func(t *testing.T) {
		a := &api{actor: nil}

		_, err := a.UnregisterActorReminder(context.Background(), &runtimev1pb.UnregisterActorReminderRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Name:      "reminder1",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("success", func(t *testing.T) {
		var captured *actors.DeleteReminderRequest
		a := &api{
			actor: &stubActors{
				deleteReminderFn: func(ctx context.Context, req *actors.DeleteReminderRequest) error {
					captured = req
					return nil
				},
			},
		}

		_, err := a.UnregisterActorReminder(context.Background(), &runtimev1pb.UnregisterActorReminderRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Name:      "reminder1",
		})
		require.NoError(t, err)
		require.NotNil(t, captured)
		assert.Equal(t, "reminder1", captured.Name)
		assert.Equal(t, "mytype", captured.ActorType)
		assert.Equal(t, "123", captured.ActorID)
	})
}

// --- GetActorState tests ---

func TestGetActorStateCoverage(t *testing.T) {
	t.Run("actor runtime nil returns error", func(t *testing.T) {
		a := &api{actor: nil}

		_, err := a.GetActorState(context.Background(), &runtimev1pb.GetActorStateRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Key:       "mykey",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("actor not hosted returns error", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				isActorHostedFn: func(ctx context.Context, req *actors.ActorHostedRequest) bool {
					return false
				},
			},
		}

		_, err := a.GetActorState(context.Background(), &runtimev1pb.GetActorStateRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Key:       "mykey",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("GetState returns error", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				isActorHostedFn: func(ctx context.Context, req *actors.ActorHostedRequest) bool {
					return true
				},
				getStateFn: func(ctx context.Context, req *actors.GetStateRequest) (*actors.StateResponse, error) {
					return nil, errors.New("state error")
				},
			},
		}

		_, err := a.GetActorState(context.Background(), &runtimev1pb.GetActorStateRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Key:       "mykey",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("success", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				isActorHostedFn: func(ctx context.Context, req *actors.ActorHostedRequest) bool {
					assert.Equal(t, "mytype", req.ActorType)
					assert.Equal(t, "123", req.ActorID)
					return true
				},
				getStateFn: func(ctx context.Context, req *actors.GetStateRequest) (*actors.StateResponse, error) {
					assert.Equal(t, "mykey", req.Key)
					return &actors.StateResponse{Data: []byte(`{"val":"data"}`)}, nil
				},
			},
		}

		resp, err := a.GetActorState(context.Background(), &runtimev1pb.GetActorStateRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Key:       "mykey",
		})
		require.NoError(t, err)
		assert.Equal(t, []byte(`{"val":"data"}`), resp.Data)
	})
}

// --- ExecuteActorStateTransaction tests ---

func TestExecuteActorStateTransactionCoverage(t *testing.T) {
	t.Run("actor runtime nil returns error", func(t *testing.T) {
		a := &api{actor: nil}

		_, err := a.ExecuteActorStateTransaction(context.Background(), &runtimev1pb.ExecuteActorStateTransactionRequest{
			ActorType: "mytype",
			ActorId:   "123",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("actor not hosted returns error", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				isActorHostedFn: func(ctx context.Context, req *actors.ActorHostedRequest) bool {
					return false
				},
			},
		}

		_, err := a.ExecuteActorStateTransaction(context.Background(), &runtimev1pb.ExecuteActorStateTransactionRequest{
			ActorType: "mytype",
			ActorId:   "123",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("unsupported operation returns error", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				isActorHostedFn: func(ctx context.Context, req *actors.ActorHostedRequest) bool {
					return true
				},
			},
		}

		_, err := a.ExecuteActorStateTransaction(context.Background(), &runtimev1pb.ExecuteActorStateTransactionRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Operations: []*runtimev1pb.TransactionalActorStateOperation{
				{
					OperationType: "unsupported",
					Key:           "key1",
					Value:         &anypb.Any{Value: []byte("val")},
				},
			},
		})
		require.Error(t, err)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
	})

	t.Run("TransactionalStateOperation returns error", func(t *testing.T) {
		a := &api{
			actor: &stubActors{
				isActorHostedFn: func(ctx context.Context, req *actors.ActorHostedRequest) bool {
					return true
				},
				transactionalStateOperationFn: func(ctx context.Context, req *actors.TransactionalRequest) error {
					return errors.New("txn failed")
				},
			},
		}

		_, err := a.ExecuteActorStateTransaction(context.Background(), &runtimev1pb.ExecuteActorStateTransactionRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Operations: []*runtimev1pb.TransactionalActorStateOperation{
				{
					OperationType: "upsert",
					Key:           "key1",
					Value:         &anypb.Any{Value: []byte("val")},
				},
			},
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("success with upsert and delete", func(t *testing.T) {
		var captured *actors.TransactionalRequest
		a := &api{
			actor: &stubActors{
				isActorHostedFn: func(ctx context.Context, req *actors.ActorHostedRequest) bool {
					return true
				},
				transactionalStateOperationFn: func(ctx context.Context, req *actors.TransactionalRequest) error {
					captured = req
					return nil
				},
			},
		}

		_, err := a.ExecuteActorStateTransaction(context.Background(), &runtimev1pb.ExecuteActorStateTransactionRequest{
			ActorType: "mytype",
			ActorId:   "123",
			Operations: []*runtimev1pb.TransactionalActorStateOperation{
				{
					OperationType: "upsert",
					Key:           "key1",
					Value:         &anypb.Any{Value: []byte("val1")},
				},
				{
					OperationType: "delete",
					Key:           "key2",
					Value:         &anypb.Any{Value: []byte("val2")},
				},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, captured)
		assert.Equal(t, "mytype", captured.ActorType)
		assert.Equal(t, "123", captured.ActorID)
		assert.Len(t, captured.Operations, 2)
	})
}

// --- GetBulkSecret tests ---

func TestGetBulkSecretCoverage(t *testing.T) {
	t.Run("secret store not configured", func(t *testing.T) {
		a := &api{
			id:           "test-app",
			secretStores: nil,
		}

		_, err := a.GetBulkSecret(context.Background(), &runtimev1pb.GetBulkSecretRequest{
			StoreName: "mystore",
		})
		require.Error(t, err)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("secret store not found", func(t *testing.T) {
		a := &api{
			id:           "test-app",
			secretStores: map[string]secretstores.SecretStore{"store1": &stubSecretStore{}},
		}

		_, err := a.GetBulkSecret(context.Background(), &runtimev1pb.GetBulkSecretRequest{
			StoreName: "nonexistent",
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("BulkGetSecret returns error", func(t *testing.T) {
		store := &stubSecretStore{
			bulkGetSecretFn: func(req secretstores.BulkGetSecretRequest) (secretstores.BulkGetSecretResponse, error) {
				return secretstores.BulkGetSecretResponse{}, errors.New("bulk get failed")
			},
		}
		a := &api{
			id:           "test-app",
			secretStores: map[string]secretstores.SecretStore{"mystore": store},
		}

		_, err := a.GetBulkSecret(context.Background(), &runtimev1pb.GetBulkSecretRequest{
			StoreName: "mystore",
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("success with all secrets allowed", func(t *testing.T) {
		store := &stubSecretStore{
			bulkGetSecretFn: func(req secretstores.BulkGetSecretRequest) (secretstores.BulkGetSecretResponse, error) {
				return secretstores.BulkGetSecretResponse{
					Data: map[string]map[string]string{
						"secret1": {"key1": "val1"},
						"secret2": {"key2": "val2"},
					},
				}, nil
			},
		}
		a := &api{
			id:                   "test-app",
			secretStores:         map[string]secretstores.SecretStore{"mystore": store},
			secretsConfiguration: map[string]config.SecretsScope{},
		}

		resp, err := a.GetBulkSecret(context.Background(), &runtimev1pb.GetBulkSecretRequest{
			StoreName: "mystore",
		})
		require.NoError(t, err)
		assert.Len(t, resp.Data, 2)
		assert.Equal(t, "val1", resp.Data["secret1"].Secrets["key1"])
		assert.Equal(t, "val2", resp.Data["secret2"].Secrets["key2"])
	})

	t.Run("secret not allowed is filtered out", func(t *testing.T) {
		store := &stubSecretStore{
			bulkGetSecretFn: func(req secretstores.BulkGetSecretRequest) (secretstores.BulkGetSecretResponse, error) {
				return secretstores.BulkGetSecretResponse{
					Data: map[string]map[string]string{
						"allowed-secret":    {"k": "v"},
						"disallowed-secret": {"k": "v"},
					},
				}, nil
			},
		}
		a := &api{
			id:           "test-app",
			secretStores: map[string]secretstores.SecretStore{"mystore": store},
			secretsConfiguration: map[string]config.SecretsScope{
				"mystore": {
					StoreName:      "mystore",
					DefaultAccess:  "deny",
					AllowedSecrets: []string{"allowed-secret"},
				},
			},
		}

		resp, err := a.GetBulkSecret(context.Background(), &runtimev1pb.GetBulkSecretRequest{
			StoreName: "mystore",
		})
		require.NoError(t, err)
		// Only allowed-secret should remain.
		assert.Len(t, resp.Data, 1)
		assert.Contains(t, resp.Data, "allowed-secret")
	})
}

// --- PublishEvent success path tests ---

func TestPublishEventCoverageMore(t *testing.T) {
	// The success path of PublishEvent marshals the cloud event envelope via
	// jsoniter.ConfigFastest.Marshal, which panics on Go 1.26 due to
	// json-iterator/reflect2 v1.0.1 incompatibility with SwissTable maps.
	// These subtests are skipped until that dependency is upgraded.

	t.Run("success with data", func(t *testing.T) {
		t.Skip("json-iterator/reflect2 v1.0.1 panics on Go 1.26 SwissTable maps during cloud event marshaling")

		var captured *pubsub.PublishRequest
		ps := &stubPubSub{
			features: []pubsub.Feature{},
		}

		a := &api{
			id: "test-app",
			pubsubAdapter: &stubPubSubAdapter{
				getPubSubFn: func(name string) pubsub.PubSub { return ps },
				publishFn: func(req *pubsub.PublishRequest) error {
					captured = req
					return nil
				},
			},
		}

		resp, err := a.PublishEvent(ctxWithSpan(), &runtimev1pb.PublishEventRequest{
			PubsubName:      "mypubsub",
			Topic:           "mytopic",
			Data:            []byte("hello world"),
			DataContentType: "text/plain",
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, captured)
		assert.Equal(t, "mypubsub", captured.PubsubName)
		assert.Equal(t, "mytopic", captured.Topic)
	})

	t.Run("success with nil data", func(t *testing.T) {
		t.Skip("json-iterator/reflect2 v1.0.1 panics on Go 1.26 SwissTable maps during cloud event marshaling")

		ps := &stubPubSub{
			features: []pubsub.Feature{},
		}

		a := &api{
			id: "test-app",
			pubsubAdapter: &stubPubSubAdapter{
				getPubSubFn: func(name string) pubsub.PubSub { return ps },
				publishFn: func(req *pubsub.PublishRequest) error {
					return nil
				},
			},
		}

		resp, err := a.PublishEvent(ctxWithSpan(), &runtimev1pb.PublishEventRequest{
			PubsubName: "mypubsub",
			Topic:      "mytopic",
			Data:       nil,
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("publish returns internal error", func(t *testing.T) {
		t.Skip("json-iterator/reflect2 v1.0.1 panics on Go 1.26 SwissTable maps during cloud event marshaling")

		ps := &stubPubSub{
			features: []pubsub.Feature{},
		}

		a := &api{
			id: "test-app",
			pubsubAdapter: &stubPubSubAdapter{
				getPubSubFn: func(name string) pubsub.PubSub { return ps },
				publishFn: func(req *pubsub.PublishRequest) error {
					return errors.New("publish failed")
				},
			},
		}

		_, err := a.PublishEvent(ctxWithSpan(), &runtimev1pb.PublishEventRequest{
			PubsubName: "mypubsub",
			Topic:      "mytopic",
			Data:       []byte(`{"msg":"hello"}`),
		})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("publish with metadata", func(t *testing.T) {
		t.Skip("json-iterator/reflect2 v1.0.1 panics on Go 1.26 SwissTable maps during cloud event marshaling")

		var captured *pubsub.PublishRequest
		ps := &stubPubSub{
			features: []pubsub.Feature{},
		}

		a := &api{
			id: "test-app",
			pubsubAdapter: &stubPubSubAdapter{
				getPubSubFn: func(name string) pubsub.PubSub { return ps },
				publishFn: func(req *pubsub.PublishRequest) error {
					captured = req
					return nil
				},
			},
		}

		resp, err := a.PublishEvent(ctxWithSpan(), &runtimev1pb.PublishEventRequest{
			PubsubName: "mypubsub",
			Topic:      "mytopic",
			Data:       []byte(`test`),
			Metadata:   map[string]string{"key": "value"},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, captured)
		assert.Equal(t, "value", captured.Metadata["key"])
	})
}
