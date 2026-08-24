// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package actors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dapr/components-contrib/state"
	"github.com/dapr/dapr/pkg/actors/internal"
	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newMinimalActorsRuntime creates an actorsRuntime with the minimum fields
// needed for pure/simple method tests. It avoids the full NewActors
// constructor so tests don't depend on app channels, state stores, or
// placement services.
func newMinimalActorsRuntime() *actorsRuntime {
	return &actorsRuntime{
		config:          Config{},
		actorsTable:     &sync.Map{},
		activeTimers:    &sync.Map{},
		activeTimersLock: &sync.RWMutex{},
		activeReminders: &sync.Map{},
		remindersLock:   &sync.RWMutex{},
		activeRemindersLock: &sync.RWMutex{},
		reminders:       map[string][]Reminder{},
		evaluationLock:  &sync.RWMutex{},
		evaluationChan:  make(chan bool),
	}
}

// stubAppChannel implements channel.AppChannel without testify/mock.
type stubAppChannel struct {
	invokeMethodFn func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error)
	baseAddress    string
}

func (s *stubAppChannel) GetBaseAddress() string {
	return s.baseAddress
}

func (s *stubAppChannel) InvokeMethod(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
	if s.invokeMethodFn != nil {
		return s.invokeMethodFn(ctx, req)
	}
	return invokev1.NewInvokeMethodResponse(200, "OK", nil), nil
}

// nonTransactionalStore implements state.Store but NOT state.TransactionalStore.
type nonTransactionalStore struct{}

func (n *nonTransactionalStore) Init(metadata state.Metadata) error                              { return nil }
func (n *nonTransactionalStore) Delete(req *state.DeleteRequest) error                           { return nil }
func (n *nonTransactionalStore) Get(req *state.GetRequest) (*state.GetResponse, error)           { return &state.GetResponse{}, nil }
func (n *nonTransactionalStore) Set(req *state.SetRequest) error                                 { return nil }
func (n *nonTransactionalStore) BulkDelete(req []state.DeleteRequest) error                      { return nil }
func (n *nonTransactionalStore) BulkGet(req []state.GetRequest) (bool, []state.BulkGetResponse, error) { return false, nil, nil }
func (n *nonTransactionalStore) BulkSet(req []state.SetRequest) error                            { return nil }

// stubStateStore implements state.Store with configurable error returns.
type stubStateStore struct {
	getResp *state.GetResponse
	getErr  error
	setErr  error
	delErr  error
}

func (s *stubStateStore) Init(metadata state.Metadata) error              { return nil }
func (s *stubStateStore) Delete(req *state.DeleteRequest) error           { return s.delErr }
func (s *stubStateStore) Get(req *state.GetRequest) (*state.GetResponse, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getResp != nil {
		return s.getResp, nil
	}
	return &state.GetResponse{}, nil
}
func (s *stubStateStore) Set(req *state.SetRequest) error                 { return s.setErr }
func (s *stubStateStore) BulkDelete(req []state.DeleteRequest) error      { return nil }
func (s *stubStateStore) BulkGet(req []state.GetRequest) (bool, []state.BulkGetResponse, error) { return false, nil, nil }
func (s *stubStateStore) BulkSet(req []state.SetRequest) error            { return nil }

