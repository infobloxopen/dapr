/*
Copyright 2023 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package patcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/dapr/dapr/pkg/injector/annotations"
	injectorConsts "github.com/dapr/dapr/pkg/injector/consts"
)

func TestSidecarConfigInit(t *testing.T) {
	c := NewSidecarConfig(&corev1.Pod{})

	// Ensure default values are set (and that those without a default value are zero)
	// Check properties of supported kinds: bools, strings, ints
	assert.Equal(t, "", c.Config)
	assert.Equal(t, "info", c.LogLevel)
	assert.Equal(t, int32(0), c.AppPort)
	assert.Equal(t, int32(9090), c.SidecarMetricsPort)
	assert.False(t, c.EnableProfiling)
	assert.True(t, c.EnableMetrics)

	// These properties don't have an annotation but should have a default value anyways
	assert.Equal(t, injectorConsts.ModeKubernetes, c.Mode)
	assert.Equal(t, int32(3500), c.SidecarHTTPPort)
	assert.Equal(t, int32(50001), c.SidecarAPIGRPCPort)

	// Nullable properties
	assert.Nil(t, c.EnableAPILogging)
}

func TestSidecarConfigSetFromAnnotations(t *testing.T) {
	t.Run("set properties", func(t *testing.T) {
		c := NewSidecarConfig(&corev1.Pod{})

		// Set properties of supported kinds: bools, strings, ints
		c.setFromAnnotations(map[string]string{
			annotations.KeyEnabled:                "1", // Will be cast using utils.IsTruthy
			annotations.KeyAppID:                  "myappid",
			annotations.KeyAppPort:                "9876",
			annotations.KeyMetricsPort:            "6789",  // Override default value
			annotations.KeyEnableAPILogging:       "false", // Nullable property
			annotations.KeyPlacementHostAddresses: "",
		})

		assert.True(t, c.Enabled)
		assert.Equal(t, "myappid", c.AppID)
		assert.Equal(t, int32(9876), c.AppPort)
		assert.Equal(t, int32(6789), c.SidecarMetricsPort)
		assert.Equal(t, "", c.PlacementAddress)

		// Nullable properties
		_ = assert.NotNil(t, c.EnableAPILogging) &&
			assert.False(t, *c.EnableAPILogging)

		// Should maintain default values
		assert.Equal(t, "info", c.LogLevel)
	})

	t.Run("skip invalid properties", func(t *testing.T) {
		c := NewSidecarConfig(&corev1.Pod{})

		// Set properties of supported kinds: bools, strings, ints
		c.setFromAnnotations(map[string]string{
			annotations.KeyAppPort:            "zorro",
			annotations.KeyHTTPMaxRequestSize: "batman", // Nullable property
		})

		assert.Equal(t, int32(0), c.AppPort)
		assert.Nil(t, c.HTTPMaxRequestSize)
	})

	t.Run("host addresses with various empty string formats", func(t *testing.T) {
		testCases := []struct {
			name  string
			value string
		}{
			{"empty string", ""},
			{"single-quoted empty string", `'""'`},
			{"single quotes", `''`},
			{"double quotes", `""`},
			{"spaces", "   "},
			{"quoted spaces", `"   "`},
			{"single-quoted spaces", `'   '`},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := NewSidecarConfig(&corev1.Pod{})
				c.setFromAnnotations(map[string]string{
					annotations.KeySchedulerHostAddresses: tc.value,
					annotations.KeyPlacementHostAddresses: tc.value,
				})
				assert.Equal(t, "", c.PlacementAddress, "PlacementAddress should be empty for input: %q", tc.value)
				assert.Equal(t, "", c.SchedulerAddress, "SchedulerAddress should be empty for input: %q", tc.value)
			})
		}
	})
}

func TestLegacyInfobloxAnnotations(t *testing.T) {
	t.Run("map legacy infoblox annotations to standard dapr annotations", func(t *testing.T) {
		pod := &corev1.Pod{}
		pod.Annotations = map[string]string{
			"com.infoblox.dapr.sidecar-grpc-port":          "27002",
			"com.infoblox.dapr.sidecar-http-port":          "27003",
			"com.infoblox.dapr.sidecar-internal-grpc-port": "27004",
		}

		c := NewSidecarConfig(pod)
		c.SetFromPodAnnotations()

		assert.Equal(t, int32(27002), c.SidecarAPIGRPCPort)
		assert.Equal(t, int32(27003), c.SidecarHTTPPort)
		assert.Equal(t, int32(27004), c.SidecarInternalGRPCPort)
	})

	t.Run("standard dapr annotations take precedence over legacy", func(t *testing.T) {
		pod := &corev1.Pod{}
		pod.Annotations = map[string]string{
			"com.infoblox.dapr.sidecar-http-port": "27003",
			"dapr.io/http-port":                   "8080",
		}

		c := NewSidecarConfig(pod)
		c.SetFromPodAnnotations()

		// Standard annotations should win
		assert.Equal(t, int32(8080), c.SidecarHTTPPort)
	})

	t.Run("use default when neither legacy nor standard annotation present", func(t *testing.T) {
		pod := &corev1.Pod{}
		pod.Annotations = map[string]string{}

		c := NewSidecarConfig(pod)
		c.SetFromPodAnnotations()

		// Should use defaults from struct tags
		assert.Equal(t, int32(3500), c.SidecarHTTPPort)
		assert.Equal(t, int32(0), c.SidecarPublicPort)
		assert.Equal(t, int32(50001), c.SidecarAPIGRPCPort)
		assert.Equal(t, int32(50002), c.SidecarInternalGRPCPort)
	})
}
