// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package pubsub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotFoundError_Error(t *testing.T) {
	t.Run("returns formatted not found message", func(t *testing.T) {
		err := NotFoundError{PubsubName: "my-pubsub"}
		assert.Equal(t, "pubsub 'my-pubsub' not found", err.Error())
	})
}

func TestNotAllowedError_Error(t *testing.T) {
	t.Run("returns formatted not allowed message", func(t *testing.T) {
		err := NotAllowedError{Topic: "my-topic", ID: "my-app"}
		assert.Equal(t, "topic my-topic is not allowed for app id my-app", err.Error())
	})
}
