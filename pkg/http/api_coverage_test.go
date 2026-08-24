// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package http

import (
	"context"
	"os"
	"testing"

	"github.com/dapr/components-contrib/secretstores"
	"github.com/dapr/components-contrib/state"
	components_v1alpha1 "github.com/dapr/dapr/pkg/apis/components/v1alpha1"
	"github.com/dapr/dapr/pkg/config"
	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
	daprt "github.com/dapr/dapr/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// stubAppChannel is a minimal implementation of channel.AppChannel for setter tests.
type stubAppChannel struct{}

func (s *stubAppChannel) GetBaseAddress() string { return "" }
func (s *stubAppChannel) InvokeMethod(_ context.Context, _ *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
	return nil, nil
}

func TestNewAPI(t *testing.T) {
	fakeStores := map[string]state.Store{
		"store1": fakeStateStore{},
	}
	fakeSecretStores := map[string]secretstores.SecretStore{
		"secret1": daprt.FakeSecretStore{},
	}
	components := []components_v1alpha1.Component{
		{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: "testComponent",
			},
		},
	}

	t.Run("constructor returns non-nil API with registered endpoints", func(t *testing.T) {
		a := NewAPI(
			"test-app",
			nil, // appChannel
			nil, // directMessaging
			components,
			fakeStores,
			fakeSecretStores,
			nil, // secretsConfiguration
			nil, // pubsubAdapter
			nil, // actor
			nil, // sendToOutputBindingFn
			config.TracingSpec{},
		)

		require.NotNil(t, a)
		endpoints := a.APIEndpoints()
		assert.NotEmpty(t, endpoints, "NewAPI should register endpoints")
	})

	t.Run("constructor with nil maps", func(t *testing.T) {
		a := NewAPI(
			"test-app",
			nil,
			nil,
			nil, // components
			nil, // stateStores
			nil, // secretStores
			nil, // secretsConfiguration
			nil, // pubsubAdapter
			nil, // actor
			nil, // sendToOutputBindingFn
			config.TracingSpec{},
		)

		require.NotNil(t, a)
		endpoints := a.APIEndpoints()
		assert.NotEmpty(t, endpoints, "NewAPI should register endpoints even with nil dependencies")
	})
}

func TestAPIEndpoints(t *testing.T) {
	t.Run("endpoints list is non-empty", func(t *testing.T) {
		a := NewAPI(
			"test-app",
			nil, nil, nil, nil, nil, nil, nil, nil, nil,
			config.TracingSpec{},
		)
		endpoints := a.APIEndpoints()
		assert.NotEmpty(t, endpoints)
	})

	t.Run("endpoints contain expected routes", func(t *testing.T) {
		a := NewAPI(
			"test-app",
			nil, nil, nil, nil, nil, nil, nil, nil, nil,
			config.TracingSpec{},
		)
		endpoints := a.APIEndpoints()

		routeSet := make(map[string]bool)
		for _, ep := range endpoints {
			routeSet[ep.Route] = true
		}

		expectedRoutes := []string{
			"healthz",
			"metadata",
		}
		for _, route := range expectedRoutes {
			assert.True(t, routeSet[route], "expected route %q to be registered", route)
		}
	})
}

func TestMarkStatusAsReady(t *testing.T) {
	t.Run("healthz returns 500 before marking ready", func(t *testing.T) {
		a := &api{readyStatus: false}
		ctx := &fasthttp.RequestCtx{}
		a.onGetHealthz(ctx)
		assert.Equal(t, fasthttp.StatusInternalServerError, ctx.Response.StatusCode())
	})

	t.Run("healthz returns 200 after marking ready", func(t *testing.T) {
		a := &api{readyStatus: false}
		a.MarkStatusAsReady()
		ctx := &fasthttp.RequestCtx{}
		a.onGetHealthz(ctx)
		assert.Equal(t, fasthttp.StatusNoContent, ctx.Response.StatusCode())
	})

	t.Run("marking ready is idempotent", func(t *testing.T) {
		a := &api{readyStatus: false}
		a.MarkStatusAsReady()
		a.MarkStatusAsReady()
		ctx := &fasthttp.RequestCtx{}
		a.onGetHealthz(ctx)
		assert.Equal(t, fasthttp.StatusNoContent, ctx.Response.StatusCode())
	})
}

