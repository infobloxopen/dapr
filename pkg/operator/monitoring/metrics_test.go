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

func TestRecordServiceCreatedCount(t *testing.T) {
	RecordServiceCreatedCount("testapp")
}

func TestRecordServiceDeletedCount(t *testing.T) {
	RecordServiceDeletedCount("testapp")
}

func TestRecordServiceUpdatedCount(t *testing.T) {
	RecordServiceUpdatedCount("testapp")
}
