// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sentryv1pb "github.com/dapr/dapr/pkg/proto/sentry/v1"
	"github.com/dapr/dapr/pkg/sentry/ca"
	"github.com/dapr/dapr/pkg/sentry/identity"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

type stubTrustRootBundler struct {
	issuerCertPem []byte
	rootCertPem   []byte
	issuerExpiry  time.Time
	trustAnchors  *x509.CertPool
	trustDomain   string
}

func (s *stubTrustRootBundler) GetIssuerCertPem() []byte      { return s.issuerCertPem }
func (s *stubTrustRootBundler) GetRootCertPem() []byte         { return s.rootCertPem }
func (s *stubTrustRootBundler) GetIssuerCertExpiry() time.Time { return s.issuerExpiry }
func (s *stubTrustRootBundler) GetTrustAnchors() *x509.CertPool { return s.trustAnchors }
func (s *stubTrustRootBundler) GetTrustDomain() string         { return s.trustDomain }

type stubCA struct {
	bundle        ca.TrustRootBundler
	signCSRFn     func(csrPem []byte, subject string, id *identity.Bundle, ttl time.Duration, isCA bool) (*ca.SignedCertificate, error)
	validateCSRFn func(csr *x509.CertificateRequest) error
}

func (s *stubCA) LoadOrStoreTrustBundle() error        { return nil }
func (s *stubCA) GetCACertBundle() ca.TrustRootBundler { return s.bundle }

func (s *stubCA) SignCSR(csrPem []byte, subject string, id *identity.Bundle, ttl time.Duration, isCA bool) (*ca.SignedCertificate, error) {
	if s.signCSRFn != nil {
		return s.signCSRFn(csrPem, subject, id, ttl, isCA)
	}
	return nil, errors.New("signCSR not configured")
}

func (s *stubCA) ValidateCSR(csr *x509.CertificateRequest) error {
	if s.validateCSRFn != nil {
		return s.validateCSRFn(csr)
	}
	return nil
}

type stubValidator struct {
	validateFn func(id, token, namespace string) error
}

func (s *stubValidator) Validate(id, token, namespace string) error {
	if s.validateFn != nil {
		return s.validateFn(id, token, namespace)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func generateTestCSRPEM(t *testing.T, cn string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: cn},
	}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
}

func generateSelfSignedCert(t *testing.T, cn string, notAfter time.Time) (*x509.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-1 * time.Minute),
		NotAfter:     notAfter,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certBytes)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	return cert, certPEM
}

// ---------------------------------------------------------------------------
// TestNeedsRefresh
// ---------------------------------------------------------------------------