func TestIsSecretAllowed(t *testing.T) {
	tests := []struct {
		name                 string
		secretsConfiguration map[string]config.SecretsScope
		storeName            string
		key                  string
		expected             bool
	}{
		{
			name:                 "no configuration for store allows all secrets",
			secretsConfiguration: map[string]config.SecretsScope{},
			storeName:            "unknownStore",
			key:                  "any-key",
			expected:             true,
		},
		{
			name: "allowed secrets list permits listed key",
			secretsConfiguration: map[string]config.SecretsScope{
				"store1": {
					DefaultAccess:  config.DenyAccess,
					AllowedSecrets: []string{"allowed-key"},
				},
			},
			storeName: "store1",
			key:       "allowed-key",
			expected:  true,
		},
		{
			name: "allowed secrets list denies unlisted key",
			secretsConfiguration: map[string]config.SecretsScope{
				"store1": {
					DefaultAccess:  config.DenyAccess,
					AllowedSecrets: []string{"allowed-key"},
				},
			},
			storeName: "store1",
			key:       "not-allowed-key",
			expected:  false,
		},
		{
			name: "denied secrets list blocks listed key",
			secretsConfiguration: map[string]config.SecretsScope{
				"store1": {
					DefaultAccess: config.AllowAccess,
					DeniedSecrets: []string{"denied-key"},
				},
			},
			storeName: "store1",
			key:       "denied-key",
			expected:  false,
		},
		{
			name: "denied secrets list allows unlisted key",
			secretsConfiguration: map[string]config.SecretsScope{
				"store1": {
					DefaultAccess: config.AllowAccess,
					DeniedSecrets: []string{"denied-key"},
				},
			},
			storeName: "store1",
			key:       "other-key",
			expected:  true,
		},
		{
			name:                 "nil configuration map allows all secrets",
			secretsConfiguration: nil,
			storeName:            "store1",
			key:                  "any-key",
			expected:             true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := &api{
				secretsConfiguration: tc.secretsConfiguration,
			}
			result := a.isSecretAllowed(tc.storeName, tc.key)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSetAppChannel(t *testing.T) {
	t.Run("set non-nil app channel", func(t *testing.T) {
		a := &api{}
		assert.Nil(t, a.appChannel)
		ch := &stubAppChannel{}
		a.SetAppChannel(ch)
		assert.Equal(t, ch, a.appChannel)
	})

	t.Run("set nil app channel", func(t *testing.T) {
		ch := &stubAppChannel{}
		a := &api{appChannel: ch}
		a.SetAppChannel(nil)
		assert.Nil(t, a.appChannel)
	})
}

func TestSetDirectMessaging(t *testing.T) {
	t.Run("set direct messaging", func(t *testing.T) {
		a := &api{}
		assert.Nil(t, a.directMessaging)
		dm := &daprt.MockDirectMessaging{}
		a.SetDirectMessaging(dm)
		assert.Equal(t, dm, a.directMessaging)
	})

	t.Run("set nil direct messaging", func(t *testing.T) {
		dm := &daprt.MockDirectMessaging{}
		a := &api{directMessaging: dm}
		a.SetDirectMessaging(nil)
		assert.Nil(t, a.directMessaging)
	})
}

func TestSetActorRuntime(t *testing.T) {
	t.Run("set non-nil actor runtime", func(t *testing.T) {
		a := &api{}
		assert.Nil(t, a.actor)
		actor := &daprt.MockActors{}
		a.SetActorRuntime(actor)
		assert.Equal(t, actor, a.actor)
	})

	t.Run("set nil actor runtime", func(t *testing.T) {
		actor := &daprt.MockActors{}
		a := &api{actor: actor}
		a.SetActorRuntime(nil)
		assert.Nil(t, a.actor)
	})
}

func TestOnGetHealthz(t *testing.T) {
	t.Run("not ready returns 500 with error code", func(t *testing.T) {
		a := &api{readyStatus: false}
		ctx := &fasthttp.RequestCtx{}
		a.onGetHealthz(ctx)
		assert.Equal(t, fasthttp.StatusInternalServerError, ctx.Response.StatusCode())
		assert.Contains(t, string(ctx.Response.Body()), "ERR_HEALTH_NOT_READY")
	})

	t.Run("ready returns 200", func(t *testing.T) {
		a := &api{readyStatus: true}
		ctx := &fasthttp.RequestCtx{}
		a.onGetHealthz(ctx)
		assert.Equal(t, fasthttp.StatusNoContent, ctx.Response.StatusCode())
	})
}

func TestUseAPIAuthenticationStandalone(t *testing.T) {
	t.Run("no token configured passes through", func(t *testing.T) {
		os.Unsetenv("DAPR_API_TOKEN")

		called := false
		inner := func(ctx *fasthttp.RequestCtx) {
			called = true
		}

		handler := useAPIAuthentication(inner)
		ctx := &fasthttp.RequestCtx{}
		handler(ctx)
		assert.True(t, called, "handler should be called when no API token is configured")
	})

	t.Run("valid token passes through", func(t *testing.T) {
		os.Setenv("DAPR_API_TOKEN", "test-token-123")
		defer os.Unsetenv("DAPR_API_TOKEN")

		called := false
		inner := func(ctx *fasthttp.RequestCtx) {
			called = true
		}

		handler := useAPIAuthentication(inner)
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.Set("dapr-api-token", "test-token-123")
		handler(ctx)
		assert.True(t, called, "handler should be called with valid API token")
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		os.Setenv("DAPR_API_TOKEN", "test-token-123")
		defer os.Unsetenv("DAPR_API_TOKEN")

		called := false
		inner := func(ctx *fasthttp.RequestCtx) {
			called = true
		}

		handler := useAPIAuthentication(inner)
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.Set("dapr-api-token", "wrong-token")
		handler(ctx)
		assert.False(t, called, "handler should not be called with invalid API token")
		assert.Equal(t, fasthttp.StatusUnauthorized, ctx.Response.StatusCode())
	})

	t.Run("missing token returns 401", func(t *testing.T) {
		os.Setenv("DAPR_API_TOKEN", "test-token-123")
		defer os.Unsetenv("DAPR_API_TOKEN")

		called := false
		inner := func(ctx *fasthttp.RequestCtx) {
			called = true
		}

		handler := useAPIAuthentication(inner)
		ctx := &fasthttp.RequestCtx{}
		handler(ctx)
		assert.False(t, called, "handler should not be called without API token")
		assert.Equal(t, fasthttp.StatusUnauthorized, ctx.Response.StatusCode())
	})
}
