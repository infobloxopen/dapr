// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package injector

import (
	"fmt"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/util/intstr"

	"strconv"
	"testing"
)

func TestLogAsJSONEnabled(t *testing.T) {
	t.Run("dapr.io/log-as-json is true", func(t *testing.T) {
		fakeAnnotation := map[string]string{
			daprLogAsJSON: "true",
		}

		assert.Equal(t, true, logAsJSONEnabled(fakeAnnotation))
	})

	t.Run("dapr.io/log-as-json is false", func(t *testing.T) {
		fakeAnnotation := map[string]string{
			daprLogAsJSON: "false",
		}

		assert.Equal(t, false, logAsJSONEnabled(fakeAnnotation))
	})

	t.Run("dapr.io/log-as-json is not given", func(t *testing.T) {
		fakeAnnotation := map[string]string{}

		assert.Equal(t, false, logAsJSONEnabled(fakeAnnotation))
	})
}

func TestFormatProbePath(t *testing.T) {
	testCases := []struct {
		given    []string
		expected string
	}{
		{
			given:    []string{"api", "v1"},
			expected: "/api/v1",
		},
		{
			given:    []string{"//api", "v1"},
			expected: "/api/v1",
		},
		{
			given:    []string{"//api", "/v1/"},
			expected: "/api/v1",
		},
		{
			given:    []string{"//api", "/v1/", "healthz"},
			expected: "/api/v1/healthz",
		},
		{
			given:    []string{""},
			expected: "/",
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, formatProbePath(tc.given...))
	}
}

func TestGetProbeHttpHandler(t *testing.T) {
	pathElements := []string{"api", "v1", "healthz"}
	expectedPath := "/api/v1/healthz"
	expectedHandler := corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: expectedPath,
			Port: intstr.IntOrString{IntVal: defaultSidecarHTTPPort},
		},
	}

	assert.EqualValues(t, expectedHandler, getProbeHTTPHandler(defaultSidecarHTTPPort, pathElements...))
}

func TestGetSideCarContainer(t *testing.T) {
	annotations := map[string]string{}
	annotations[daprConfigKey] = "config"
	annotations[daprAppPortKey] = "5000"
	annotations[daprLogAsJSON] = "true"
	annotations[daprAPITokenSecret] = "secret"
	annotations[daprAppTokenSecret] = "appsecret"

	container, _ := getSidecarContainer(annotations, "app_id", "darpio/dapr", "Always", "dapr-system", "controlplane:9000", "placement:50000", nil, "", "", "", "sentry:50000", true, "pod_identity")

	expectedArgs := []string{
		"--mode", "kubernetes",
		"--dapr-http-port", "3500",
		"--dapr-grpc-port", "50001",
		"--dapr-internal-grpc-port", "50002",
		"--app-port", "5000",
		"--app-id", "app_id",
		"--control-plane-address", "controlplane:9000",
		"--app-protocol", "http",
		"--placement-host-address", "placement:50000",
		"--config", "config",
		"--log-level", "info",
		"--app-max-concurrency", "-1",
		"--sentry-address", "sentry:50000",
		"--metrics-port", "9090",
		"--dapr-http-max-request-size", "-1",
		"--log-as-json",
	}

	// DAPR_HOST_IP
	assert.Equal(t, "", container.Env[0].Value)
	// NAMESPACE
	assert.Equal(t, "dapr-system", container.Env[1].Value)
	// DAPR_API_TOKEN
	assert.Equal(t, "secret", container.Env[2].ValueFrom.SecretKeyRef.Name)
	// DAPR_APP_TOKEN
	assert.Equal(t, "appsecret", container.Env[3].ValueFrom.SecretKeyRef.Name)
	assert.EqualValues(t, expectedArgs, container.Args)
	assert.Equal(t, corev1.PullAlways, container.ImagePullPolicy)
}

