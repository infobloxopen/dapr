// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package injector

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigWithDefaults(t *testing.T) {
	t.Run("SidecarImagePullPolicy is Always", func(t *testing.T) {
		c := NewConfigWithDefaults()
		assert.Equal(t, "Always", c.SidecarImagePullPolicy)
	})

	t.Run("other fields are empty", func(t *testing.T) {
		c := NewConfigWithDefaults()
		assert.Empty(t, c.TLSCertFile)
		assert.Empty(t, c.TLSKeyFile)
		assert.Empty(t, c.SidecarImage)
		assert.Empty(t, c.Namespace)
	})
}

func TestGetConfigFromEnvironment(t *testing.T) {
	t.Run("required env vars set", func(t *testing.T) {
		envVars := map[string]string{
			"TLS_CERT_FILE": "/certs/cert.pem",
			"TLS_KEY_FILE":  "/certs/key.pem",
			"SIDECAR_IMAGE": "daprio/daprd:latest",
			"NAMESPACE":     "dapr-system",
		}
		for k, v := range envVars {
			os.Setenv(k, v)
		}
		defer func() {
			for k := range envVars {
				os.Unsetenv(k)
			}
		}()

		c, err := GetConfigFromEnvironment()
		require.NoError(t, err)
		assert.Equal(t, "/certs/cert.pem", c.TLSCertFile)
		assert.Equal(t, "/certs/key.pem", c.TLSKeyFile)
		assert.Equal(t, "daprio/daprd:latest", c.SidecarImage)
		assert.Equal(t, "dapr-system", c.Namespace)
		assert.Equal(t, "Always", c.SidecarImagePullPolicy)
	})

	t.Run("pull policy override from env", func(t *testing.T) {
		envVars := map[string]string{
			"TLS_CERT_FILE":             "/certs/cert.pem",
			"TLS_KEY_FILE":              "/certs/key.pem",
			"SIDECAR_IMAGE":             "daprio/daprd:latest",
			"NAMESPACE":                 "dapr-system",
			"SIDECAR_IMAGE_PULL_POLICY": "IfNotPresent",
		}
		for k, v := range envVars {
			os.Setenv(k, v)
		}
		defer func() {
			for k := range envVars {
				os.Unsetenv(k)
			}
		}()

		c, err := GetConfigFromEnvironment()
		require.NoError(t, err)
		assert.Equal(t, "IfNotPresent", c.SidecarImagePullPolicy)
	})

	t.Run("missing required env vars returns error", func(t *testing.T) {
		// Ensure none of the required vars are set.
		for _, k := range []string{"TLS_CERT_FILE", "TLS_KEY_FILE", "SIDECAR_IMAGE", "NAMESPACE"} {
			os.Unsetenv(k)
		}

		_, err := GetConfigFromEnvironment()
		require.Error(t, err)
	})
}
