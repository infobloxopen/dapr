// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package diagnostics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitMetrics(t *testing.T) {
	// InitMetrics uses the Default* global singletons. Since views can only
	// be registered once per process and other tests in this package may
	// already have registered them, we tolerate an error from duplicate
	// registration but verify no panic occurs.
	err := InitMetrics("test-metrics-app")

	if err != nil {
		// Duplicate view registration is expected when other tests run first
		assert.Contains(t, err.Error(), "cannot register")
	}

	// Verify the globals were touched (appID set, enabled toggled)
	assert.Equal(t, "test-metrics-app", DefaultMonitoring.appID)
	assert.Equal(t, "test-metrics-app", DefaultGRPCMonitoring.appID)
	assert.Equal(t, "test-metrics-app", DefaultHTTPMonitoring.appID)
}
