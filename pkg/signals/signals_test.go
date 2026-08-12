package signals

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext(t *testing.T) {
	t.Run("returns non-nil context", func(t *testing.T) {
		ctx := Context()
		require.NotNil(t, ctx)
		assert.Nil(t, ctx.Err())
	})
}
