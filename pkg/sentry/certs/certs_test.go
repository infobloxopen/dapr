package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestCertAndKey(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageCertSign,
		IsCA:         true,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}

func TestDecodePEMKey(t *testing.T) {
	t.Run("valid EC key", func(t *testing.T) {
		_, keyPEM := generateTestCertAndKey(t)
		pk, err := DecodePEMKey(keyPEM)
		require.NoError(t, err)
		require.NotNil(t, pk)
		assert.Equal(t, ECPrivateKey, pk.Type)
	})

	t.Run("invalid PEM returns error", func(t *testing.T) {
		pk, err := DecodePEMKey([]byte("not a pem"))
		assert.Error(t, err)
		assert.Nil(t, pk)
	})

	t.Run("unsupported block type", func(t *testing.T) {
		block := pem.EncodeToMemory(&pem.Block{Type: "UNKNOWN KEY", Bytes: []byte("fake")})
		pk, err := DecodePEMKey(block)
		assert.Error(t, err)
		assert.Nil(t, pk)
		assert.Contains(t, err.Error(), "unsupported block type")
	})
}

func TestDecodePEMCertificates(t *testing.T) {
	t.Run("valid certificate", func(t *testing.T) {
		certPEM, _ := generateTestCertAndKey(t)
		certs, err := DecodePEMCertificates(certPEM)
		require.NoError(t, err)
		assert.Len(t, certs, 1)
	})

	t.Run("invalid PEM returns error", func(t *testing.T) {
		_, err := DecodePEMCertificates([]byte("not a cert"))
		assert.Error(t, err)
	})
}

func TestPEMCredentialsFromFiles(t *testing.T) {
	t.Run("valid cert and key pair", func(t *testing.T) {
		certPEM, keyPEM := generateTestCertAndKey(t)
		creds, err := PEMCredentialsFromFiles(certPEM, keyPEM)
		require.NoError(t, err)
		require.NotNil(t, creds)
		assert.NotNil(t, creds.PrivateKey)
		assert.NotNil(t, creds.Certificate)
	})

	t.Run("invalid key returns error", func(t *testing.T) {
		certPEM, _ := generateTestCertAndKey(t)
		creds, err := PEMCredentialsFromFiles(certPEM, []byte("bad key"))
		assert.Error(t, err)
		assert.Nil(t, creds)
	})

	t.Run("invalid cert returns error", func(t *testing.T) {
		_, keyPEM := generateTestCertAndKey(t)
		creds, err := PEMCredentialsFromFiles([]byte("bad cert"), keyPEM)
		assert.Error(t, err)
		assert.Nil(t, creds)
	})

	t.Run("mismatched cert and key", func(t *testing.T) {
		certPEM, _ := generateTestCertAndKey(t)
		_, keyPEM2 := generateTestCertAndKey(t)
		creds, err := PEMCredentialsFromFiles(certPEM, keyPEM2)
		assert.Error(t, err)
		assert.Nil(t, creds)
	})

	t.Run("no certificates found in PEM", func(t *testing.T) {
		// Valid PEM but not a certificate type, so DecodePEMCertificates returns empty list
		_, keyPEM := generateTestCertAndKey(t)
		nonCertPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("dummy")})
		creds, err := PEMCredentialsFromFiles(nonCertPEM, keyPEM)
		assert.Error(t, err)
		assert.Nil(t, creds)
		assert.Contains(t, err.Error(), "no certificates found")
	})
}

func TestCertPoolFromPEM(t *testing.T) {
	t.Run("valid PEM creates pool", func(t *testing.T) {
		certPEM, _ := generateTestCertAndKey(t)
		pool, err := CertPoolFromPEM(certPEM)
		require.NoError(t, err)
		require.NotNil(t, pool)
	})

	t.Run("invalid PEM returns error", func(t *testing.T) {
		pool, err := CertPoolFromPEM([]byte("bad pem"))
		assert.Error(t, err)
		assert.Nil(t, pool)
	})

	t.Run("no certs found in PEM returns error", func(t *testing.T) {
		nonCertPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("dummy")})
		pool, err := CertPoolFromPEM(nonCertPEM)
		assert.Error(t, err)
		assert.Nil(t, pool)
		assert.Contains(t, err.Error(), "no certificates found")
	})
}