func TestImagePullPolicy(t *testing.T) {
	testCases := []struct {
		testName       string
		pullPolicy     string
		expectedPolicy corev1.PullPolicy
	}{
		{
			"TestDefaultPullPolicy",
			"",
			corev1.PullIfNotPresent,
		},
		{
			"TestAlwaysPullPolicy",
			"Always",
			corev1.PullAlways,
		},
		{
			"TestNeverPullPolicy",
			"Never",
			corev1.PullNever,
		},
		{
			"TestIfNotPresentPullPolicy",
			"IfNotPresent",
			corev1.PullIfNotPresent,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			actualPolicy := getPullPolicy(tc.pullPolicy)
			fmt.Println(tc.testName)
			assert.Equal(t, tc.expectedPolicy, actualPolicy)
		})
	}
}

func TestGetSideCarContainerMTLSEnabled(t *testing.T) {
	t.Run("mTLS enabled with trust anchors adds cert env vars", func(t *testing.T) {
		annotations := map[string]string{}
		annotations[daprConfigKey] = "config"
		annotations[daprAppPortKey] = "5000"

		trustAnchors := "test-trust-anchors"
		certChain := "test-cert-chain"
		certKey := "test-cert-key"

		container, err := getSidecarContainer(annotations, "app_id", "darpio/dapr", "Always", "dapr-system", "controlplane:9000", "placement:50000", nil, trustAnchors, certChain, certKey, "sentry:50000", true, "ns:sa")
		assert.Nil(t, err)
		assert.NotNil(t, container)

		// Verify DAPR_TRUST_ANCHORS env var is present.
		found := false
		for _, env := range container.Env {
			if env.Name == "DAPR_TRUST_ANCHORS" {
				found = true
				assert.Equal(t, trustAnchors, env.Value)
				break
			}
		}
		assert.True(t, found, "DAPR_TRUST_ANCHORS env var not found")

		// Verify DAPR_CERT_CHAIN env var is present.
		found = false
		for _, env := range container.Env {
			if env.Name == "DAPR_CERT_CHAIN" {
				found = true
				assert.Equal(t, certChain, env.Value)
				break
			}
		}
		assert.True(t, found, "DAPR_CERT_CHAIN env var not found")

		// Verify DAPR_CERT_KEY env var is present.
		found = false
		for _, env := range container.Env {
			if env.Name == "DAPR_CERT_KEY" {
				found = true
				assert.Equal(t, certKey, env.Value)
				break
			}
		}
		assert.True(t, found, "DAPR_CERT_KEY env var not found")

		// Verify SENTRY_LOCAL_IDENTITY env var is present.
		found = false
		for _, env := range container.Env {
			if env.Name == "SENTRY_LOCAL_IDENTITY" {
				found = true
				assert.Equal(t, "ns:sa", env.Value)
				break
			}
		}
		assert.True(t, found, "SENTRY_LOCAL_IDENTITY env var not found")

		// Verify --enable-mtls arg is present.
		found = false
		for _, a := range container.Args {
			if a == "--enable-mtls" {
				found = true
				break
			}
		}
		assert.True(t, found, "--enable-mtls arg not found")
	})

	t.Run("mTLS enabled but empty trust anchors skips cert env vars", func(t *testing.T) {
		annotations := map[string]string{}
		annotations[daprConfigKey] = "config"
		annotations[daprAppPortKey] = "5000"

		container, err := getSidecarContainer(annotations, "app_id", "darpio/dapr", "Always", "dapr-system", "controlplane:9000", "placement:50000", nil, "", "", "", "sentry:50000", true, "ns:sa")
		assert.Nil(t, err)
		assert.NotNil(t, container)

		for _, env := range container.Env {
			assert.NotEqual(t, "DAPR_TRUST_ANCHORS", env.Name, "DAPR_TRUST_ANCHORS should not be set when trust anchors are empty")
		}

		for _, a := range container.Args {
			assert.NotEqual(t, "--enable-mtls", a, "--enable-mtls should not be set when trust anchors are empty")
		}
	})
}

