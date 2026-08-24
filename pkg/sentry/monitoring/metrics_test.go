// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package monitoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInitMetrics(t *testing.T) {
	err := InitMetrics()
	assert.NoError(t, err)
}

func TestCertSignRequestRecieved(t *testing.T) {
	CertSignRequestRecieved()
}

func TestCertSignSucceed(t *testing.T) {
	CertSignSucceed()
}

func TestCertSignFailed(t *testing.T) {
	CertSignFailed("test-reason")
}

func TestIssuerCertExpiry(t *testing.T) {
	IssuerCertExpiry(time.Now().Add(24 * time.Hour))
}

func TestServerCertIssueFailed(t *testing.T) {
	ServerCertIssueFailed("test-reason")
}

func TestIssuerCertChanged(t *testing.T) {
	IssuerCertChanged()
}
