// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package diagnostics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceMetricsInit(t *testing.T) {
	s := newServiceMetrics()
	assert.False(t, s.enabled)

	// Init registers OpenCensus views. In the test process, views with the
	// same names may already be registered by other tests (e.g. TestInitMetrics
	// or TestFastHTTPMiddleware), so duplicate registration errors are tolerated.
	err := s.Init("test-svc-app")
	if err != nil {
		assert.Contains(t, err.Error(), "cannot register view")
	}

	// Init always sets appID and enabled regardless of view registration outcome.
	assert.True(t, s.enabled)
	assert.Equal(t, "test-svc-app", s.appID)
}

func TestServiceMetricsAllMethods(t *testing.T) {
	s := newServiceMetrics()
	// Manually enable to avoid duplicate view registration errors in the test
	// process. This lets us exercise every recording method.
	s.appID = "test-svc-all"
	s.enabled = true

	t.Run("ComponentLoaded", func(t *testing.T) {
		assert.NotPanics(t, func() { s.ComponentLoaded() })
	})
	t.Run("ComponentInitialized", func(t *testing.T) {
		assert.NotPanics(t, func() { s.ComponentInitialized("statestore") })
	})
	t.Run("ComponentInitFailed", func(t *testing.T) {
		assert.NotPanics(t, func() { s.ComponentInitFailed("statestore", "connection-error") })
	})
	t.Run("MTLSInitCompleted", func(t *testing.T) {
		assert.NotPanics(t, func() { s.MTLSInitCompleted() })
	})
	t.Run("MTLSInitFailed", func(t *testing.T) {
		assert.NotPanics(t, func() { s.MTLSInitFailed("cert-error") })
	})
	t.Run("MTLSWorkLoadCertRotationCompleted", func(t *testing.T) {
		assert.NotPanics(t, func() { s.MTLSWorkLoadCertRotationCompleted() })
	})
	t.Run("MTLSWorkLoadCertRotationFailed", func(t *testing.T) {
		assert.NotPanics(t, func() { s.MTLSWorkLoadCertRotationFailed("rotation-error") })
	})
	t.Run("ActorRebalanced", func(t *testing.T) {
		assert.NotPanics(t, func() { s.ActorRebalanced("DemoActor") })
	})
	t.Run("ActorDeactivated", func(t *testing.T) {
		assert.NotPanics(t, func() { s.ActorDeactivated("DemoActor") })
	})
	t.Run("ActorDeactivationFailed", func(t *testing.T) {
		assert.NotPanics(t, func() { s.ActorDeactivationFailed("DemoActor", "timeout") })
	})
	t.Run("ReportActorPendingCalls", func(t *testing.T) {
		assert.NotPanics(t, func() { s.ReportActorPendingCalls("DemoActor", 5) })
	})
	t.Run("ActorStatusReported", func(t *testing.T) {
		assert.NotPanics(t, func() { s.ActorStatusReported("register") })
	})
	t.Run("ActorStatusReportFailed", func(t *testing.T) {
		assert.NotPanics(t, func() { s.ActorStatusReportFailed("register", "network-error") })
	})
	t.Run("ActorPlacementTableOperationReceived", func(t *testing.T) {
		assert.NotPanics(t, func() { s.ActorPlacementTableOperationReceived("update") })
	})
	t.Run("RequestAllowedByAppAction", func(t *testing.T) {
		assert.NotPanics(t, func() {
			s.RequestAllowedByAppAction("app1", "public", "default", "/invoke", "POST", true)
		})
	})
	t.Run("RequestBlockedByAppAction", func(t *testing.T) {
		assert.NotPanics(t, func() {
			s.RequestBlockedByAppAction("app1", "public", "default", "/invoke", "POST", false)
		})
	})
	t.Run("RequestAllowedByGlobalAction", func(t *testing.T) {
		assert.NotPanics(t, func() {
			s.RequestAllowedByGlobalAction("app1", "public", "default", "/invoke", "GET", true)
		})
	})
	t.Run("RequestBlockedByGlobalAction", func(t *testing.T) {
		assert.NotPanics(t, func() {
			s.RequestBlockedByGlobalAction("app1", "public", "default", "/invoke", "GET", false)
		})
	})
}

func TestServiceMetricsDisabledNoPanic(t *testing.T) {
	s := newServiceMetrics()
	assert.False(t, s.enabled)

	// All methods should be no-ops and not panic when disabled
	s.ComponentLoaded()
	s.ComponentInitialized("statestore")
	s.ComponentInitFailed("statestore", "error")
	s.MTLSInitCompleted()
	s.MTLSInitFailed("error")
	s.MTLSWorkLoadCertRotationCompleted()
	s.MTLSWorkLoadCertRotationFailed("error")
	s.ActorRebalanced("DemoActor")
	s.ActorDeactivated("DemoActor")
	s.ActorDeactivationFailed("DemoActor", "timeout")
	s.ReportActorPendingCalls("DemoActor", 3)
	s.ActorStatusReported("register")
	s.ActorStatusReportFailed("register", "error")
	s.ActorPlacementTableOperationReceived("update")
	s.RequestAllowedByAppAction("app1", "td", "ns", "/op", "GET", true)
	s.RequestBlockedByAppAction("app1", "td", "ns", "/op", "GET", false)
	s.RequestAllowedByGlobalAction("app1", "td", "ns", "/op", "GET", true)
	s.RequestBlockedByGlobalAction("app1", "td", "ns", "/op", "GET", false)
}
