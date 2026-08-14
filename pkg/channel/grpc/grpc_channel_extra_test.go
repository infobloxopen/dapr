package grpc

import (
	"testing"

	"github.com/dapr/dapr/pkg/channel"
	"github.com/dapr/dapr/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestGetBaseAddress(t *testing.T) {
	c := &Channel{
		baseAddress: "127.0.0.1:5000",
	}
	assert.Equal(t, "127.0.0.1:5000", c.GetBaseAddress())
}

func TestCreateLocalChannelWithConcurrency(t *testing.T) {
	t.Run("with max concurrency creates buffered channel", func(t *testing.T) {
		ch := CreateLocalChannel(5000, 10, nil, config.TracingSpec{})
		assert.NotNil(t, ch)
		assert.NotNil(t, ch.ch)
		assert.Equal(t, 10, cap(ch.ch))
		assert.Equal(t, channel.DefaultChannelAddress+":5000", ch.baseAddress)
	})

	t.Run("with zero concurrency does not create channel", func(t *testing.T) {
		ch := CreateLocalChannel(5000, 0, nil, config.TracingSpec{})
		assert.NotNil(t, ch)
		assert.Nil(t, ch.ch)
	})

	t.Run("with negative concurrency does not create channel", func(t *testing.T) {
		ch := CreateLocalChannel(5000, -1, nil, config.TracingSpec{})
		assert.NotNil(t, ch)
		assert.Nil(t, ch.ch)
	})

	t.Run("stores tracing spec", func(t *testing.T) {
		spec := config.TracingSpec{SamplingRate: "1"}
		ch := CreateLocalChannel(3000, 0, nil, spec)
		assert.Equal(t, "1", ch.tracingSpec.SamplingRate)
	})
}