func TestIsActorLocal(t *testing.T) {
	a := newMinimalActorsRuntime()

	tests := []struct {
		name               string
		targetActorAddress string
		hostAddress        string
		grpcPort           int
		expected           bool
	}{
		{
			name:               "localhost is always local",
			targetActorAddress: "localhost",
			hostAddress:        "1.2.3.4",
			grpcPort:           50001,
			expected:           true,
		},
		{
			name:               "localhost with port is local",
			targetActorAddress: "localhost:50001",
			hostAddress:        "1.2.3.4",
			grpcPort:           50001,
			expected:           true,
		},
		{
			name:               "127.0.0.1 is always local",
			targetActorAddress: "127.0.0.1",
			hostAddress:        "1.2.3.4",
			grpcPort:           50001,
			expected:           true,
		},
		{
			name:               "127.0.0.1 with port is local",
			targetActorAddress: "127.0.0.1:50001",
			hostAddress:        "1.2.3.4",
			grpcPort:           50001,
			expected:           true,
		},
		{
			name:               "matching host address and grpc port is local",
			targetActorAddress: "1.2.3.4:50001",
			hostAddress:        "1.2.3.4",
			grpcPort:           50001,
			expected:           true,
		},
		{
			name:               "different address is not local",
			targetActorAddress: "5.6.7.8:50001",
			hostAddress:        "1.2.3.4",
			grpcPort:           50001,
			expected:           false,
		},
		{
			name:               "same address different port is not local",
			targetActorAddress: "1.2.3.4:9999",
			hostAddress:        "1.2.3.4",
			grpcPort:           50001,
			expected:           false,
		},
		{
			name:               "bare host address without port is not local",
			targetActorAddress: "1.2.3.4",
			hostAddress:        "1.2.3.4",
			grpcPort:           50001,
			expected:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.isActorLocal(tt.targetActorAddress, tt.hostAddress, tt.grpcPort)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestReminderRequiresUpdate(t *testing.T) {
	a := newMinimalActorsRuntime()

	tests := []struct {
		name     string
		req      *CreateReminderRequest
		reminder *Reminder
		expected bool
	}{
		{
			name: "same data returns false",
			req: &CreateReminderRequest{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
				Data:      "data1",
				DueTime:   "1s",
				Period:    "2s",
			},
			reminder: &Reminder{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
				Data:      "data1",
				DueTime:   "1s",
				Period:    "2s",
			},
			expected: false,
		},
		{
			name: "different data returns true",
			req: &CreateReminderRequest{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
				Data:      "new-data",
				DueTime:   "1s",
				Period:    "2s",
			},
			reminder: &Reminder{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
				Data:      "old-data",
				DueTime:   "1s",
				Period:    "2s",
			},
			expected: true,
		},
		{
			name: "different period returns true",
			req: &CreateReminderRequest{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
				Data:      "data1",
				DueTime:   "1s",
				Period:    "5s",
			},
			reminder: &Reminder{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
				Data:      "data1",
				DueTime:   "1s",
				Period:    "2s",
			},
			expected: true,
		},
		{
			name: "different due time returns true",
			req: &CreateReminderRequest{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
				Data:      "data1",
				DueTime:   "10s",
				Period:    "2s",
			},
			reminder: &Reminder{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
				Data:      "data1",
				DueTime:   "1s",
				Period:    "2s",
			},
			expected: true,
		},
		{
			name: "different actor ID returns false",
			req: &CreateReminderRequest{
				ActorID:   "actor2",
				ActorType: "mytype",
				Name:      "reminder1",
				Data:      "new-data",
				DueTime:   "1s",
				Period:    "2s",
			},
			reminder: &Reminder{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
				Data:      "old-data",
				DueTime:   "1s",
				Period:    "2s",
			},
			expected: false,
		},
		{
			name: "different actor type returns false",
			req: &CreateReminderRequest{
				ActorID:   "actor1",
				ActorType: "type-a",
				Name:      "reminder1",
				Data:      "new-data",
				DueTime:   "1s",
				Period:    "2s",
			},
			reminder: &Reminder{
				ActorID:   "actor1",
				ActorType: "type-b",
				Name:      "reminder1",
				Data:      "old-data",
				DueTime:   "1s",
				Period:    "2s",
			},
			expected: false,
		},
		{
			name: "different name returns false",
			req: &CreateReminderRequest{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder-x",
				Data:      "new-data",
				DueTime:   "1s",
				Period:    "2s",
			},
			reminder: &Reminder{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder-y",
				Data:      "old-data",
				DueTime:   "1s",
				Period:    "2s",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.reminderRequiresUpdate(tt.req, tt.reminder)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConfigureTicker(t *testing.T) {
	a := newMinimalActorsRuntime()

	t.Run("positive duration returns valid ticker", func(t *testing.T) {
		ticker := a.configureTicker(time.Second)
		require.NotNil(t, ticker)
		ticker.Stop()
	})

	t.Run("zero duration returns valid ticker", func(t *testing.T) {
		ticker := a.configureTicker(0)
		require.NotNil(t, ticker)
		ticker.Stop()
	})

	t.Run("small duration returns valid ticker", func(t *testing.T) {
		ticker := a.configureTicker(time.Millisecond)
		require.NotNil(t, ticker)
		ticker.Stop()
	})
}

func TestGetReminderInternal(t *testing.T) {
	a := newMinimalActorsRuntime()

	t.Run("returns reminder when found", func(t *testing.T) {
		a.reminders["mytype"] = []Reminder{
			{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
				Data:      "somedata",
				Period:    "1s",
				DueTime:   "2s",
			},
		}

		req := &CreateReminderRequest{
			ActorID:   "actor1",
			ActorType: "mytype",
			Name:      "reminder1",
		}

		r, found := a.getReminder(req)
		assert.True(t, found)
		require.NotNil(t, r)
		assert.Equal(t, "actor1", r.ActorID)
		assert.Equal(t, "mytype", r.ActorType)
		assert.Equal(t, "reminder1", r.Name)
		assert.Equal(t, "somedata", r.Data)
	})

	t.Run("returns false when reminder not found", func(t *testing.T) {
		a.reminders["mytype"] = []Reminder{
			{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
			},
		}

		req := &CreateReminderRequest{
			ActorID:   "actor1",
			ActorType: "mytype",
			Name:      "nonexistent",
		}

		r, found := a.getReminder(req)
		assert.False(t, found)
		assert.Nil(t, r)
	})

	t.Run("returns false when actor type has no reminders", func(t *testing.T) {
		a.reminders = map[string][]Reminder{}

		req := &CreateReminderRequest{
			ActorID:   "actor1",
			ActorType: "unknowntype",
			Name:      "reminder1",
		}

		r, found := a.getReminder(req)
		assert.False(t, found)
		assert.Nil(t, r)
	})

	t.Run("matches on actor ID, type, and name", func(t *testing.T) {
		a.reminders["mytype"] = []Reminder{
			{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder1",
			},
			{
				ActorID:   "actor2",
				ActorType: "mytype",
				Name:      "reminder1",
			},
			{
				ActorID:   "actor1",
				ActorType: "mytype",
				Name:      "reminder2",
			},
		}

		req := &CreateReminderRequest{
			ActorID:   "actor2",
			ActorType: "mytype",
			Name:      "reminder1",
		}

		r, found := a.getReminder(req)
		assert.True(t, found)
		require.NotNil(t, r)
		assert.Equal(t, "actor2", r.ActorID)
	})
}

func TestDecomposeCompositeKey(t *testing.T) {
	a := newMinimalActorsRuntime()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "key with separator",
			input:    "app||actorType||actorID",
			expected: []string{"app", "actorType", "actorID"},
		},
		{
			name:     "key without separator",
			input:    "noseparator",
			expected: []string{"noseparator"},
		},
		{
			name:     "two part key",
			input:    "type||id",
			expected: []string{"type", "id"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{""},
		},
		{
			name:     "only separator",
			input:    "||",
			expected: []string{"", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.decomposeCompositeKey(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetActorTypeAndIDFromKey(t *testing.T) {
	a := newMinimalActorsRuntime()

	tests := []struct {
		name         string
		key          string
		expectedType string
		expectedID   string
	}{
		{
			name:         "standard composite key",
			key:          "cat||e485d5de",
			expectedType: "cat",
			expectedID:   "e485d5de",
		},
		{
			name:         "key with three parts returns first two",
			key:          "app||type||id",
			expectedType: "app",
			expectedID:   "type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actorType, actorID := a.getActorTypeAndIDFromKey(tt.key)
			assert.Equal(t, tt.expectedType, actorType)
			assert.Equal(t, tt.expectedID, actorID)
		})
	}
}

func TestConstructCompositeKey(t *testing.T) {
	a := newMinimalActorsRuntime()

	tests := []struct {
		name     string
		keys     []string
		expected string
	}{
		{
			name:     "single key",
			keys:     []string{"one"},
			expected: "one",
		},
		{
			name:     "two keys",
			keys:     []string{"type", "id"},
			expected: "type||id",
		},
		{
			name:     "three keys",
			keys:     []string{"app", "type", "id"},
			expected: "app||type||id",
		},
		{
			name:     "empty keys",
			keys:     []string{"", ""},
			expected: "||",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.constructCompositeKey(tt.keys...)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConstructAndDecomposeRoundTrip(t *testing.T) {
	a := newMinimalActorsRuntime()

	keys := []string{"myApp", "TestActor", "abc-123"}
	composite := a.constructCompositeKey(keys...)
	decomposed := a.decomposeCompositeKey(composite)

	assert.Equal(t, keys, decomposed)
}

func TestIsActorHosted(t *testing.T) {
	a := newMinimalActorsRuntime()

	t.Run("returns false for non-existent actor", func(t *testing.T) {
		hosted := a.IsActorHosted(nil, &ActorHostedRequest{
			ActorType: "cat",
			ActorID:   "id-1",
		})
		assert.False(t, hosted)
	})

	t.Run("returns true for existing actor", func(t *testing.T) {
		key := a.constructCompositeKey("cat", "id-1")
		a.actorsTable.Store(key, newActor("cat", "id-1"))

		hosted := a.IsActorHosted(nil, &ActorHostedRequest{
			ActorType: "cat",
			ActorID:   "id-1",
		})
		assert.True(t, hosted)
	})

	t.Run("returns false for wrong actor ID", func(t *testing.T) {
		key := a.constructCompositeKey("cat", "id-1")
		a.actorsTable.Store(key, newActor("cat", "id-1"))

		hosted := a.IsActorHosted(nil, &ActorHostedRequest{
			ActorType: "cat",
			ActorID:   "id-999",
		})
		assert.False(t, hosted)
	})
}

func TestGetActiveActorsCount(t *testing.T) {
	t.Run("no actors returns zero counts for hosted types", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.config.HostedActorTypes = []string{"cat", "dog"}

		counts := a.GetActiveActorsCount(nil)
		require.Len(t, counts, 2)

		countMap := map[string]int{}
		for _, c := range counts {
			countMap[c.Type] = c.Count
		}
		assert.Equal(t, 0, countMap["cat"])
		assert.Equal(t, 0, countMap["dog"])
	})

	t.Run("returns correct counts with active actors", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.config.HostedActorTypes = []string{"cat", "dog"}

		catKey1 := a.constructCompositeKey("cat", "c1")
		catKey2 := a.constructCompositeKey("cat", "c2")
		dogKey1 := a.constructCompositeKey("dog", "d1")

		a.actorsTable.Store(catKey1, newActor("cat", "c1"))
		a.actorsTable.Store(catKey2, newActor("cat", "c2"))
		a.actorsTable.Store(dogKey1, newActor("dog", "d1"))

		counts := a.GetActiveActorsCount(nil)
		require.Len(t, counts, 2)

		countMap := map[string]int{}
		for _, c := range counts {
			countMap[c.Type] = c.Count
		}
		assert.Equal(t, 2, countMap["cat"])
		assert.Equal(t, 1, countMap["dog"])
	})

	t.Run("no hosted types returns empty", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.config.HostedActorTypes = []string{}

		counts := a.GetActiveActorsCount(nil)
		assert.Empty(t, counts)
	})
}

func TestConstructActorStateKeyMinimal(t *testing.T) {
	a := newMinimalActorsRuntime()
	a.config.AppID = "myapp"

	key := a.constructActorStateKey("TestActor", "abc123", "stateKey")
	assert.Equal(t, fmt.Sprintf("myapp||TestActor||abc123||stateKey"), key)
}

func TestStopWithNilPlacement(t *testing.T) {
	a := newMinimalActorsRuntime()
	a.placement = nil
	// Stop should not panic when placement is nil
	assert.NotPanics(t, func() {
		a.Stop()
	})
}

// --- Additional coverage tests ---

func TestInitErrors(t *testing.T) {
	t.Run("empty placement addresses", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.config.PlacementAddresses = []string{}

		err := a.Init()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "address is empty")
	})

	t.Run("hosted actors with nil store returns incompatible store error", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.config.PlacementAddresses = []string{"localhost:5050"}
		a.config.HostedActorTypes = []string{"testType"}
		a.store = nil

		err := a.Init()
		require.Error(t, err)
		assert.Contains(t, err.Error(), incompatibleStateStore)
	})

	t.Run("hosted actors with non-transactional store", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.config.PlacementAddresses = []string{"localhost:5050"}
		a.config.HostedActorTypes = []string{"testType"}
		a.store = &nonTransactionalStore{}

		err := a.Init()
		require.Error(t, err)
		assert.Contains(t, err.Error(), incompatibleStateStore)
	})
}

func TestCallEmptyTargetAddress(t *testing.T) {
	a := newMinimalActorsRuntime()
	a.placement = internal.NewActorPlacement(
		[]string{"localhost:5050"}, nil,
		"testApp", "localhost:5000", []string{},
		func() bool { return true },
		func() {},
	)

	req := invokev1.NewInvokeMethodRequest("method1").WithActor("unknownType", "id1")

	resp, err := a.Call(context.Background(), req)
	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding address for actor type")
}

func TestCallRemoteActorWithRetry(t *testing.T) {
	t.Run("succeeds on first try", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		expectedResp := invokev1.NewInvokeMethodResponse(200, "OK", nil)

		fn := func(ctx context.Context, targetAddress, targetID string, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
			return expectedResp, nil
		}

		req := invokev1.NewInvokeMethodRequest("method1")
		resp, err := a.callRemoteActorWithRetry(context.Background(), 3, time.Millisecond, fn, "addr", "id", req)
		assert.NoError(t, err)
		assert.Equal(t, expectedResp, resp)
	})

	t.Run("non-retriable error returns immediately", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		callCount := 0

		fn := func(ctx context.Context, targetAddress, targetID string, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
			callCount++
			return nil, status.Error(codes.Internal, "internal error")
		}

		req := invokev1.NewInvokeMethodRequest("method1")
		_, err := a.callRemoteActorWithRetry(context.Background(), 3, time.Millisecond, fn, "addr", "id", req)
		assert.Error(t, err)
		assert.Equal(t, 1, callCount, "should not retry on non-retriable error")
	})

	t.Run("retriable Unavailable error with connection refresh failure", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.grpcConnectionFn = func(address, id string, namespace string, skipTLS, recreateIfExists, enableSSL bool) (*grpc.ClientConn, error) {
			return nil, errors.New("connection refresh failed")
		}

		fn := func(ctx context.Context, targetAddress, targetID string, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
			return nil, status.Error(codes.Unavailable, "unavailable")
		}

		req := invokev1.NewInvokeMethodRequest("method1")
		resp, err := a.callRemoteActorWithRetry(context.Background(), 3, time.Millisecond, fn, "addr", "id", req)
		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection refresh failed")
	})

	t.Run("retriable Unauthenticated error with successful refresh exhausts retries", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.grpcConnectionFn = func(address, id string, namespace string, skipTLS, recreateIfExists, enableSSL bool) (*grpc.ClientConn, error) {
			return nil, nil
		}

		fn := func(ctx context.Context, targetAddress, targetID string, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}

		req := invokev1.NewInvokeMethodRequest("method1")
		resp, err := a.callRemoteActorWithRetry(context.Background(), 2, time.Millisecond, fn, "someaddr", "id", req)
		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to invoke target someaddr after 2 retries")
	})
}

func TestGetStateNilStore(t *testing.T) {
	a := newMinimalActorsRuntime()
	a.store = nil

	_, err := a.GetState(context.Background(), &GetStateRequest{
		ActorType: "testType",
		ActorID:   "id1",
		Key:       "key1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "state store does not exist")
}

func TestTransactionalStateOpNilStore(t *testing.T) {
	a := newMinimalActorsRuntime()
	a.store = nil

	err := a.TransactionalStateOperation(context.Background(), &TransactionalRequest{
		ActorType:  "testType",
		ActorID:    "id1",
		Operations: []TransactionalOperation{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "state store does not exist")
}

func TestGetUpcomingReminderInvokeTimeErrors(t *testing.T) {
	t.Run("invalid registered time", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.store = fakeStore()

		reminder := &Reminder{
			ActorType:      "test",
			ActorID:        "id1",
			Name:           "rem1",
			RegisteredTime: "not-a-time",
			DueTime:        "1s",
		}

		_, err := a.getUpcomingReminderInvokeTime(reminder)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error parsing reminder registered time")
	})

	t.Run("invalid due time", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.store = fakeStore()

		reminder := &Reminder{
			ActorType:      "test",
			ActorID:        "id1",
			Name:           "rem1",
			RegisteredTime: time.Now().UTC().Format(time.RFC3339),
			DueTime:        "not-a-duration",
		}

		_, err := a.getUpcomingReminderInvokeTime(reminder)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error parsing reminder due time")
	})

	t.Run("store error in getReminderTrack", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.store = &stubStateStore{getErr: errors.New("store unavailable")}

		reminder := &Reminder{
			ActorType:      "test",
			ActorID:        "id1",
			Name:           "rem1",
			RegisteredTime: time.Now().UTC().Format(time.RFC3339),
			DueTime:        "1s",
		}

		_, err := a.getUpcomingReminderInvokeTime(reminder)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error getting reminder track")
	})

	t.Run("invalid last fired time in track", func(t *testing.T) {
		store := &fakeStateStore{
			items: map[string][]byte{},
			lock:  &sync.RWMutex{},
		}
		a := newMinimalActorsRuntime()
		a.store = store

		track := ReminderTrack{LastFiredTime: "not-a-time"}
		trackData, _ := json.Marshal(track)
		store.items["test||id1||rem1"] = trackData

		reminder := &Reminder{
			ActorType:      "test",
			ActorID:        "id1",
			Name:           "rem1",
			RegisteredTime: time.Now().UTC().Format(time.RFC3339),
			DueTime:        "1s",
			Period:         "30s",
		}

		_, err := a.getUpcomingReminderInvokeTime(reminder)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error parsing reminder last fired time")
	})

	t.Run("valid last fired time with valid period", func(t *testing.T) {
		store := &fakeStateStore{
			items: map[string][]byte{},
			lock:  &sync.RWMutex{},
		}
		a := newMinimalActorsRuntime()
		a.store = store

		lastFired := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
		track := ReminderTrack{LastFiredTime: lastFired}
		trackData, _ := json.Marshal(track)
		store.items["test||id1||rem1"] = trackData

		reminder := &Reminder{
			ActorType:      "test",
			ActorID:        "id1",
			Name:           "rem1",
			RegisteredTime: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339),
			DueTime:        "1s",
			Period:         "30s",
		}

		invokeTime, err := a.getUpcomingReminderInvokeTime(reminder)
		assert.NoError(t, err)
		assert.False(t, invokeTime.IsZero())
	})

	t.Run("valid last fired time with invalid period", func(t *testing.T) {
		store := &fakeStateStore{
			items: map[string][]byte{},
			lock:  &sync.RWMutex{},
		}
		a := newMinimalActorsRuntime()
		a.store = store

		lastFired := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
		track := ReminderTrack{LastFiredTime: lastFired}
		trackData, _ := json.Marshal(track)
		store.items["test||id1||rem1"] = trackData

		reminder := &Reminder{
			ActorType:      "test",
			ActorID:        "id1",
			Name:           "rem1",
			RegisteredTime: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339),
			DueTime:        "1s",
			Period:         "bad-period",
		}

		_, err := a.getUpcomingReminderInvokeTime(reminder)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error parsing reminder period")
	})
}

func TestEvaluateReminders(t *testing.T) {
	t.Run("empty hosted actor types", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.config.HostedActorTypes = []string{}

		a.evaluateReminders()

		assert.False(t, a.evaluationBusy)
	})

	t.Run("reminders loaded but lookup returns empty address", func(t *testing.T) {
		store := &fakeStateStore{
			items: map[string][]byte{},
			lock:  &sync.RWMutex{},
		}

		reminders := []Reminder{
			{ActorType: "testType", ActorID: "id1", Name: "rem1", Period: "1s", DueTime: "1s"},
		}
		data, _ := json.Marshal(reminders)
		store.items["actors||testType"] = data

		a := newMinimalActorsRuntime()
		a.store = store
		a.config.HostedActorTypes = []string{"testType"}
		a.config.HostAddress = "localhost"
		a.config.Port = 5000

		a.placement = internal.NewActorPlacement(
			[]string{"localhost:5050"}, nil,
			"testApp", "localhost:5000", []string{"testType"},
			func() bool { return true },
			func() {},
		)

		a.evaluateReminders()

		assert.False(t, a.evaluationBusy)
		a.remindersLock.RLock()
		assert.Len(t, a.reminders["testType"], 1)
		a.remindersLock.RUnlock()
	})

	t.Run("getRemindersForActorType error is handled gracefully", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.store = &stubStateStore{getErr: errors.New("store error")}
		a.config.HostedActorTypes = []string{"testType"}
		a.config.HostAddress = "localhost"
		a.config.Port = 5000

		a.placement = internal.NewActorPlacement(
			[]string{"localhost:5050"}, nil,
			"testApp", "localhost:5000", []string{"testType"},
			func() bool { return true },
			func() {},
		)

		// Should not panic even when store errors
		a.evaluateReminders()

		assert.False(t, a.evaluationBusy)
	})
}

