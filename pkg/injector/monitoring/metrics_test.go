// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package monitoring

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitMetrics(t *testing.T) {
	err := InitMetrics()
	assert.NoError(t, err)
}

func TestRecordSidecarInjectionRequestsCount(t *testing.T) {
	RecordSidecarInjectionRequestsCount()
}

func TestRecordSuccessfulSidecarInjectionCount(t *testing.T) {
	RecordSuccessfulSidecarInjectionCount("testapp")
}

func TestRecordFailedSidecarInjectionCount(t *testing.T) {
	RecordFailedSidecarInjectionCount("testapp", "some-reason")
}
