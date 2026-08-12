// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package raft

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRaftStorePath(t *testing.T) {
	tests := []struct {
		name             string
		id               string
		raftLogStorePath string
		expected         string
	}{
		{
			name:             "empty raftLogStorePath returns default with prefix",
			id:               "node0",
			raftLogStorePath: "",
			expected:         "log-node0",
		},
		{
			name:             "non-empty raftLogStorePath returns the path",
			id:               "node0",
			raftLogStorePath: "/custom/path",
			expected:         "/custom/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				id:               tt.id,
				raftLogStorePath: tt.raftLogStorePath,
			}
			assert.Equal(t, tt.expected, s.raftStorePath())
		})
	}
}

func TestNewServer(t *testing.T) {
	peers := []PeerInfo{
		{ID: "node0", Address: "127.0.0.1:3030"},
		{ID: "node1", Address: "127.0.0.1:3031"},
		{ID: "node2", Address: "127.0.0.1:3032"},
	}

	t.Run("id found in peers returns server", func(t *testing.T) {
		srv := New("node0", true, peers, "")
		assert.NotNil(t, srv)
		assert.Equal(t, "node0", srv.id)
		assert.Equal(t, true, srv.inMem)
		assert.Equal(t, "127.0.0.1:3030", srv.raftBind)
		assert.Equal(t, peers, srv.peers)
	})

	t.Run("id not in peers returns nil", func(t *testing.T) {
		srv := New("nodeX", true, peers, "")
		assert.Nil(t, srv)
	})

	t.Run("logStorePath is set", func(t *testing.T) {
		srv := New("node1", false, peers, "/tmp/raft-logs")
		assert.NotNil(t, srv)
		assert.Equal(t, "/tmp/raft-logs", srv.raftLogStorePath)
		assert.Equal(t, false, srv.inMem)
	})
}