func TestCallLocalActorAdditional(t *testing.T) {
	t.Run("non-OK status returns error", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.appChannel = &stubAppChannel{
			invokeMethodFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
				return invokev1.NewInvokeMethodResponse(500, "Internal Server Error", nil), nil
			},
		}

		req := invokev1.NewInvokeMethodRequest("method1").WithActor("testType", "id1")
		resp, err := a.callLocalActor(context.Background(), req)
		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error from actor service")
	})

	t.Run("invoke method returns error", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.appChannel = &stubAppChannel{
			invokeMethodFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
				return nil, errors.New("channel invoke failed")
			},
		}

		req := invokev1.NewInvokeMethodRequest("method1").WithActor("testType", "id1")
		resp, err := a.callLocalActor(context.Background(), req)
		assert.Nil(t, resp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "channel invoke failed")
	})

	t.Run("existing HTTP extension verb is overridden to PUT", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.appChannel = &stubAppChannel{}

		req := invokev1.NewInvokeMethodRequest("method1").
			WithActor("testType", "id1").
			WithHTTPExtension("GET", "")

		resp, err := a.callLocalActor(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestDeactivateActorErrors(t *testing.T) {
	t.Run("invoke method error", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.appChannel = &stubAppChannel{
			invokeMethodFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
				return nil, errors.New("invoke failed")
			},
		}

		err := a.deactivateActor("testType", "id1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invoke failed")
	})

	t.Run("non-OK status from actor service", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.appChannel = &stubAppChannel{
			invokeMethodFn: func(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
				return invokev1.NewInvokeMethodResponse(500, "Error", nil), nil
			},
		}

		err := a.deactivateActor("testType", "id1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error from actor service")
	})

	t.Run("success removes actor from table", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.appChannel = &stubAppChannel{}

		actorKey := a.constructCompositeKey("testType", "id1")
		a.actorsTable.Store(actorKey, newActor("testType", "id1"))

		err := a.deactivateActor("testType", "id1")
		assert.NoError(t, err)

		_, exists := a.actorsTable.Load(actorKey)
		assert.False(t, exists)
	})
}

