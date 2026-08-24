// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapr/dapr/pkg/components/bindings"
	http_comp "github.com/dapr/dapr/pkg/components/middleware/http"
	"github.com/dapr/dapr/pkg/components/nameresolution"
	"github.com/dapr/dapr/pkg/components/pubsub"
	"github.com/dapr/dapr/pkg/components/secretstores"
	"github.com/dapr/dapr/pkg/components/state"
)

func TestWithSecretStores(t *testing.T) {
	t.Run("appends secret stores to opts", func(t *testing.T) {
		var opts runtimeOpts
		s := secretstores.SecretStore{Name: "test-secret-store"}
		o := WithSecretStores(s)
		o(&opts)

		assert.Len(t, opts.secretStores, 1)
		assert.Equal(t, "test-secret-store", opts.secretStores[0].Name)
	})
}

func TestWithStates(t *testing.T) {
	t.Run("appends state stores to opts", func(t *testing.T) {
		var opts runtimeOpts
		s := state.State{Name: "test-state"}
		o := WithStates(s)
		o(&opts)

		assert.Len(t, opts.states, 1)
		assert.Equal(t, "test-state", opts.states[0].Name)
	})
}

func TestWithPubSubs(t *testing.T) {
	t.Run("appends pubsubs to opts", func(t *testing.T) {
		var opts runtimeOpts
		p := pubsub.PubSub{Name: "test-pubsub"}
		o := WithPubSubs(p)
		o(&opts)

		assert.Len(t, opts.pubsubs, 1)
		assert.Equal(t, "test-pubsub", opts.pubsubs[0].Name)
	})
}

func TestWithNameResolutions(t *testing.T) {
	t.Run("appends name resolutions to opts", func(t *testing.T) {
		var opts runtimeOpts
		nr := nameresolution.NameResolution{Name: "test-nr"}
		o := WithNameResolutions(nr)
		o(&opts)

		assert.Len(t, opts.nameResolutions, 1)
		assert.Equal(t, "test-nr", opts.nameResolutions[0].Name)
	})
}

func TestWithInputBindings(t *testing.T) {
	t.Run("appends input bindings to opts", func(t *testing.T) {
		var opts runtimeOpts
		b := bindings.InputBinding{Name: "test-input"}
		o := WithInputBindings(b)
		o(&opts)

		assert.Len(t, opts.inputBindings, 1)
		assert.Equal(t, "test-input", opts.inputBindings[0].Name)
	})
}

func TestWithOutputBindings(t *testing.T) {
	t.Run("appends output bindings to opts", func(t *testing.T) {
		var opts runtimeOpts
		b := bindings.OutputBinding{Name: "test-output"}
		o := WithOutputBindings(b)
		o(&opts)

		assert.Len(t, opts.outputBindings, 1)
		assert.Equal(t, "test-output", opts.outputBindings[0].Name)
	})
}

func TestWithHTTPMiddleware(t *testing.T) {
	t.Run("appends http middleware to opts", func(t *testing.T) {
		var opts runtimeOpts
		m := http_comp.Middleware{Name: "test-middleware"}
		o := WithHTTPMiddleware(m)
		o(&opts)

		assert.Len(t, opts.httpMiddleware, 1)
		assert.Equal(t, "test-middleware", opts.httpMiddleware[0].Name)
	})
}
