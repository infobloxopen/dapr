// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package raft

import (
	"bytes"
	"errors"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

type MockSnapShotSink struct {
	*bytes.Buffer
	cancel bool
}

func (m *MockSnapShotSink) ID() string {
	return "Mock"
}

func (m *MockSnapShotSink) Cancel() error {
	m.cancel = true
	return nil
}

func (m *MockSnapShotSink) Close() error {
	return nil
}

// errorSnapShotSink is a mock sink that returns an error on Write.
type errorSnapShotSink struct {
	cancel bool
}

func (e *errorSnapShotSink) Write(p []byte) (int, error) {
	return 0, errors.New("write error")
}

func (e *errorSnapShotSink) ID() string {
	return "ErrorMock"
}

func (e *errorSnapShotSink) Cancel() error {
	e.cancel = true
	return nil
}

func (e *errorSnapShotSink) Close() error {
	return nil
}

func TestRelease(t *testing.T) {
	fsm := newFSM()
	snap, err := fsm.Snapshot()
	assert.NoError(t, err)

	// Release is a no-op, just verify it doesn't panic
	assert.NotPanics(t, func() {
		snap.Release()
	})
}

func TestPersistWriteError(t *testing.T) {
	// arrange
	fsm := newFSM()
	testMember := DaprHostMember{
		Name:     "127.0.0.1:3030",
		AppID:    "fakeAppID",
		Entities: []string{"actorTypeOne", "actorTypeTwo"},
	}
	cmdLog, _ := makeRaftLogCommand(MemberUpsert, testMember)
	raftLog := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  cmdLog,
	}
	fsm.Apply(raftLog)

	// act
	snap, err := fsm.Snapshot()
	assert.NoError(t, err)

	errSink := &errorSnapShotSink{}
	err = snap.Persist(errSink)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write error")
	assert.True(t, errSink.cancel, "Cancel should be called when Write fails")
}

func TestPersist(t *testing.T) {
	// arrange
	fsm := newFSM()
	testMember := DaprHostMember{
		Name:     "127.0.0.1:3030",
		AppID:    "fakeAppID",
		Entities: []string{"actorTypeOne", "actorTypeTwo"},
	}
	cmdLog, _ := makeRaftLogCommand(MemberUpsert, testMember)
	raftLog := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  cmdLog,
	}
	fsm.Apply(raftLog)
	buf := bytes.NewBuffer(nil)
	fakeSink := &MockSnapShotSink{buf, false}

	// act
	snap, err := fsm.Snapshot()
	assert.NoError(t, err)
	snap.Persist(fakeSink)

	// assert
	restoredState := &DaprHostMemberState{}
	err = unmarshalMsgPack(buf.Bytes(), restoredState)
	assert.NoError(t, err)

	expectedMember := fsm.State().Members[testMember.Name]
	restoredMember := restoredState.Members[testMember.Name]
	assert.Equal(t, fsm.State().Index, restoredState.Index)
	assert.Equal(t, expectedMember.Name, restoredMember.Name)
	assert.Equal(t, expectedMember.AppID, restoredMember.AppID)
	assert.EqualValues(t, expectedMember.Entities, restoredMember.Entities)
}