func TestDecodePEMCertificatesNonCertBlock(t *testing.T) {
	t.Run("non-certificate PEM block is skipped", func(t *testing.T) {
		// A valid PEM block that is not a CERTIFICATE type
		block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("dummy")})
		certs, err := DecodePEMCertificates(block)
		require.NoError(t, err)
		assert.Len(t, certs, 0)
	})
}

func TestParsePemCSR(t *testing.T) {
	t.Run("valid CSR", func(t *testing.T) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		tmpl := &x509.CertificateRequest{
			Subject: pkix.Name{Organization: []string{"test"}},
		}
		csrDER, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
		require.NoError(t, err)

		csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
		csr, err := ParsePemCSR(csrPEM)
		require.NoError(t, err)
		require.NotNil(t, csr)
		assert.Equal(t, "test", csr.Subject.Organization[0])
	})

	t.Run("invalid PEM returns error", func(t *testing.T) {
		csr, err := ParsePemCSR([]byte("not a csr"))
		assert.Error(t, err)
		assert.Nil(t, csr)
	})

	t.Run("invalid CSR bytes in valid PEM", func(t *testing.T) {
		invalidCSRPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: []byte("invalid-csr-data")})
		csr, err := ParsePemCSR(invalidCSRPEM)
		assert.Error(t, err)
		assert.Nil(t, csr)
	})
}

func TestGenerateECPrivateKey(t *testing.T) {
	t.Run("generates valid key", func(t *testing.T) {
		key, err := GenerateECPrivateKey()
		require.NoError(t, err)
		require.NotNil(t, key)
		assert.Equal(t, elliptic.P256(), key.Curve)
	})
}

func generateRSATestCertAndKey(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-rsa"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageCertSign,
		IsCA:         true,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER := x509.MarshalPKCS1PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}

func TestDecodePEMKeyRSA(t *testing.T) {
	t.Run("valid RSA key", func(t *testing.T) {
		_, keyPEM := generateRSATestCertAndKey(t)
		pk, err := DecodePEMKey(keyPEM)
		require.NoError(t, err)
		require.NotNil(t, pk)
		assert.Equal(t, RSAPrivateKey, pk.Type)
	})

	t.Run("invalid EC key bytes", func(t *testing.T) {
		block := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("invalid-key-data")})
		pk, err := DecodePEMKey(block)
		assert.Error(t, err)
		assert.Nil(t, pk)
	})

	t.Run("invalid RSA key bytes", func(t *testing.T) {
		block := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("invalid-key-data")})
		pk, err := DecodePEMKey(block)
		assert.Error(t, err)
		assert.Nil(t, pk)
	})
}

func TestMatchCertificateAndKeyRSA(t *testing.T) {
	t.Run("matching RSA cert and key", func(t *testing.T) {
		certPEM, keyPEM := generateRSATestCertAndKey(t)
		creds, err := PEMCredentialsFromFiles(certPEM, keyPEM)
		require.NoError(t, err)
		require.NotNil(t, creds)
		assert.Equal(t, RSAPrivateKey, creds.PrivateKey.Type)
	})

	t.Run("mismatched RSA cert and key", func(t *testing.T) {
		certPEM, _ := generateRSATestCertAndKey(t)
		_, keyPEM2 := generateRSATestCertAndKey(t)
		creds, err := PEMCredentialsFromFiles(certPEM, keyPEM2)
		assert.Error(t, err)
		assert.Nil(t, creds)
	})
}

func TestMatchCertificateAndKeyUnsupported(t *testing.T) {
	t.Run("unsupported key type returns false", func(t *testing.T) {
		certPEM, _ := generateTestCertAndKey(t)
		certs, err := DecodePEMCertificates(certPEM)
		require.NoError(t, err)
		require.Len(t, certs, 1)

		pk := &PrivateKey{Type: "UNKNOWN KEY TYPE", Key: nil}
		result := matchCertificateAndKey(pk, certs[0])
		assert.False(t, result)
	})
}
