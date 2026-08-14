package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewServerConfig(t *testing.T) {
	c := NewServerConfig("app1", "localhost", 3500, 7777, "http://example.com", true, 4)

	assert.Equal(t, "app1", c.AppID)
	assert.Equal(t, "localhost", c.HostAddress)
	assert.Equal(t, 3500, c.Port)
	assert.Equal(t, 7777, c.ProfilePort)
	assert.Equal(t, "http://example.com", c.AllowedOrigins)
	assert.True(t, c.EnableProfiling)
	assert.Equal(t, 4, c.MaxRequestBodySize)
}

func TestNewServerConfigDefaults(t *testing.T) {
	c := NewServerConfig("", "", 0, 0, "", false, 0)

	assert.Empty(t, c.AppID)
	assert.Empty(t, c.HostAddress)
	assert.Zero(t, c.Port)
	assert.Zero(t, c.ProfilePort)
	assert.Empty(t, c.AllowedOrigins)
	assert.False(t, c.EnableProfiling)
	assert.Zero(t, c.MaxRequestBodySize)
}
