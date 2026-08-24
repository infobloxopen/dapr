// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package grpc

import (
	"testing"
	"time"

	"github.com/dapr/dapr/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPIServer(t *testing.T) {
	t.Run("returns server with correct fields", func(t *testing.T) {
		cfg := ServerConfig{
			AppID:              "test-app",
			HostAddress:        "localhost",
			Port:               5000,
			MaxRequestBodySize: 4,
		}
		tracingSpec := config.TracingSpec{SamplingRate: "1"}
		metricSpec := config.MetricSpec{Enabled: true}

		srv := NewAPIServer(nil, cfg, tracingSpec, metricSpec)

		require.NotNil(t, srv)
		concrete, ok := srv.(*server)
		require.True(t, ok)
		assert.Equal(t, apiServer, concrete.kind)
		assert.Equal(t, cfg, concrete.config)
		assert.Equal(t, tracingSpec, concrete.tracingSpec)
		assert.Equal(t, metricSpec, concrete.metricSpec)
		assert.Nil(t, concrete.authenticator)
		assert.NotNil(t, concrete.logger)
		// maxConnectionAge is nil for API servers
		assert.Nil(t, concrete.maxConnectionAge)
		// renewMutex is nil for API servers
		assert.Nil(t, concrete.renewMutex)
	})

	t.Run("nil api is accepted", func(t *testing.T) {
		srv := NewAPIServer(nil, ServerConfig{}, config.TracingSpec{}, config.MetricSpec{})

		require.NotNil(t, srv)
		concrete := srv.(*server)
		assert.Nil(t, concrete.api)
		assert.Equal(t, apiServer, concrete.kind)
	})
}

func TestNewInternalServer(t *testing.T) {
	t.Run("returns server with correct fields", func(t *testing.T) {
		cfg := ServerConfig{
			AppID:              "test-app",
			HostAddress:        "localhost",
			Port:               50001,
			NameSpace:          "default",
			TrustDomain:        "cluster.local",
			MaxRequestBodySize: 4,
		}
		tracingSpec := config.TracingSpec{SamplingRate: "0.5"}
		metricSpec := config.MetricSpec{Enabled: false}

		srv := NewInternalServer(nil, cfg, tracingSpec, metricSpec, nil)

		require.NotNil(t, srv)
		concrete, ok := srv.(*server)
		require.True(t, ok)
		assert.Equal(t, internalServer, concrete.kind)
		assert.Equal(t, cfg, concrete.config)
		assert.Equal(t, tracingSpec, concrete.tracingSpec)
		assert.Equal(t, metricSpec, concrete.metricSpec)
		assert.Nil(t, concrete.authenticator)
		assert.NotNil(t, concrete.renewMutex)
		assert.NotNil(t, concrete.maxConnectionAge)
		assert.NotNil(t, concrete.logger)
	})

	t.Run("maxConnectionAge is set to default", func(t *testing.T) {
		srv := NewInternalServer(nil, ServerConfig{}, config.TracingSpec{}, config.MetricSpec{}, nil)

		concrete := srv.(*server)
		require.NotNil(t, concrete.maxConnectionAge)
		assert.Equal(t, time.Second*30, *concrete.maxConnectionAge)
	})
}

func TestGetDefaultMaxAgeDuration(t *testing.T) {
	t.Run("returns pointer to 30 seconds", func(t *testing.T) {
		d := getDefaultMaxAgeDuration()

		require.NotNil(t, d)
		assert.Equal(t, time.Second*30, *d)
	})

	t.Run("returns a new pointer each call", func(t *testing.T) {
		d1 := getDefaultMaxAgeDuration()
		d2 := getDefaultMaxAgeDuration()

		assert.Equal(t, *d1, *d2)
		// They should be distinct pointers.
		assert.NotSame(t, d1, d2)
	})
}
