// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package raft

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func TestFSMApply(t *testing.T) {
	fsm := newFSM()

	t.Run("upsertMember", func(t *testing.T) {
		cmdLog, err := makeRaftLogCommand(MemberUpsert, DaprHostMember{
			Name:     "127.0.0.1:3030",
			AppID:    "fakeAppID",
			Entities: []string{"actorTypeOne", "actorTypeTwo"},
		})

		assert.NoError(t, err)

		raftLog := &raft.Log{
			Index: 1,
			Term:  1,
			Type:  raft.LogCommand,
			Data:  cmdLog,
		}

		resp := fsm.Apply(raftLog)
		updated, ok := resp.(bool)

		assert.True(t, ok)
		assert.True(t, updated)
		assert.Equal(t, uint64(1), fsm.state.TableGeneration)
		assert.Equal(t, 1, len(fsm.state.Members))
	})

	t.Run("removeMember", func(t *testing.T) {
		cmdLog, err := makeRaftLogCommand(MemberRemove, DaprHostMember{
			Name: "127.0.0.1:3030",
		})

		assert.NoError(t, err)

		raftLog := &raft.Log{
			Index: 2,
			Term:  1,
			Type:  raft.LogCommand,
			Data:  cmdLog,
		}

		resp := fsm.Apply(raftLog)
		updated, ok := resp.(bool)

		assert.True(t, ok)
		assert.True(t, updated)
		assert.Equal(t, uint64(2), fsm.state.TableGeneration)
		assert.Equal(t, 0, len(fsm.state.Members))
	})
}

func TestFSMApplyOldLogIndex(t *testing.T) {
	fsm := newFSM()

	// Set the state index to a value higher than the log we will apply
	fsm.state.Index = 10

	cmdLog, err := makeRaftLogCommand(MemberUpsert, DaprHostMember{
		Name:     "127.0.0.1:3031",
		AppID:    "anotherApp",
		Entities: []string{"actorTypeTwo"},
	})
	assert.NoError(t, err)

	// Apply a log with an index lower than state.Index, which should be skipped
	resp := fsm.Apply(&raft.Log{
		Index: 3,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  cmdLog,
	})

	// The old log should be skipped and return false
	skipped, ok := resp.(bool)
	assert.True(t, ok)
	assert.False(t, skipped)

	// No members should have been added
	assert.Equal(t, 0, len(fsm.state.Members))
}

func TestFSMApplyUnknownCommandType(t *testing.T) {
	fsm := newFSM()

	// Create a log with an unknown command type (e.g., 255)
	data := []byte{255, 0x01, 0x02}
	resp := fsm.Apply(&raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  data,
	})

	// Unknown command type should return false
	result, ok := resp.(bool)
	assert.True(t, ok)
	assert.False(t, result)
}

func TestFSMUpsertMemberInvalidData(t *testing.T) {
	fsm := newFSM()

	// Create a log entry with MemberUpsert command type but invalid msgpack data
	data := make([]byte, 3)
	data[0] = uint8(MemberUpsert)
	data[1] = 0xFF // Invalid msgpack data
	data[2] = 0xFF

	resp := fsm.Apply(&raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  data,
	})

	// Invalid data should cause an error, and Apply should return false
	result, ok := resp.(bool)
	assert.True(t, ok)
	assert.False(t, result)
}

func TestFSMRemoveMemberInvalidData(t *testing.T) {
	fsm := newFSM()

	// Create a log entry with MemberRemove command type but invalid msgpack data
	data := make([]byte, 3)
	data[0] = uint8(MemberRemove)
	data[1] = 0xFF // Invalid msgpack data
	data[2] = 0xFF

	resp := fsm.Apply(&raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  data,
	})

	// Invalid data should cause an error, and Apply should return false
	result, ok := resp.(bool)
	assert.True(t, ok)
	assert.False(t, result)
}

func TestRestore(t *testing.T) {
	// arrange
	fsm := newFSM()

	s := newDaprHostMemberState()
	s.upsertMember(&DaprHostMember{
		Name:     "127.0.0.1:8080",
		AppID:    "FakeID",
		Entities: []string{"actorTypeOne", "actorTypeTwo"},
	})
	data, err := marshalMsgPack(s)
	assert.NoError(t, err)
	buf := ioutil.NopCloser(bytes.NewBuffer(data))

	// act
	err = fsm.Restore(buf)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 1, len(fsm.State().Members))
	assert.Equal(t, 2, len(fsm.State().hashingTableMap))
}

func TestPlacementState(t *testing.T) {
	fsm := newFSM()
	m := DaprHostMember{
		Name:     "127.0.0.1:3030",
		AppID:    "fakeAppID",
		Entities: []string{"actorTypeOne", "actorTypeTwo"},
	}
	cmdLog, err := makeRaftLogCommand(MemberUpsert, m)
	assert.NoError(t, err)

	fsm.Apply(&raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  cmdLog,
	})

	newTable := fsm.PlacementState()
	assert.Equal(t, "1", newTable.Version)
	assert.Equal(t, 2, len(newTable.Entries))
}
