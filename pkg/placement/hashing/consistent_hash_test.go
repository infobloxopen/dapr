// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package hashing

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var nodes = []string{"node1", "node2", "node3", "node4", "node5"}

func TestReplicationFactor(t *testing.T) {
	keys := []string{}
	for i := 0; i < 100; i++ {
		keys = append(keys, fmt.Sprint(i))
	}

	t.Run("varying replication factors, no movement", func(t *testing.T) {
		factors := []int{1, 100, 1000, 10000}

		for _, f := range factors {
			SetReplicationFactor(f)

			h := NewConsistentHash()
			for _, n := range nodes {
				s := h.Add(n, n, 1)
				assert.False(t, s)
			}

			k1 := map[string]string{}

			for _, k := range keys {
				h, err := h.Get(k)
				assert.NoError(t, err)

				k1[k] = h
			}

			nodeToRemove := "node3"
			h.Remove(nodeToRemove)

			for _, k := range keys {
				h, err := h.Get(k)
				assert.NoError(t, err)

				orgS := k1[k]
				if orgS != nodeToRemove {
					assert.Equal(t, h, orgS)
				}
			}
		}
	})
}

func TestSetReplicationFactor(t *testing.T) {
	f := 10
	SetReplicationFactor(f)

	assert.Equal(t, f, replicationFactor)
}

func TestNewPlacementTables(t *testing.T) {
	SetReplicationFactor(10)

	tests := []struct {
		name    string
		version string
		entries map[string]*Consistent
	}{
		{
			name:    "with nil entries",
			version: "v1",
			entries: nil,
		},
		{
			name:    "with empty entries",
			version: "v2",
			entries: map[string]*Consistent{},
		},
		{
			name:    "with populated entries",
			version: "v3",
			entries: map[string]*Consistent{"app1": NewConsistentHash()},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tables := NewPlacementTables(tc.version, tc.entries)
			require.NotNil(t, tables)
			assert.Equal(t, tc.version, tables.Version)
			assert.Equal(t, tc.entries, tables.Entries)
		})
	}
}

func TestNewHost(t *testing.T) {
	SetReplicationFactor(10)

	tests := []struct {
		name     string
		hostName string
		id       string
		load     int64
		port     int64
	}{
		{
			name:     "basic host",
			hostName: "host1",
			id:       "app1",
			load:     0,
			port:     3000,
		},
		{
			name:     "host with load",
			hostName: "host2",
			id:       "app2",
			load:     42,
			port:     5000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHost(tc.hostName, tc.id, tc.load, tc.port)
			require.NotNil(t, h)
			assert.Equal(t, tc.hostName, h.Name)
			assert.Equal(t, tc.id, h.AppID)
			assert.Equal(t, tc.load, h.Load)
			assert.Equal(t, tc.port, h.Port)
		})
	}
}

func TestNewFromExisting(t *testing.T) {
	SetReplicationFactor(10)

	hosts := map[uint64]string{
		100: "host1",
		200: "host2",
	}
	sortedSet := []uint64{100, 200}
	loadMap := map[string]*Host{
		"host1": {Name: "host1", AppID: "app1", Load: 1, Port: 3000},
		"host2": {Name: "host2", AppID: "app2", Load: 2, Port: 4000},
	}

	c := NewFromExisting(hosts, sortedSet, loadMap)
	require.NotNil(t, c)

	gotHosts, gotSortedSet, gotLoadMap, gotTotalLoad := c.GetInternals()
	assert.Equal(t, hosts, gotHosts)
	assert.Equal(t, sortedSet, gotSortedSet)
	assert.Equal(t, loadMap, gotLoadMap)
	assert.Equal(t, int64(0), gotTotalLoad) // totalLoad is not set by NewFromExisting
}