func TestGetSideCarContainerWithAPITokenSecret(t *testing.T) {
	t.Run("API token secret set adds DAPR_API_TOKEN env var", func(t *testing.T) {
		annotations := map[string]string{}
		annotations[daprConfigKey] = "config"
		annotations[daprAppPortKey] = "5000"
		annotations[daprAPITokenSecret] = "my-api-secret"

		container, err := getSidecarContainer(annotations, "app_id", "darpio/dapr", "Always", "dapr-system", "controlplane:9000", "placement:50000", nil, "", "", "", "sentry:50000", false, "")
		assert.Nil(t, err)
		assert.NotNil(t, container)

		found := false
		for _, env := range container.Env {
			if env.Name == "DAPR_API_TOKEN" {
				found = true
				assert.NotNil(t, env.ValueFrom)
				assert.NotNil(t, env.ValueFrom.SecretKeyRef)
				assert.Equal(t, "my-api-secret", env.ValueFrom.SecretKeyRef.Name)
				assert.Equal(t, "token", env.ValueFrom.SecretKeyRef.Key)
				break
			}
		}
		assert.True(t, found, "DAPR_API_TOKEN env var not found")
	})

	t.Run("no API token secret skips DAPR_API_TOKEN env var", func(t *testing.T) {
		annotations := map[string]string{}
		annotations[daprConfigKey] = "config"
		annotations[daprAppPortKey] = "5000"

		container, err := getSidecarContainer(annotations, "app_id", "darpio/dapr", "Always", "dapr-system", "controlplane:9000", "placement:50000", nil, "", "", "", "sentry:50000", false, "")
		assert.Nil(t, err)

		for _, env := range container.Env {
			assert.NotEqual(t, "DAPR_API_TOKEN", env.Name, "DAPR_API_TOKEN should not be set when no secret is configured")
		}
	})
}

func TestGetSideCarContainerWithAppTokenSecret(t *testing.T) {
	t.Run("app token secret set adds APP_API_TOKEN env var", func(t *testing.T) {
		annotations := map[string]string{}
		annotations[daprConfigKey] = "config"
		annotations[daprAppPortKey] = "5000"
		annotations[daprAppTokenSecret] = "my-app-secret"

		container, err := getSidecarContainer(annotations, "app_id", "darpio/dapr", "Always", "dapr-system", "controlplane:9000", "placement:50000", nil, "", "", "", "sentry:50000", false, "")
		assert.Nil(t, err)
		assert.NotNil(t, container)

		found := false
		for _, env := range container.Env {
			if env.Name == "APP_API_TOKEN" {
				found = true
				assert.NotNil(t, env.ValueFrom)
				assert.NotNil(t, env.ValueFrom.SecretKeyRef)
				assert.Equal(t, "my-app-secret", env.ValueFrom.SecretKeyRef.Name)
				assert.Equal(t, "token", env.ValueFrom.SecretKeyRef.Key)
				break
			}
		}
		assert.True(t, found, "APP_API_TOKEN env var not found")
	})

	t.Run("no app token secret skips APP_API_TOKEN env var", func(t *testing.T) {
		annotations := map[string]string{}
		annotations[daprConfigKey] = "config"
		annotations[daprAppPortKey] = "5000"

		container, err := getSidecarContainer(annotations, "app_id", "darpio/dapr", "Always", "dapr-system", "controlplane:9000", "placement:50000", nil, "", "", "", "sentry:50000", false, "")
		assert.Nil(t, err)

		for _, env := range container.Env {
			assert.NotEqual(t, "APP_API_TOKEN", env.Name, "APP_API_TOKEN should not be set when no secret is configured")
		}
	})
}

