package selfhosted

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewValidator(t *testing.T) {
	v := NewValidator()
	require.NotNil(t, v)
}

func TestValidate(t *testing.T) {
	v := NewValidator()
	err := v.Validate("id", "token", "namespace")
	assert.NoError(t, err)
}