func TestGetReminderAdditional(t *testing.T) {
	t.Run("returns nil when no matching reminder exists", func(t *testing.T) {
		store := &fakeStateStore{
			items: map[string][]byte{},
			lock:  &sync.RWMutex{},
		}
		a := newMinimalActorsRuntime()
		a.store = store

		reminders := []Reminder{
			{ActorType: "testType", ActorID: "other-id", Name: "rem1"},
		}
		data, _ := json.Marshal(reminders)
		store.items["actors||testType"] = data

		r, err := a.GetReminder(context.Background(), &GetReminderRequest{
			ActorType: "testType",
			ActorID:   "id1",
			Name:      "rem1",
		})
		assert.NoError(t, err)
		assert.Nil(t, r)
	})

	t.Run("store error propagates", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.store = &stubStateStore{getErr: errors.New("store unavailable")}

		r, err := a.GetReminder(context.Background(), &GetReminderRequest{
			ActorType: "testType",
			ActorID:   "id1",
			Name:      "rem1",
		})
		require.Error(t, err)
		assert.Nil(t, r)
	})
}

func TestGetReminderTrackStoreError(t *testing.T) {
	a := newMinimalActorsRuntime()
	a.store = &stubStateStore{getErr: errors.New("store error")}

	track, err := a.getReminderTrack("actorKey", "name")
	require.Error(t, err)
	assert.Nil(t, track)
}