func TestGetLeast(t *testing.T) {
	SetReplicationFactor(10)

	t.Run("returns least loaded host", func(t *testing.T) {
		c := NewConsistentHash()
		c.Add("host1", "app1", 3000)
		c.Add("host2", "app2", 3001)
		c.Add("host3", "app3", 3002)

		// Make host1 and host3 heavily loaded
		for i := 0; i < 100; i++ {
			c.Inc("host1")
			c.Inc("host3")
		}

		// Try many keys — GetLeast should avoid overloaded hosts
		leastCounts := map[string]int{}
		for i := 0; i < 50; i++ {
			h, err := c.GetLeast(fmt.Sprintf("key-%d", i))
			require.NoError(t, err)
			leastCounts[h]++
		}

		// host2 has load 0, so it should be preferred
		assert.Greater(t, leastCounts["host2"], 0, "least loaded host should be selected at least once")
	})

	t.Run("empty ring returns error", func(t *testing.T) {
		c := NewConsistentHash()
		_, err := c.GetLeast("somekey")
		assert.Equal(t, ErrNoHosts, err)
	})
}

func TestUpdateLoad(t *testing.T) {
	SetReplicationFactor(10)

	tests := []struct {
		name        string
		hosts       []string
		targetHost  string
		load        int64
		expectInMap bool
	}{
		{
			name:        "set load on existing host",
			hosts:       []string{"host1", "host2"},
			targetHost:  "host1",
			load:        99,
			expectInMap: true,
		},
		{
			name:        "set load on unknown host is no-op",
			hosts:       []string{"host1"},
			targetHost:  "unknown",
			load:        50,
			expectInMap: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewConsistentHash()
			for _, h := range tc.hosts {
				c.Add(h, h, 3000)
			}

			c.UpdateLoad(tc.targetHost, tc.load)

			loads := c.GetLoads()
			if tc.expectInMap {
				assert.Equal(t, tc.load, loads[tc.targetHost])
			} else {
				_, exists := loads[tc.targetHost]
				assert.False(t, exists)
			}
		})
	}
}

func TestInc(t *testing.T) {
	SetReplicationFactor(10)

	t.Run("increments host load", func(t *testing.T) {
		c := NewConsistentHash()
		c.Add("host1", "app1", 3000)

		loads := c.GetLoads()
		assert.Equal(t, int64(0), loads["host1"])

		c.Inc("host1")
		loads = c.GetLoads()
		assert.Equal(t, int64(1), loads["host1"])

		c.Inc("host1")
		c.Inc("host1")
		loads = c.GetLoads()
		assert.Equal(t, int64(3), loads["host1"])

		_, _, _, totalLoad := c.GetInternals()
		assert.Equal(t, int64(3), totalLoad)
	})
}

func TestDone(t *testing.T) {
	SetReplicationFactor(10)

	t.Run("decrements host load", func(t *testing.T) {
		c := NewConsistentHash()
		c.Add("host1", "app1", 3000)

		c.Inc("host1")
		c.Inc("host1")
		c.Inc("host1")

		loads := c.GetLoads()
		assert.Equal(t, int64(3), loads["host1"])

		c.Done("host1")
		loads = c.GetLoads()
		assert.Equal(t, int64(2), loads["host1"])

		_, _, _, totalLoad := c.GetInternals()
		assert.Equal(t, int64(2), totalLoad)
	})

	t.Run("unknown host is no-op", func(t *testing.T) {
		c := NewConsistentHash()
		c.Add("host1", "app1", 3000)
		c.Inc("host1")

		_, _, _, totalBefore := c.GetInternals()

		c.Done("nonexistent")

		_, _, _, totalAfter := c.GetInternals()
		assert.Equal(t, totalBefore, totalAfter)
	})
}

func TestGetLoads(t *testing.T) {
	SetReplicationFactor(10)

	t.Run("returns loads for all hosts", func(t *testing.T) {
		c := NewConsistentHash()
		c.Add("host1", "app1", 3000)
		c.Add("host2", "app2", 3001)
		c.Add("host3", "app3", 3002)

		c.UpdateLoad("host1", 10)
		c.UpdateLoad("host2", 20)
		// host3 stays at 0

		loads := c.GetLoads()
		assert.Len(t, loads, 3)
		assert.Equal(t, int64(10), loads["host1"])
		assert.Equal(t, int64(20), loads["host2"])
		assert.Equal(t, int64(0), loads["host3"])
	})

	t.Run("empty ring returns empty map", func(t *testing.T) {
		c := NewConsistentHash()
		loads := c.GetLoads()
		assert.Empty(t, loads)
	})
}