func TestAddDaprEnvVarsToContainers(t *testing.T) {
	testCases := []struct {
		testName      string
		mockContainer corev1.Container
		mockEnvs      []corev1.EnvVar
		expOpsLen     int
		expOps        []PatchOperation
	}{
		{
			testName: "empty environment vars",
			mockContainer: corev1.Container{
				Name: "MockContainer",
			},
			mockEnvs: []corev1.EnvVar{
				{
					Name:  userContainerDaprHTTPPortName,
					Value: fmt.Sprint(defaultSidecarHTTPPort),
				},
				{
					Name:  userContainerDaprGRPCPortName,
					Value: strconv.Itoa(defaultSidecarAPIGRPCPort),
				},
			},
			expOpsLen: 1,
			expOps: []PatchOperation{
				{
					Op:   "add",
					Path: "/spec/containers/0/env",
					Value: []corev1.EnvVar{
						{
							Name:  userContainerDaprHTTPPortName,
							Value: fmt.Sprint(defaultSidecarHTTPPort),
						},
						{
							Name:  userContainerDaprGRPCPortName,
							Value: strconv.Itoa(defaultSidecarAPIGRPCPort),
						},
					},
				},
			},
		},
		{
			testName: "existing env var",
			mockContainer: corev1.Container{
				Name: "Mock Container",
				Env: []corev1.EnvVar{
					{
						Name:  "TEST",
						Value: "Existing value",
					},
				},
			},
			mockEnvs: []corev1.EnvVar{
				{
					Name:  userContainerDaprHTTPPortName,
					Value: fmt.Sprint(defaultSidecarHTTPPort),
				},
				{
					Name:  userContainerDaprGRPCPortName,
					Value: strconv.Itoa(defaultSidecarAPIGRPCPort),
				},
			},
			expOpsLen: 2,
			expOps: []PatchOperation{
				{
					Op:   "add",
					Path: "/spec/containers/0/env/-",
					Value: corev1.EnvVar{
						Name:  userContainerDaprHTTPPortName,
						Value: fmt.Sprint(defaultSidecarHTTPPort),
					},
				},
				{
					Op:   "add",
					Path: "/spec/containers/0/env/-",
					Value: corev1.EnvVar{
						Name:  userContainerDaprGRPCPortName,
						Value: strconv.Itoa(defaultSidecarAPIGRPCPort),
					},
				},
			},
		},
		{
			testName: "existing conflicting env var",
			mockContainer: corev1.Container{
				Name: "Mock Container",
				Env: []corev1.EnvVar{
					{
						Name:  "TEST",
						Value: "Existing value",
					},
					{
						Name:  userContainerDaprGRPCPortName,
						Value: "550000",
					},
				},
			},
			mockEnvs: []corev1.EnvVar{
				{
					Name:  userContainerDaprHTTPPortName,
					Value: fmt.Sprint(defaultSidecarHTTPPort),
				},
				{
					Name:  userContainerDaprGRPCPortName,
					Value: strconv.Itoa(defaultSidecarAPIGRPCPort),
				},
			},
			expOpsLen: 1,
			expOps: []PatchOperation{
				{
					Op:   "add",
					Path: "/spec/containers/0/env/-",
					Value: corev1.EnvVar{
						Name:  userContainerDaprHTTPPortName,
						Value: fmt.Sprint(defaultSidecarHTTPPort),
					},
				},
			},
		},
		{
			testName: "multiple existing conflicting env vars",
			mockContainer: corev1.Container{
				Name: "Mock Container",
				Env: []corev1.EnvVar{
					{
						Name:  userContainerDaprHTTPPortName,
						Value: "3510",
					},
					{
						Name:  userContainerDaprGRPCPortName,
						Value: "550000",
					},
				},
			},
			mockEnvs: []corev1.EnvVar{
				{
					Name:  userContainerDaprHTTPPortName,
					Value: fmt.Sprint(defaultSidecarHTTPPort),
				},
				{
					Name:  userContainerDaprGRPCPortName,
					Value: strconv.Itoa(defaultSidecarAPIGRPCPort),
				},
			},
			expOpsLen: 0,
			expOps:    []PatchOperation{},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			patchEnv := addDaprEnvVarsToContainers([]corev1.Container{tc.mockContainer}, tc.mockEnvs)
			fmt.Println(tc.testName)
			assert.Equal(t, tc.expOpsLen, len(patchEnv))
			assert.Equal(t, tc.expOps, patchEnv)
		})
	}
}
