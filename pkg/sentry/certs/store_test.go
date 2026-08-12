package certs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/dapr/pkg/sentry/config"
)

func TestGetNamespace(t *testing.T) {
	t.Run("returns NAMESPACE env var", func(t *testing.T) {
		os.Setenv("NAMESPACE", "test-ns")
		defer os.Unsetenv("NAMESPACE")

		ns := getNamespace()
		assert.Equal(t, "test-ns", ns)
	})

	t.Run("returns default when NAMESPACE not set", func(t *testing.T) {
		os.Unsetenv("NAMESPACE")

		ns := getNamespace()
		assert.Equal(t, defaultSecretNamespace, ns)
	})
}

func TestStoreSelfhosted(t *testing.T) {
	t.Run("stores credentials to disk", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "store-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		rootPath := filepath.Join(dir, "ca.crt")
		certPath := filepath.Join(dir, "issuer.crt")
		keyPath := filepath.Join(dir, "issuer.key")

		rootPEM := []byte("root-cert-pem")
		certPEM := []byte("issuer-cert-pem")
		keyPEM := []byte("issuer-key-pem")

		err = storeSelfhosted(rootPEM, certPEM, keyPEM, rootPath, certPath, keyPath)
		require.NoError(t, err)

		data, err := ioutil.ReadFile(rootPath)
		require.NoError(t, err)
		assert.Equal(t, rootPEM, data)

		data, err = ioutil.ReadFile(certPath)
		require.NoError(t, err)
		assert.Equal(t, certPEM, data)

		data, err = ioutil.ReadFile(keyPath)
		require.NoError(t, err)
		assert.Equal(t, keyPEM, data)
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		err := storeSelfhosted([]byte("a"), []byte("b"), []byte("c"),
			"/nonexistent/dir/ca.crt",
			"/nonexistent/dir/issuer.crt",
			"/nonexistent/dir/issuer.key")
		assert.Error(t, err)
	})

	t.Run("returns error when issuer cert path is invalid", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "store-test-cert-err")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		rootPath := filepath.Join(dir, "ca.crt")
		err = storeSelfhosted([]byte("root"), []byte("cert"), []byte("key"),
			rootPath,
			"/nonexistent/dir/issuer.crt",
			filepath.Join(dir, "issuer.key"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "issuer.crt")
	})

	t.Run("returns error when issuer key path is invalid", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "store-test-key-err")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		rootPath := filepath.Join(dir, "ca.crt")
		certPath := filepath.Join(dir, "issuer.crt")
		err = storeSelfhosted([]byte("root"), []byte("cert"), []byte("key"),
			rootPath,
			certPath,
			"/nonexistent/dir/issuer.key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "issuer.key")
	})
}

func TestStoreCredentialsSelfhosted(t *testing.T) {
	// Ensure we are not in kubernetes mode
	os.Unsetenv("KUBERNETES_SERVICE_HOST")

	t.Run("stores credentials via selfhosted path", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "store-creds-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		conf := config.SentryConfig{
			RootCertPath:   filepath.Join(dir, "ca.crt"),
			IssuerCertPath: filepath.Join(dir, "issuer.crt"),
			IssuerKeyPath:  filepath.Join(dir, "issuer.key"),
		}

		err = StoreCredentials(conf, []byte("root"), []byte("cert"), []byte("key"))
		require.NoError(t, err)

		data, err := ioutil.ReadFile(conf.RootCertPath)
		require.NoError(t, err)
		assert.Equal(t, []byte("root"), data)
	})
}

func TestCredentialsExistSelfhosted(t *testing.T) {
	// Ensure we are not in kubernetes mode
	os.Unsetenv("KUBERNETES_SERVICE_HOST")

	t.Run("returns false when not kubernetes hosted", func(t *testing.T) {
		conf := config.SentryConfig{}
		exists, err := CredentialsExist(conf)
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestCredentialsExistKubernetes(t *testing.T) {
	// Set kubernetes env to trigger the kubernetes path, which will fail
	// at GetClient because we are not running inside a cluster
	os.Setenv("KUBERNETES_SERVICE_HOST", "fake-host")
	os.Setenv("KUBERNETES_SERVICE_PORT", "443")
	defer os.Unsetenv("KUBERNETES_SERVICE_HOST")
	defer os.Unsetenv("KUBERNETES_SERVICE_PORT")

	t.Run("returns error when kubernetes client fails", func(t *testing.T) {
		conf := config.SentryConfig{}
		exists, err := CredentialsExist(conf)
		assert.Error(t, err)
		assert.False(t, exists)
	})
}

func TestStoreCredentialsKubernetes(t *testing.T) {
	// Set kubernetes env to trigger the kubernetes path, which will fail
	// at GetClient because we are not running inside a cluster
	os.Setenv("KUBERNETES_SERVICE_HOST", "fake-host")
	os.Setenv("KUBERNETES_SERVICE_PORT", "443")
	defer os.Unsetenv("KUBERNETES_SERVICE_HOST")
	defer os.Unsetenv("KUBERNETES_SERVICE_PORT")

	t.Run("returns error when kubernetes client fails", func(t *testing.T) {
		conf := config.SentryConfig{}
		err := StoreCredentials(conf, []byte("root"), []byte("cert"), []byte("key"))
		assert.Error(t, err)
	})
}
