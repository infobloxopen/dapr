// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/dapr/dapr/pkg/channel"
	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
	operatorv1pb "github.com/dapr/dapr/pkg/proto/operator/v1"
	commonv1pb "github.com/dapr/dapr/pkg/proto/common/v1"
	runtimev1pb "github.com/dapr/dapr/pkg/proto/runtime/v1"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	subscriptionsapi "github.com/dapr/dapr/pkg/apis/subscriptions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

// stubAppChannel implements channel.AppChannel for testing GetSubscriptionsHTTP.
type stubAppChannel struct {
	resp *invokev1.InvokeMethodResponse
	err  error
}

var _ channel.AppChannel = (*stubAppChannel)(nil)

func (s *stubAppChannel) GetBaseAddress() string { return "localhost" }

func (s *stubAppChannel) InvokeMethod(_ context.Context, _ *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
	return s.resp, s.err
}

// stubAppCallbackClient implements runtimev1pb.AppCallbackClient for testing GetSubscriptionsGRPC.
type stubAppCallbackClient struct {
	resp *runtimev1pb.ListTopicSubscriptionsResponse
	err  error
}

var _ runtimev1pb.AppCallbackClient = (*stubAppCallbackClient)(nil)

func (s *stubAppCallbackClient) OnInvoke(_ context.Context, _ *commonv1pb.InvokeRequest, _ ...grpc.CallOption) (*commonv1pb.InvokeResponse, error) {
	return nil, nil
}

