package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	t.Run("returns default version", func(t *testing.T) {
		assert.Equal(t, "edge", Version())
	})
}

func TestCommit(t *testing.T) {
	t.Run("returns commit value", func(t *testing.T) {
		// commit is empty string unless set via ldflags
		assert.Equal(t, "", Commit())
	})
}
