package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBundle(t *testing.T) {
	t.Run("all fields populated", func(t *testing.T) {
		b := NewBundle("app1", "default", "cluster.local")
		assert.NotNil(t, b)
		assert.Equal(t, "app1", b.ID)
		assert.Equal(t, "default", b.Namespace)
		assert.Equal(t, "cluster.local", b.TrustDomain)
	})

	t.Run("empty namespace returns nil", func(t *testing.T) {
		b := NewBundle("app1", "", "cluster.local")
		assert.Nil(t, b)
	})

	t.Run("empty trust domain returns nil", func(t *testing.T) {
		b := NewBundle("app1", "default", "")
		assert.Nil(t, b)
	})

	t.Run("both empty returns nil", func(t *testing.T) {
		b := NewBundle("app1", "", "")
		assert.Nil(t, b)
	})
}