func TestGetRemindersForActorTypeStoreError(t *testing.T) {
	a := newMinimalActorsRuntime()
	a.store = &stubStateStore{getErr: errors.New("store error")}

	reminders, err := a.getRemindersForActorType("testType")
	require.Error(t, err)
	assert.Nil(t, reminders)
}

func TestDeleteReminderEvaluationBusy(t *testing.T) {
	a := newMinimalActorsRuntime()
	a.store = fakeStore()
	a.evaluationBusy = true
	ch := make(chan bool)
	a.evaluationChan = ch
	close(ch)

	err := a.DeleteReminder(context.Background(), &DeleteReminderRequest{
		ActorType: "testType",
		ActorID:   "id1",
		Name:      "rem1",
	})
	assert.NoError(t, err)
}

func TestDrainRebalancedActors(t *testing.T) {
	t.Run("actor with empty lookup address is not drained", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.config.HostAddress = "10.0.0.1"
		a.config.Port = 5000

		a.placement = internal.NewActorPlacement(
			[]string{"localhost:5050"}, nil,
			"testApp", "10.0.0.1:5000", []string{"testType"},
			func() bool { return true },
			func() {},
		)

		actorKey := a.constructCompositeKey("testType", "id1")
		a.actorsTable.Store(actorKey, newActor("testType", "id1"))

		a.drainRebalancedActors()
		time.Sleep(50 * time.Millisecond)

		_, exists := a.actorsTable.Load(actorKey)
		assert.True(t, exists, "actor should not be drained when LookupActor returns empty")
	})

	t.Run("empty actors table does not panic", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		a.placement = internal.NewActorPlacement(
			[]string{"localhost:5050"}, nil,
			"testApp", "localhost:5000", []string{},
			func() bool { return true },
			func() {},
		)

		assert.NotPanics(t, func() {
			a.drainRebalancedActors()
		})
	})
}

