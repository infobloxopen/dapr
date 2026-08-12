package credentials

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTLSCredentials(t *testing.T) {
	t.Run("creates credentials with path", func(t *testing.T) {
		creds := NewTLSCredentials("/test/path")
		assert.Equal(t, "/test/path", creds.Path())
	})
}

func TestPath(t *testing.T) {
	t.Run("returns correct path", func(t *testing.T) {
		creds := NewTLSCredentials("/certs")
		assert.Equal(t, "/certs", creds.Path())
	})
}

func TestRootCertPath(t *testing.T) {
	t.Run("returns root cert file path", func(t *testing.T) {
		creds := NewTLSCredentials("/certs")
		assert.Equal(t, filepath.Join("/certs", RootCertFilename), creds.RootCertPath())
	})
}

func TestCertPath(t *testing.T) {
	t.Run("returns issuer cert file path", func(t *testing.T) {
		creds := NewTLSCredentials("/certs")
		assert.Equal(t, filepath.Join("/certs", IssuerCertFilename), creds.CertPath())
	})
}

func TestKeyPath(t *testing.T) {
	t.Run("returns issuer key file path", func(t *testing.T) {
		creds := NewTLSCredentials("/certs")
		assert.Equal(t, filepath.Join("/certs", IssuerKeyFilename), creds.KeyPath())
	})
}

func TestLoadFromDisk(t *testing.T) {
	t.Run("loads valid cert chain", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "creds-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		rootPath := filepath.Join(dir, "ca.crt")
		certPath := filepath.Join(dir, "issuer.crt")
		keyPath := filepath.Join(dir, "issuer.key")

		require.NoError(t, ioutil.WriteFile(rootPath, []byte(TestCACert), 0644))
		require.NoError(t, ioutil.WriteFile(certPath, []byte(TestCert), 0644))
		require.NoError(t, ioutil.WriteFile(keyPath, []byte(TestKey), 0644))

		chain, err := LoadFromDisk(rootPath, certPath, keyPath)
		require.NoError(t, err)
		require.NotNil(t, chain)
		assert.NotEmpty(t, chain.RootCA)
		assert.NotEmpty(t, chain.Cert)
		assert.NotEmpty(t, chain.Key)
	})

	t.Run("returns error for missing root cert", func(t *testing.T) {
		chain, err := LoadFromDisk("/nonexistent/ca.crt", "/nonexistent/issuer.crt", "/nonexistent/issuer.key")
		require.Error(t, err)
		assert.Nil(t, chain)
	})

	t.Run("returns error for missing issuer cert", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "creds-test2")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		rootPath := filepath.Join(dir, "ca.crt")
		require.NoError(t, ioutil.WriteFile(rootPath, []byte(TestCACert), 0644))

		chain, err := LoadFromDisk(rootPath, filepath.Join(dir, "missing.crt"), filepath.Join(dir, "missing.key"))
		require.Error(t, err)
		assert.Nil(t, chain)
	})

	t.Run("returns error for missing issuer key", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "creds-test3")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		rootPath := filepath.Join(dir, "ca.crt")
		certPath := filepath.Join(dir, "issuer.crt")
		require.NoError(t, ioutil.WriteFile(rootPath, []byte(TestCACert), 0644))
		require.NoError(t, ioutil.WriteFile(certPath, []byte(TestCert), 0644))

		chain, err := LoadFromDisk(rootPath, certPath, filepath.Join(dir, "missing.key"))
		require.Error(t, err)
		assert.Nil(t, chain)
	})
}
