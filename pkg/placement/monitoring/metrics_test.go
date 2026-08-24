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

func TestRecordRuntimesCount(t *testing.T) {
	RecordRuntimesCount(5)
}

func TestRecordActorRuntimesCount(t *testing.T) {
	RecordActorRuntimesCount(3)
}