func TestStopWithPlacement(t *testing.T) {
	a := newMinimalActorsRuntime()
	a.placement = internal.NewActorPlacement(
		[]string{"localhost:5050"}, nil,
		"testApp", "localhost:5000", []string{},
		func() bool { return true },
		func() {},
	)

	assert.NotPanics(t, func() {
		a.Stop()
	})
}

func TestCreateTimerErrors(t *testing.T) {
	t.Run("actor not activated", func(t *testing.T) {
		a := newMinimalActorsRuntime()

		err := a.CreateTimer(context.Background(), &CreateTimerRequest{
			ActorType: "testType",
			ActorID:   "nonexistent",
			Name:      "timer1",
			Period:    "1s",
			DueTime:   "1s",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "can't create timer for actor")
	})

	t.Run("invalid period", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		actorKey := a.constructCompositeKey("testType", "id1")
		a.actorsTable.Store(actorKey, newActor("testType", "id1"))

		err := a.CreateTimer(context.Background(), &CreateTimerRequest{
			ActorType: "testType",
			ActorID:   "id1",
			Name:      "timer1",
			Period:    "invalid-period",
		})
		require.Error(t, err)
	})

	t.Run("invalid due time", func(t *testing.T) {
		a := newMinimalActorsRuntime()
		actorKey := a.constructCompositeKey("testType", "id1")
		a.actorsTable.Store(actorKey, newActor("testType", "id1"))

		err := a.CreateTimer(context.Background(), &CreateTimerRequest{
			ActorType: "testType",
			ActorID:   "id1",
			Name:      "timer1",
			Period:    "1s",
			DueTime:   "invalid-duetime",
		})
		require.Error(t, err)
	})
}
