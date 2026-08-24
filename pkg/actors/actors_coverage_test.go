// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package actors

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
