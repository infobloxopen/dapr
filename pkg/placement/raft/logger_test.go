// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package raft

import (
	"io"
	"log"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestNewLoggerAdapter(t *testing.T) {
	l := newLoggerAdapter()
	assert.NotNil(t, l)
}

func TestLoggerAdapterLogMethods(t *testing.T) {
	l := &loggerAdapter{}

	tests := []struct {
		name string
		fn   func()
	}{
		{"Log_Debug", func() { l.Log(hclog.Debug, "debug msg %s", "arg1") }},
		{"Log_Warn", func() { l.Log(hclog.Warn, "warn msg %s", "arg1") }},
		{"Log_Error", func() { l.Log(hclog.Error, "error msg %s", "arg1") }},
		{"Log_Info", func() { l.Log(hclog.Info, "info msg %s", "arg1") }},
		{"Trace", func() { l.Trace("trace msg %s", "arg1") }},
		{"Debug", func() { l.Debug("debug msg %s", "arg1") }},
		{"Info", func() { l.Info("info msg %s", "arg1") }},
		{"Warn", func() { l.Warn("warn msg %s", "arg1") }},
		{"Error", func() { l.Error("error msg %s", "arg1") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, tt.fn)
		})
	}
}

func TestLoggerAdapterBoolMethods(t *testing.T) {
	l := &loggerAdapter{}

	tests := []struct {
		name     string
		fn       func() bool
		expected bool
	}{
		{"IsTrace", l.IsTrace, false},
		{"IsDebug", l.IsDebug, true},
		{"IsInfo", l.IsInfo, false},
		{"IsWarn", l.IsWarn, false},
		{"IsError", l.IsError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.fn())
		})
	}
}

func TestLoggerAdapterImpliedArgs(t *testing.T) {
	l := &loggerAdapter{}
	args := l.ImpliedArgs()
	assert.Equal(t, []interface{}{}, args)
}

func TestLoggerAdapterWith(t *testing.T) {
	l := &loggerAdapter{}
	result := l.With("key", "value")
	assert.Same(t, l, result)
}

func TestLoggerAdapterName(t *testing.T) {
	l := &loggerAdapter{}
	assert.Equal(t, "dapr", l.Name())
}

func TestLoggerAdapterNamed(t *testing.T) {
	l := &loggerAdapter{}
	result := l.Named("test")
	assert.Same(t, l, result)
}

func TestLoggerAdapterResetNamed(t *testing.T) {
	l := &loggerAdapter{}
	result := l.ResetNamed("test")
	assert.Same(t, l, result)
}

func TestLoggerAdapterSetLevel(t *testing.T) {
	l := &loggerAdapter{}
	assert.NotPanics(t, func() {
		l.SetLevel(hclog.Debug)
	})
}

func TestLoggerAdapterStandardLogger(t *testing.T) {
	l := &loggerAdapter{}
	stdLogger := l.StandardLogger(&hclog.StandardLoggerOptions{})
	assert.NotNil(t, stdLogger)
	assert.IsType(t, &log.Logger{}, stdLogger)
}

func TestLoggerAdapterStandardWriter(t *testing.T) {
	l := &loggerAdapter{}
	writer := l.StandardWriter(&hclog.StandardLoggerOptions{})
	assert.NotNil(t, writer)

	// Verify the writer implements io.Writer
	var _ io.Writer = writer
}
