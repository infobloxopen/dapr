package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewErrorResponse(t *testing.T) {
	t.Run("creates error response with code and message", func(t *testing.T) {
		r := NewErrorResponse("ERR_STATE_STORE_NOT_FOUND", "state store not found")
		assert.Equal(t, "ERR_STATE_STORE_NOT_FOUND", r.ErrorCode)
		assert.Equal(t, "state store not found", r.Message)
	})

	t.Run("creates error response with empty values", func(t *testing.T) {
		r := NewErrorResponse("", "")
		assert.Empty(t, r.ErrorCode)
		assert.Empty(t, r.Message)
	})
}