func TestNeedsRefresh(t *testing.T) {
	tests := []struct {
		name         string
		cert         *tls.Certificate
		expiryBuffer time.Duration
		want         bool
	}{
		{
			name:         "nil leaf returns true",
			cert:         &tls.Certificate{},
			expiryBuffer: serverCertExpiryBuffer,
			want:         true,
		},
		{
			name: "expired leaf returns true",
			cert: &tls.Certificate{
				Leaf: &x509.Certificate{
					NotAfter: time.Now().Add(-1 * time.Hour),
				},
			},
			expiryBuffer: serverCertExpiryBuffer,
			want:         true,
		},
		{
			name: "leaf expiring within buffer returns true",
			cert: &tls.Certificate{
				Leaf: &x509.Certificate{
					NotAfter: time.Now().Add(10 * time.Minute),
				},
			},
			expiryBuffer: serverCertExpiryBuffer,
			want:         true,
		},
		{
			name: "valid leaf not expiring soon returns false",
			cert: &tls.Certificate{
				Leaf: &x509.Certificate{
					NotAfter: time.Now().Add(1 * time.Hour),
				},
			},
			expiryBuffer: serverCertExpiryBuffer,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsRefresh(tt.cert, tt.expiryBuffer)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// TestNewCAServer
// ---------------------------------------------------------------------------

func TestNewCAServer(t *testing.T) {
	t.Run("returns non-nil CAServer", func(t *testing.T) {
		mockCA := &stubCA{}
		mockVal := &stubValidator{}

		srv := NewCAServer(mockCA, mockVal)
		require.NotNil(t, srv)
	})

	t.Run("underlying type is *server", func(t *testing.T) {
		mockCA := &stubCA{}
		mockVal := &stubValidator{}

		srv := NewCAServer(mockCA, mockVal)
		concrete, ok := srv.(*server)
		require.True(t, ok)
		assert.Equal(t, mockCA, concrete.certAuth)
		assert.Equal(t, mockVal, concrete.validator)
	})
}

// ---------------------------------------------------------------------------
// TestSignCertificate
// ---------------------------------------------------------------------------

func TestSignCertificate(t *testing.T) {
	tests := []struct {
		name        string
		req         *sentryv1pb.SignCertificateRequest
		setupCA     func() *stubCA
		setupVal    func() *stubValidator
		wantErr     bool
		errContains string
	}{
		{
			name: "empty CSR returns error",
			req: &sentryv1pb.SignCertificateRequest{
				CertificateSigningRequest: nil,
			},
			setupCA:     func() *stubCA { return &stubCA{} },
			setupVal:    func() *stubValidator { return &stubValidator{} },
			wantErr:     true,
			errContains: "cannot parse certificate signing request pem",
		},
		{
			name: "invalid CSR PEM returns error",
			req: &sentryv1pb.SignCertificateRequest{
				CertificateSigningRequest: []byte("not-a-valid-pem"),
			},
			setupCA:     func() *stubCA { return &stubCA{} },
			setupVal:    func() *stubValidator { return &stubValidator{} },
			wantErr:     true,
			errContains: "cannot parse certificate signing request pem",
		},
		{
			name: "ValidateCSR failure returns error",
			req:  nil, // set in test body after generating CSR
			setupCA: func() *stubCA {
				return &stubCA{
					validateCSRFn: func(csr *x509.CertificateRequest) error {
						return errors.New("csr validation failed")
					},
				}
			},
			setupVal:    func() *stubValidator { return &stubValidator{} },
			wantErr:     true,
			errContains: "error validating csr",
		},
		{
			name: "validator rejects identity",
			req:  nil, // set in test body after generating CSR
			setupCA: func() *stubCA {
				return &stubCA{}
			},
			setupVal: func() *stubValidator {
				return &stubValidator{
					validateFn: func(id, token, namespace string) error {
						return errors.New("identity not authorized")
					},
				}
			},
			wantErr:     true,
			errContains: "error validating requester identity",
		},
		{
			name: "SignCSR failure returns error",
			req:  nil, // set in test body after generating CSR
			setupCA: func() *stubCA {
				return &stubCA{
					signCSRFn: func(csrPem []byte, subject string, id *identity.Bundle, ttl time.Duration, isCA bool) (*ca.SignedCertificate, error) {
						return nil, errors.New("signing failed")
					},
				}
			},
			setupVal:    func() *stubValidator { return &stubValidator{} },
			wantErr:     true,
			errContains: "error signing csr",
		},
		{
			name: "empty cert chain returns insufficient data error",
			req:  nil, // set in test body after generating CSR
			setupCA: func() *stubCA {
				cert, _ := generateSelfSignedCert(t, "test-app", time.Now().Add(1*time.Hour))
				return &stubCA{
					bundle: &stubTrustRootBundler{
						issuerCertPem: nil,
						rootCertPem:   nil,
						trustDomain:   "test.domain",
					},
					signCSRFn: func(csrPem []byte, subject string, id *identity.Bundle, ttl time.Duration, isCA bool) (*ca.SignedCertificate, error) {
						return &ca.SignedCertificate{
							Certificate: cert,
							CertPEM:     nil, // empty to trigger the len check
						}, nil
					},
				}
			},
			setupVal:    func() *stubValidator { return &stubValidator{} },
			wantErr:     true,
			errContains: "insufficient data in certificate signing request",
		},
		{
			name: "successful signing returns certificate response",
			req:  nil, // set in test body after generating CSR
			setupCA: func() *stubCA {
				cert, certPEM := generateSelfSignedCert(t, "test-app", time.Now().Add(1*time.Hour))
				_, issuerPEM := generateSelfSignedCert(t, "issuer", time.Now().Add(24*time.Hour))
				_, rootPEM := generateSelfSignedCert(t, "root", time.Now().Add(24*time.Hour))

				return &stubCA{
					bundle: &stubTrustRootBundler{
						issuerCertPem: issuerPEM,
						rootCertPem:   rootPEM,
						trustDomain:   "test.domain",
					},
					signCSRFn: func(csrPem []byte, subject string, id *identity.Bundle, ttl time.Duration, isCA bool) (*ca.SignedCertificate, error) {
						return &ca.SignedCertificate{
							Certificate: cert,
							CertPEM:     certPEM,
						}, nil
					},
				}
			},
			setupVal: func() *stubValidator { return &stubValidator{} },
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCA := tt.setupCA()
			mockVal := tt.setupVal()

			req := tt.req
			// For tests that need a valid CSR, generate one.
			if req == nil {
				csrPEM := generateTestCSRPEM(t, "test-app")
				req = &sentryv1pb.SignCertificateRequest{
					Id:                        "test-app",
					Token:                     "test-token",
					TrustDomain:               "test.domain",
					Namespace:                 "default",
					CertificateSigningRequest: csrPEM,
				}
			}

			s := &server{
				certAuth:  mockCA,
				validator: mockVal,
			}

			resp, err := s.SignCertificate(context.Background(), req)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.NotEmpty(t, resp.WorkloadCertificate)
				assert.Len(t, resp.TrustChainCertificates, 2)
				assert.NotNil(t, resp.ValidUntil)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestGetServerCertificate
// ---------------------------------------------------------------------------

func TestGetServerCertificate(t *testing.T) {
	t.Run("SignCSR error is propagated", func(t *testing.T) {
		mockCA := &stubCA{
			bundle: &stubTrustRootBundler{
				issuerExpiry: time.Now().Add(24 * time.Hour),
				trustDomain:  "test.domain",
			},
			signCSRFn: func(csrPem []byte, subject string, id *identity.Bundle, ttl time.Duration, isCA bool) (*ca.SignedCertificate, error) {
				return nil, errors.New("sign error")
			},
		}

		s := &server{certAuth: mockCA}
		cert, err := s.getServerCertificate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sign error")
		assert.Nil(t, cert)
	})

	t.Run("invalid cert PEM causes X509KeyPair error", func(t *testing.T) {
		cert, _ := generateSelfSignedCert(t, "test", time.Now().Add(1*time.Hour))
		mockCA := &stubCA{
			bundle: &stubTrustRootBundler{
				issuerExpiry:  time.Now().Add(24 * time.Hour),
				trustDomain:   "test.domain",
				issuerCertPem: nil,
				rootCertPem:   nil,
			},
			signCSRFn: func(csrPem []byte, subject string, id *identity.Bundle, ttl time.Duration, isCA bool) (*ca.SignedCertificate, error) {
				// Return a valid-looking cert that does NOT match the CSR's
				// private key, causing tls.X509KeyPair to fail.
				return &ca.SignedCertificate{
					Certificate: cert,
					CertPEM:     []byte("-----BEGIN CERTIFICATE-----\nbm90LWEtcmVhbC1jZXJ0\n-----END CERTIFICATE-----\n"),
				}, nil
			},
		}

		s := &server{certAuth: mockCA}
		result, err := s.getServerCertificate()
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("successful certificate generation", func(t *testing.T) {
		mockCA := &stubCA{
			bundle: &stubTrustRootBundler{
				issuerExpiry:  time.Now().Add(24 * time.Hour),
				trustDomain:   "test.domain",
				issuerCertPem: nil, // empty to keep X509KeyPair simple
				rootCertPem:   nil,
			},
			signCSRFn: func(csrPem []byte, subject string, id *identity.Bundle, ttl time.Duration, isCA bool) (*ca.SignedCertificate, error) {
				// Parse the CSR to extract its public key so the returned
				// certificate matches the private key from csr.GenerateCSR.
				block, _ := pem.Decode(csrPem)
				if block == nil {
					return nil, errors.New("failed to decode CSR PEM")
				}
				parsedCSR, err := x509.ParseCertificateRequest(block.Bytes)
				if err != nil {
					return nil, err
				}

				signingKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				if err != nil {
					return nil, err
				}

				template := &x509.Certificate{
					SerialNumber: big.NewInt(1),
					Subject:      parsedCSR.Subject,
					NotBefore:    time.Now().Add(-1 * time.Minute),
					NotAfter:     time.Now().Add(1 * time.Hour),
				}
				certBytes, err := x509.CreateCertificate(rand.Reader, template, template, parsedCSR.PublicKey, signingKey)
				if err != nil {
					return nil, err
				}
				cert, err := x509.ParseCertificate(certBytes)
				if err != nil {
					return nil, err
				}
				certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})

				return &ca.SignedCertificate{
					Certificate: cert,
					CertPEM:     certPEM,
				}, nil
			},
		}

		s := &server{certAuth: mockCA}
		cert, err := s.getServerCertificate()
		require.NoError(t, err)
		require.NotNil(t, cert)
	})
}
