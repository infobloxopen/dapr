package csr

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/dapr/pkg/sentry/identity"
)

func TestGenerateCSRTemplate(t *testing.T) {
	t.Run("valid csr template", func(t *testing.T) {
		tmpl, err := genCSRTemplate("test-org")
		assert.Nil(t, err)
		assert.Equal(t, "test-org", tmpl.Subject.Organization[0])
	})
}

func TestGenerateBaseCertificate(t *testing.T) {
	pk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	cert, err := generateBaseCert(time.Second*5, pk)

	assert.NoError(t, err)
	assert.Equal(t, cert.PublicKey, pk)
}

func TestGenerateIssuerCertCSR(t *testing.T) {
	pk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	cert, err := GenerateIssuerCertCSR("name", pk, time.Second*5)

	assert.NoError(t, err)
	assert.Equal(t, "name", cert.DNSNames[0])
	assert.Equal(t, "name", cert.Subject.CommonName)
}

func TestGenerateRootCertCSR(t *testing.T) {
	pk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	cert, err := GenerateRootCertCSR("org", "name", pk, time.Second*5)

	assert.NoError(t, err)
	assert.Equal(t, "name", cert.Subject.CommonName)
	assert.Equal(t, "org", cert.Subject.Organization[0])
}

func TestCertSerialNumber(t *testing.T) {
	n, err := newSerialNumber()
	assert.NoError(t, err)
	assert.NotNil(t, n)
}

func TestGenerateCSR(t *testing.T) {
	c, b, err := GenerateCSR("org", false)
	assert.NoError(t, err)
	assert.True(t, len(b) > 0)
	assert.True(t, len(c) > 0)
}

func TestGenerateCSRWithPKCS8(t *testing.T) {
	// Tests the pkcs8=true branch in GenerateCSR and the pkcs8 branch in encode()
	c, b, err := GenerateCSR("org", true)
	assert.NoError(t, err)
	assert.True(t, len(b) > 0)
	assert.True(t, len(c) > 0)
}

func TestEncodeCert(t *testing.T) {
	// Tests the csr=false branch in encode()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	// Test csr=false, pkcs8=false
	certPem, keyPem, err := encode(false, certDER, key, false)
	assert.NoError(t, err)
	assert.Contains(t, string(certPem), "CERTIFICATE")
	assert.Contains(t, string(keyPem), "EC PRIVATE KEY")

	// Test csr=false, pkcs8=true
	certPem2, keyPem2, err := encode(false, certDER, key, true)
	assert.NoError(t, err)
	assert.Contains(t, string(certPem2), "CERTIFICATE")
	assert.Contains(t, string(keyPem2), "PRIVATE KEY")
}

func TestGenerateCSRCertificate(t *testing.T) {
	// Generate a CA cert and key for signing
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	caCert, err := GenerateRootCertCSR("test-org", "test-ca", &caKey.PublicKey, time.Hour)
	require.NoError(t, err)

	caCertDER, err := x509.CreateCertificate(rand.Reader, caCert, caCert, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	signedCACert, err := x509.ParseCertificate(caCertDER)
	require.NoError(t, err)

	// Generate a CSR
	csrKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	csrTemplate := &x509.CertificateRequest{
		Subject:            pkix.Name{Organization: []string{"test-org"}},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, csrKey)
	require.NoError(t, err)
	csr, err := x509.ParseCertificateRequest(csrDER)
	require.NoError(t, err)

	t.Run("non-CA cert without identity bundle", func(t *testing.T) {
		certDER, err := GenerateCSRCertificate(csr, "test-app", nil, signedCACert, &csrKey.PublicKey, caKey, time.Hour, false)
		assert.NoError(t, err)
		assert.NotEmpty(t, certDER)

		cert, err := x509.ParseCertificate(certDER)
		assert.NoError(t, err)
		assert.False(t, cert.IsCA)
	})

	t.Run("CA cert", func(t *testing.T) {
		certDER, err := GenerateCSRCertificate(csr, "test-ca", nil, signedCACert, &csrKey.PublicKey, caKey, time.Hour, true)
		assert.NoError(t, err)
		assert.NotEmpty(t, certDER)

		cert, err := x509.ParseCertificate(certDER)
		assert.NoError(t, err)
		assert.True(t, cert.IsCA)
	})

	t.Run("cluster.local subject", func(t *testing.T) {
		certDER, err := GenerateCSRCertificate(csr, "cluster.local", nil, signedCACert, &csrKey.PublicKey, caKey, time.Hour, false)
		assert.NoError(t, err)
		assert.NotEmpty(t, certDER)

		cert, err := x509.ParseCertificate(certDER)
		assert.NoError(t, err)
		assert.Equal(t, "cluster.local", cert.Subject.CommonName)
	})

	t.Run("with identity bundle", func(t *testing.T) {
		bundle := &identity.Bundle{
			ID:          "test-app",
			Namespace:   "default",
			TrustDomain: "cluster.local",
		}
		certDER, err := GenerateCSRCertificate(csr, "test-app", bundle, signedCACert, &csrKey.PublicKey, caKey, time.Hour, false)
		assert.NoError(t, err)
		assert.NotEmpty(t, certDER)
	})
}