func TestMaxLoad(t *testing.T) {
	SetReplicationFactor(10)

	t.Run("with zero total load", func(t *testing.T) {
		c := NewConsistentHash()
		c.Add("host1", "app1", 3000)
		c.Add("host2", "app2", 3001)

		// totalLoad is 0, MaxLoad sets it to 1 internally
		// avgLoadPerNode = floor(1/2) = 0 -> set to 1
		// ceil(1 * 1.25) = ceil(1.25) = 2
		ml := c.MaxLoad()
		assert.Equal(t, int64(2), ml)
	})

	t.Run("with non-zero total load", func(t *testing.T) {
		c := NewConsistentHash()
		c.Add("host1", "app1", 3000)
		c.Add("host2", "app2", 3001)

		c.UpdateLoad("host1", 10)
		c.UpdateLoad("host2", 10)
		// totalLoad = 20, numHosts = 2
		// avgLoadPerNode = floor(20/2) = 10
		// ceil(10 * 1.25) = ceil(12.5) = 13
		ml := c.MaxLoad()
		assert.Equal(t, int64(13), ml)
	})
}

func TestLoadOK(t *testing.T) {
	SetReplicationFactor(10)

	t.Run("negative totalLoad returns true", func(t *testing.T) {
		c := NewConsistentHash()
		c.Add("host1", "app1", 3000)

		// Force totalLoad to a negative value (simulating excessive Done calls)
		// We do this via Done on a host with zero load
		c.Done("host1")
		_, _, _, totalLoad := c.GetInternals()
		assert.True(t, totalLoad < 0, "totalLoad should be negative, got %d", totalLoad)

		// loadOK should still return true because the safety check resets totalLoad
		ok := c.loadOK("host1")
		assert.True(t, ok)
	})

	t.Run("load within threshold returns true", func(t *testing.T) {
		c := NewConsistentHash()
		c.Add("host1", "app1", 3000)
		c.Add("host2", "app2", 3001)

		// Both hosts at load 0, totalLoad = 0
		// loadOK computes: (0+1)/2 = 0 -> set to 1, ceil(1*1.25) = 2
		// host load is 0, 0+1=1 <= 2 -> true
		ok := c.loadOK("host1")
		assert.True(t, ok)
	})

	t.Run("load exceeds threshold returns false", func(t *testing.T) {
		c := NewConsistentHash()
		c.Add("host1", "app1", 3000)
		c.Add("host2", "app2", 3001)

		// Put heavy load on host1
		// After many increments, host1's load will exceed the threshold
		for i := 0; i < 50; i++ {
			c.Inc("host1")
		}
		// totalLoad = 50, numHosts = 2
		// loadOK computes: (50+1)/2 = 25, ceil(25*1.25) = ceil(31.25) = 32
		// host1.Load = 50, 50+1=51 > 32 -> false
		ok := c.loadOK("host1")
		assert.False(t, ok)
	})
}

func TestGetHost(t *testing.T) {
	SetReplicationFactor(10)

	t.Run("returns host struct", func(t *testing.T) {
		c := NewConsistentHash()
		c.Add("host1", "app1", 3000)

		host, err := c.GetHost("somekey")
		require.NoError(t, err)
		require.NotNil(t, host)
		assert.Equal(t, "host1", host.Name)
		assert.Equal(t, "app1", host.AppID)
		assert.Equal(t, int64(3000), host.Port)
		assert.Equal(t, int64(0), host.Load)
	})

	t.Run("empty ring returns error", func(t *testing.T) {
		c := NewConsistentHash()
		host, err := c.GetHost("somekey")
		assert.Nil(t, host)
		assert.Equal(t, ErrNoHosts, err)
	})
}