func (s *stubAppCallbackClient) ListTopicSubscriptions(_ context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (*runtimev1pb.ListTopicSubscriptionsResponse, error) {
	return s.resp, s.err
}

func (s *stubAppCallbackClient) OnTopicEvent(_ context.Context, _ *runtimev1pb.TopicEventRequest, _ ...grpc.CallOption) (*runtimev1pb.TopicEventResponse, error) {
	return nil, nil
}

func (s *stubAppCallbackClient) ListInputBindings(_ context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (*runtimev1pb.ListInputBindingsResponse, error) {
	return nil, nil
}

func (s *stubAppCallbackClient) OnBindingEvent(_ context.Context, _ *runtimev1pb.BindingEventRequest, _ ...grpc.CallOption) (*runtimev1pb.BindingEventResponse, error) {
	return nil, nil
}

// stubOperatorClient implements operatorv1pb.OperatorClient for testing DeclarativeKubernetes.
type stubOperatorClient struct {
	resp *operatorv1pb.ListSubscriptionsResponse
	err  error
}

var _ operatorv1pb.OperatorClient = (*stubOperatorClient)(nil)

func (s *stubOperatorClient) ComponentUpdate(_ context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (operatorv1pb.Operator_ComponentUpdateClient, error) {
	return nil, nil
}

func (s *stubOperatorClient) ListComponents(_ context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (*operatorv1pb.ListComponentResponse, error) {
	return nil, nil
}

func (s *stubOperatorClient) GetConfiguration(_ context.Context, _ *operatorv1pb.GetConfigurationRequest, _ ...grpc.CallOption) (*operatorv1pb.GetConfigurationResponse, error) {
	return nil, nil
}

func (s *stubOperatorClient) ListSubscriptions(_ context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (*operatorv1pb.ListSubscriptionsResponse, error) {
	return s.resp, s.err
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeHTTPResponse builds an InvokeMethodResponse with the given status code
// and optional raw JSON body.
func makeHTTPResponse(statusCode int32, body []byte) *invokev1.InvokeMethodResponse {
	resp := invokev1.NewInvokeMethodResponse(statusCode, http.StatusText(int(statusCode)), nil)
	resp.WithRawData(body, invokev1.JSONContentType)
	return resp
}

// subscriptionCRDBytes serialises a subscription CRD to YAML bytes.
func subscriptionCRDBytes(kind, topic, route, pubsubname string, scopes []string) []byte {
	s := subscriptionsapi.Subscription{
		TypeMeta: metav1.TypeMeta{Kind: kind},
		Spec: subscriptionsapi.SubscriptionSpec{
			Topic:      topic,
			Route:      route,
			Pubsubname: pubsubname,
		},
		Scopes: scopes,
	}
	b, _ := yaml.Marshal(s)
	return b
}

// ---------------------------------------------------------------------------
// Tests: GetSubscriptionsHTTP
// ---------------------------------------------------------------------------

func TestGetSubscriptionsHTTP(t *testing.T) {
	validSubs := []Subscription{
		{PubsubName: "pubsub", Topic: "orders", Route: "/orders"},
		{PubsubName: "pubsub", Topic: "events", Route: "/events"},
	}
	validJSON, err := json.Marshal(validSubs)
	require.NoError(t, err)

	tests := []struct {
		name     string
		resp     *invokev1.InvokeMethodResponse
		err      error
		wantLen  int
		wantSubs []Subscription
	}{
		{
			name:    "status 200 with valid subscriptions",
			resp:    makeHTTPResponse(http.StatusOK, validJSON),
			wantLen: 2,
			wantSubs: []Subscription{
				{PubsubName: "pubsub", Topic: "orders", Route: "/orders"},
				{PubsubName: "pubsub", Topic: "events", Route: "/events"},
			},
		},
		{
			name:    "status 200 with empty JSON array",
			resp:    makeHTTPResponse(http.StatusOK, []byte(`[]`)),
			wantLen: 0,
		},
		{
			name:    "status 200 with invalid JSON",
			resp:    makeHTTPResponse(http.StatusOK, []byte(`not-json`)),
			wantLen: 0,
		},
		{
			name:    "status 404 no subscriptions",
			resp:    makeHTTPResponse(http.StatusNotFound, nil),
			wantLen: 0,
		},
		{
			name:    "unexpected status code 500",
			resp:    makeHTTPResponse(http.StatusInternalServerError, nil),
			wantLen: 0,
		},
		{
			name:    "invoke error with valid fallback response",
			resp:    makeHTTPResponse(http.StatusOK, []byte(`[]`)),
			err:     errors.New("connection refused"),
			wantLen: 0,
		},
		{
			name: "subscriptions with empty route are filtered out",
			resp: makeHTTPResponse(http.StatusOK, []byte(`[
				{"pubsubname":"pubsub","topic":"t1","route":"/t1"},
				{"pubsubname":"pubsub","topic":"t2","route":""}
			]`)),
			wantLen: 1,
			wantSubs: []Subscription{
				{PubsubName: "pubsub", Topic: "t1", Route: "/t1"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := &stubAppChannel{resp: tc.resp, err: tc.err}
			subs := GetSubscriptionsHTTP(ch, log)
			assert.Len(t, subs, tc.wantLen)
			if tc.wantSubs != nil {
				for i, want := range tc.wantSubs {
					assert.Equal(t, want.PubsubName, subs[i].PubsubName)
					assert.Equal(t, want.Topic, subs[i].Topic)
					assert.Equal(t, want.Route, subs[i].Route)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: GetSubscriptionsGRPC
// ---------------------------------------------------------------------------

func TestGetSubscriptionsGRPC(t *testing.T) {
	tests := []struct {
		name     string
		resp     *runtimev1pb.ListTopicSubscriptionsResponse
		err      error
		wantLen  int
		wantSubs []Subscription
	}{
		{
			name:    "error from ListTopicSubscriptions",
			err:     errors.New("unavailable"),
			wantLen: 0,
		},
		{
			name:    "nil response",
			resp:    nil,
			wantLen: 0,
		},
		{
			name:    "response with nil subscriptions slice",
			resp:    &runtimev1pb.ListTopicSubscriptionsResponse{Subscriptions: nil},
			wantLen: 0,
		},
		{
			name:    "response with empty subscriptions slice",
			resp:    &runtimev1pb.ListTopicSubscriptionsResponse{Subscriptions: []*runtimev1pb.TopicSubscription{}},
			wantLen: 0,
		},
		{
			name: "response with valid subscriptions",
			resp: &runtimev1pb.ListTopicSubscriptionsResponse{
				Subscriptions: []*runtimev1pb.TopicSubscription{
					{PubsubName: "pubsub", Topic: "orders", Metadata: map[string]string{"key": "val"}},
					{PubsubName: "pubsub", Topic: "events"},
				},
			},
			wantLen: 2,
			wantSubs: []Subscription{
				{PubsubName: "pubsub", Topic: "orders", Metadata: map[string]string{"key": "val"}},
				{PubsubName: "pubsub", Topic: "events"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &stubAppCallbackClient{resp: tc.resp, err: tc.err}
			subs := GetSubscriptionsGRPC(client, log)
			assert.Len(t, subs, tc.wantLen)
			if tc.wantSubs != nil {
				for i, want := range tc.wantSubs {
					assert.Equal(t, want.PubsubName, subs[i].PubsubName)
					assert.Equal(t, want.Topic, subs[i].Topic)
					if want.Metadata != nil {
						assert.Equal(t, want.Metadata, subs[i].Metadata)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: DeclarativeKubernetes
// ---------------------------------------------------------------------------

func TestDeclarativeKubernetes(t *testing.T) {
	validCRD := subscriptionCRDBytes("Subscription", "orders", "/orders", "pubsub", nil)

	tests := []struct {
		name    string
		resp    *operatorv1pb.ListSubscriptionsResponse
		err     error
		wantLen int
	}{
		{
			name:    "error from ListSubscriptions",
			err:     errors.New("operator unavailable"),
			wantLen: 0,
		},
		{
			name: "valid subscription CRD",
			resp: &operatorv1pb.ListSubscriptionsResponse{
				Subscriptions: [][]byte{validCRD},
			},
			wantLen: 1,
		},
		{
			name: "invalid subscription bytes",
			resp: &operatorv1pb.ListSubscriptionsResponse{
				Subscriptions: [][]byte{[]byte(`{{{not yaml`)},
			},
			wantLen: 0,
		},
		{
			name: "wrong Kind is silently skipped",
			resp: &operatorv1pb.ListSubscriptionsResponse{
				Subscriptions: [][]byte{
					subscriptionCRDBytes("Component", "orders", "/orders", "pubsub", nil),
				},
			},
			wantLen: 0,
		},
		{
			name: "mix of valid and invalid CRDs loses prior entries on error",
			resp: &operatorv1pb.ListSubscriptionsResponse{
				Subscriptions: [][]byte{
					validCRD,
					[]byte(`{{{bad`),
					subscriptionCRDBytes("Subscription", "events", "/events", "pubsub", []string{"app1"}),
				},
			},
			// appendSubscription returns (nil, err) on bad YAML, so the
			// previously accumulated list is lost. Only entries after the
			// last error survive.
			wantLen: 1,
		},
		{
			name: "empty subscriptions list",
			resp: &operatorv1pb.ListSubscriptionsResponse{
				Subscriptions: [][]byte{},
			},
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &stubOperatorClient{resp: tc.resp, err: tc.err}
			subs := DeclarativeKubernetes(client, log)
			assert.Len(t, subs, tc.wantLen)
		})
	}

	t.Run("valid CRD fields are parsed correctly", func(t *testing.T) {
		crd := subscriptionCRDBytes("Subscription", "orders", "/orders", "pubsub", []string{"app1", "app2"})
		client := &stubOperatorClient{
			resp: &operatorv1pb.ListSubscriptionsResponse{
				Subscriptions: [][]byte{crd},
			},
		}
		subs := DeclarativeKubernetes(client, log)
		require.Len(t, subs, 1)
		assert.Equal(t, "orders", subs[0].Topic)
		assert.Equal(t, "/orders", subs[0].Route)
		assert.Equal(t, "pubsub", subs[0].PubsubName)
		assert.Equal(t, []string{"app1", "app2"}, subs[0].Scopes)
	})
}

// ---------------------------------------------------------------------------
// Tests: marshalSubscription (improve from 71.4%)
// ---------------------------------------------------------------------------

func TestMarshalSubscription(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantSub *Subscription
		wantErr bool
	}{
		{
			name:  "valid Subscription kind",
			input: subscriptionCRDBytes("Subscription", "topic1", "/route1", "pubsub", []string{"scope1"}),
			wantSub: &Subscription{
				Topic:      "topic1",
				Route:      "/route1",
				PubsubName: "pubsub",
				Scopes:     []string{"scope1"},
			},
		},
		{
			name:    "wrong Kind returns nil without error",
			input:   subscriptionCRDBytes("Component", "topic1", "/route1", "pubsub", nil),
			wantSub: nil,
			wantErr: false,
		},
		{
			name:    "invalid YAML returns error",
			input:   []byte(`{{{not valid yaml`),
			wantSub: nil,
			wantErr: true,
		},
		{
			name:    "empty input returns nil without error",
			input:   []byte(``),
			wantSub: nil,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sub, err := marshalSubscription(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, sub)
			} else {
				assert.NoError(t, err)
				if tc.wantSub == nil {
					assert.Nil(t, sub)
				} else {
					require.NotNil(t, sub)
					assert.Equal(t, tc.wantSub.Topic, sub.Topic)
					assert.Equal(t, tc.wantSub.Route, sub.Route)
					assert.Equal(t, tc.wantSub.PubsubName, sub.PubsubName)
					assert.Equal(t, tc.wantSub.Scopes, sub.Scopes)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: appendSubscription (improve from 83.3%)
// ---------------------------------------------------------------------------

func TestAppendSubscription(t *testing.T) {
	tests := []struct {
		name     string
		initial  []Subscription
		input    []byte
		wantLen  int
		wantErr  bool
	}{
		{
			name:    "valid subscription appended",
			initial: nil,
			input:   subscriptionCRDBytes("Subscription", "topic1", "/route1", "pubsub", nil),
			wantLen: 1,
		},
		{
			name:    "wrong kind is skipped without error",
			initial: nil,
			input:   subscriptionCRDBytes("Component", "topic1", "/route1", "pubsub", nil),
			wantLen: 0,
		},
		{
			name:    "invalid YAML returns error",
			initial: nil,
			input:   []byte(`{{{not valid`),
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "appended to existing list",
			initial: []Subscription{{Topic: "existing"}},
			input:   subscriptionCRDBytes("Subscription", "new", "/new", "pubsub", nil),
			wantLen: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := appendSubscription(tc.initial, tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tc.wantLen)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: DeclarativeSelfHosted (improve from 73.7%)
// ---------------------------------------------------------------------------

func TestDeclarativeSelfHostedAdditional(t *testing.T) {
	t.Run("directory with subdirectory is skipped", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "subs-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		// Create a subdirectory that should be skipped.
		require.NoError(t, os.MkdirAll(dir+"/subdir", 0777))

		// Also create a valid subscription file.
		b := subscriptionCRDBytes("Subscription", "topic1", "/route1", "pubsub", nil)
		require.NoError(t, ioutil.WriteFile(dir+"/sub.yaml", b, 0600))

		subs := DeclarativeSelfHosted(dir, log)
		assert.Len(t, subs, 1)
		assert.Equal(t, "topic1", subs[0].Topic)
	})

	t.Run("file with invalid YAML is skipped", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "subs-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		require.NoError(t, ioutil.WriteFile(dir+"/bad.yaml", []byte(`{{{not yaml`), 0600))

		subs := DeclarativeSelfHosted(dir, log)
		assert.Len(t, subs, 0)
	})

	t.Run("nonexistent path returns empty", func(t *testing.T) {
		subs := DeclarativeSelfHosted("/tmp/does-not-exist-"+t.Name(), log)
		assert.Len(t, subs, 0)
	})

	t.Run("file with wrong kind is skipped", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "subs-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		b := subscriptionCRDBytes("Component", "topic1", "/route1", "pubsub", nil)
		require.NoError(t, ioutil.WriteFile(dir+"/comp.yaml", b, 0600))

		subs := DeclarativeSelfHosted(dir, log)
		assert.Len(t, subs, 0)
	})
}
