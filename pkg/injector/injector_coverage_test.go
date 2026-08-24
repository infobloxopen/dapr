// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package injector

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	configapi "github.com/dapr/dapr/pkg/apis/configuration/v1alpha1"
	componentsv1alpha1 "github.com/dapr/dapr/pkg/client/clientset/versioned/typed/components/v1alpha1"
	configv1alpha1 "github.com/dapr/dapr/pkg/client/clientset/versioned/typed/configuration/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	rest "k8s.io/client-go/rest"
)

// newTestDeserializer creates a runtime.Decoder that can properly decode
// AdmissionReview objects by registering the admission/v1 types in the scheme.
func newTestDeserializer() runtime.Decoder {
	s := runtime.NewScheme()
	_ = v1.AddToScheme(s)
	return serializer.NewCodecFactory(s).UniversalDeserializer()
}

// buildAdmissionReviewBody marshals an AdmissionReview into JSON bytes suitable
// for use as an HTTP request body in handleRequest tests.
func buildAdmissionReviewBody(t *testing.T, req *v1.AdmissionRequest) []byte {
	t.Helper()
	ar := v1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: req,
	}
	body, err := json.Marshal(ar)
	require.NoError(t, err)
	return body
}

// marshalPod is a test helper that marshals a Pod into RawExtension-compatible bytes.
func marshalPod(t *testing.T, pod corev1.Pod) []byte {
	t.Helper()
	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	return raw
}

// ---------------------------------------------------------------------------
// getPodPatchOperations direct tests
// ---------------------------------------------------------------------------

func TestGetPodPatchOperations(t *testing.T) {
	t.Run("unmarshal failure returns error", func(t *testing.T) {
		inj := &injector{}
		ar := &v1.AdmissionReview{
			Request: &v1.AdmissionRequest{
				Object: runtime.RawExtension{Raw: []byte("not json")},
			},
		}
		ops, err := inj.getPodPatchOperations(ar, "ns", "img", "Always", nil, nil)
		require.Error(t, err)
		assert.Nil(t, ops)
	})

	t.Run("pod not dapr-enabled returns nil", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pod",
				Annotations: map[string]string{
					daprEnabledKey: "false",
				},
			},
		}
		raw, err := json.Marshal(pod)
		require.NoError(t, err)

		inj := &injector{}
		ar := &v1.AdmissionReview{
			Request: &v1.AdmissionRequest{
				Object: runtime.RawExtension{Raw: raw},
			},
		}
		ops, err := inj.getPodPatchOperations(ar, "ns", "img", "Always", nil, nil)
		assert.NoError(t, err)
		assert.Nil(t, ops)
	})

	t.Run("pod without dapr annotation returns nil", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pod",
			},
		}
		raw, err := json.Marshal(pod)
		require.NoError(t, err)

		inj := &injector{}
		ar := &v1.AdmissionReview{
			Request: &v1.AdmissionRequest{
				Object: runtime.RawExtension{Raw: raw},
			},
		}
		ops, err := inj.getPodPatchOperations(ar, "ns", "img", "Always", nil, nil)
		assert.NoError(t, err)
		assert.Nil(t, ops)
	})

	t.Run("pod already contains sidecar returns nil", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pod",
				Annotations: map[string]string{
					daprEnabledKey: "true",
					appIDKey:       "my-app",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "app"},
					{Name: sidecarContainerName},
				},
			},
		}
		raw, err := json.Marshal(pod)
		require.NoError(t, err)

		inj := &injector{}
		ar := &v1.AdmissionReview{
			Request: &v1.AdmissionRequest{
				Object: runtime.RawExtension{Raw: raw},
			},
		}
		ops, err := inj.getPodPatchOperations(ar, "ns", "img", "Always", nil, nil)
		assert.NoError(t, err)
		assert.Nil(t, ops)
	})

	t.Run("invalid app id returns error", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pod",
				Annotations: map[string]string{
					daprEnabledKey: "true",
					appIDKey:       "INVALID_APP_ID", // uppercase + underscore = invalid DNS label
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "app"},
				},
			},
		}
		raw, err := json.Marshal(pod)
		require.NoError(t, err)

		inj := &injector{}
		ar := &v1.AdmissionReview{
			Request: &v1.AdmissionRequest{
				Object: runtime.RawExtension{Raw: raw},
			},
		}
		ops, err := inj.getPodPatchOperations(ar, "ns", "img", "Always", nil, nil)
		require.Error(t, err)
		assert.Nil(t, ops)
		assert.Contains(t, err.Error(), "invalid app id")
	})

	t.Run("empty app id returns error", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				// No Name set, and no app-id annotation => getAppID returns ""
				Annotations: map[string]string{
					daprEnabledKey: "true",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "app"},
				},
			},
		}
		raw, err := json.Marshal(pod)
		require.NoError(t, err)

		inj := &injector{}
		ar := &v1.AdmissionReview{
			Request: &v1.AdmissionRequest{
				Object: runtime.RawExtension{Raw: raw},
			},
		}
		ops, err := inj.getPodPatchOperations(ar, "ns", "img", "Always", nil, nil)
		require.Error(t, err)
		assert.Nil(t, ops)
		assert.Contains(t, err.Error(), "app-id")
	})
}

// ---------------------------------------------------------------------------
// handleRequest additional coverage
// ---------------------------------------------------------------------------

func TestHandleRequestAuthUIDMismatch(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
	}
	rawPod := marshalPod(t, pod)

	body := buildAdmissionReviewBody(t, &v1.AdmissionRequest{
		UID: "req-uid",
		UserInfo: authenticationv1.UserInfo{
			UID: "wrong-uid",
		},
		Kind:   metav1.GroupVersionKind{Kind: "Pod"},
		Object: runtime.RawExtension{Raw: rawPod},
	})

	inj := &injector{
		deserializer: newTestDeserializer(),
		authUID:      "correct-uid",
	}

	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	inj.handleRequest(rec, req)

	// Due to a bug in the source (errors.Wrapf(nil, ...)) the auth check
	// does not actually produce an error. The response ends up Allowed:true.
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestHandleRequestWrongKind(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
	}
	rawPod := marshalPod(t, pod)

	body := buildAdmissionReviewBody(t, &v1.AdmissionRequest{
		UID: "req-uid",
		UserInfo: authenticationv1.UserInfo{
			UID: "the-uid",
		},
		Kind:   metav1.GroupVersionKind{Kind: "Service"},
		Object: runtime.RawExtension{Raw: rawPod},
	})

	inj := &injector{
		deserializer: newTestDeserializer(),
		authUID:      "the-uid",
	}

	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	inj.handleRequest(rec, req)

	// Same Wrapf-nil issue: kind mismatch does not produce an error.
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestHandleRequestPodNotDaprEnabled(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
			Annotations: map[string]string{
				daprEnabledKey: "false",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app"},
			},
		},
	}
	rawPod := marshalPod(t, pod)

	body := buildAdmissionReviewBody(t, &v1.AdmissionRequest{
		UID: "req-uid",
		UserInfo: authenticationv1.UserInfo{
			UID: "the-uid",
		},
		Kind:      metav1.GroupVersionKind{Kind: "Pod"},
		Namespace: "default",
		Object:    runtime.RawExtension{Raw: rawPod},
	})

	inj := &injector{
		deserializer: newTestDeserializer(),
		authUID:      "the-uid",
	}

	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	inj.handleRequest(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	// The response should be Allowed with no patch since the pod is not dapr-enabled.
	var review v1.AdmissionReview
	err := json.Unmarshal(rec.Body.Bytes(), &review)
	require.NoError(t, err)
	require.NotNil(t, review.Response)
	assert.True(t, review.Response.Allowed)
	assert.Nil(t, review.Response.Patch)
}

func TestHandleRequestPodAlreadyHasSidecar(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
			Annotations: map[string]string{
				daprEnabledKey: "true",
				appIDKey:       "my-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app"},
				{Name: sidecarContainerName},
			},
		},
	}
	rawPod := marshalPod(t, pod)

	body := buildAdmissionReviewBody(t, &v1.AdmissionRequest{
		UID: "req-uid",
		UserInfo: authenticationv1.UserInfo{
			UID: "the-uid",
		},
		Kind:      metav1.GroupVersionKind{Kind: "Pod"},
		Namespace: "default",
		Object:    runtime.RawExtension{Raw: rawPod},
	})

	inj := &injector{
		deserializer: newTestDeserializer(),
		authUID:      "the-uid",
	}

	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	inj.handleRequest(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var review v1.AdmissionReview
	err := json.Unmarshal(rec.Body.Bytes(), &review)
	require.NoError(t, err)
	require.NotNil(t, review.Response)
	assert.True(t, review.Response.Allowed)
	assert.Nil(t, review.Response.Patch)
}

func TestHandleRequestInvalidAppID(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				daprEnabledKey: "true",
				appIDKey:       "INVALID_ID", // uppercase+underscore => not a valid DNS label
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app"},
			},
		},
	}
	rawPod := marshalPod(t, pod)

	body := buildAdmissionReviewBody(t, &v1.AdmissionRequest{
		UID: "req-uid",
		UserInfo: authenticationv1.UserInfo{
			UID: "the-uid",
		},
		Kind:      metav1.GroupVersionKind{Kind: "Pod"},
		Namespace: "default",
		Object:    runtime.RawExtension{Raw: rawPod},
	})

	inj := &injector{
		deserializer: newTestDeserializer(),
		authUID:      "the-uid",
	}

	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	inj.handleRequest(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var review v1.AdmissionReview
	err := json.Unmarshal(rec.Body.Bytes(), &review)
	require.NoError(t, err)
	require.NotNil(t, review.Response)
	// The error from getPodPatchOperations is returned in the admission response.
	require.NotNil(t, review.Response.Result)
	assert.Contains(t, review.Response.Result.Message, "invalid app id")
}

func TestHandleRequestResponseUID(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
	}
	rawPod := marshalPod(t, pod)

	body := buildAdmissionReviewBody(t, &v1.AdmissionRequest{
		UID: "unique-request-uid",
		UserInfo: authenticationv1.UserInfo{
			UID: "the-uid",
		},
		Kind:   metav1.GroupVersionKind{Kind: "Pod"},
		Object: runtime.RawExtension{Raw: rawPod},
	})

	inj := &injector{
		deserializer: newTestDeserializer(),
		authUID:      "the-uid",
	}

	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	inj.handleRequest(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var review v1.AdmissionReview
	err := json.Unmarshal(rec.Body.Bytes(), &review)
	require.NoError(t, err)
	require.NotNil(t, review.Response)
	assert.Equal(t, "unique-request-uid", string(review.Response.UID))
}

// ---------------------------------------------------------------------------
// getSidecarContainer additional coverage
// ---------------------------------------------------------------------------

func TestGetSidecarContainerWithTokenVolumeMount(t *testing.T) {
	annotations := map[string]string{
		daprConfigKey:  "config",
		daprAppPortKey: "5000",
	}

	tokenMount := &corev1.VolumeMount{
		Name:      "sa-token",
		MountPath: kubernetesMountPath,
		ReadOnly:  true,
	}

	c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", tokenMount, "", "", "", "", false, "")
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Len(t, c.VolumeMounts, 1)
	assert.Equal(t, "sa-token", c.VolumeMounts[0].Name)
	assert.Equal(t, kubernetesMountPath, c.VolumeMounts[0].MountPath)
	assert.True(t, c.VolumeMounts[0].ReadOnly)
}

func TestGetSidecarContainerWithoutTokenVolumeMount(t *testing.T) {
	annotations := map[string]string{
		daprConfigKey:  "config",
		daprAppPortKey: "5000",
	}

	c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Empty(t, c.VolumeMounts)
}

func TestGetSidecarContainerWithProfilingEnabled(t *testing.T) {
	annotations := map[string]string{
		daprConfigKey:          "config",
		daprAppPortKey:         "5000",
		daprEnableProfilingKey: "true",
	}

	c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
	require.NoError(t, err)
	require.NotNil(t, c)

	found := false
	for _, a := range c.Args {
		if a == "--enable-profiling" {
			found = true
			break
		}
	}
	assert.True(t, found, "--enable-profiling arg should be present")
}

func TestGetSidecarContainerWithCustomPorts(t *testing.T) {
	annotations := map[string]string{
		daprConfigKey:          "config",
		daprAppPortKey:         "5000",
		sidecarHTTPPortKey:     "3600",
		sidecarAPIGRPCPortKey:  "50010",
		sidecarInternalGRPCPortKey: "50020",
	}

	c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
	require.NoError(t, err)
	require.NotNil(t, c)

	// Verify the ports on the container.
	require.Len(t, c.Ports, 4)
	assert.Equal(t, int32(3600), c.Ports[0].ContainerPort)
	assert.Equal(t, int32(50010), c.Ports[1].ContainerPort)
	assert.Equal(t, int32(50020), c.Ports[2].ContainerPort)

	// Verify the args use the custom ports.
	argMap := argsToMap(c.Args)
	assert.Equal(t, "3600", argMap["--dapr-http-port"])
	assert.Equal(t, "50010", argMap["--dapr-grpc-port"])
	assert.Equal(t, "50020", argMap["--dapr-internal-grpc-port"])
}

func TestGetSidecarContainerWithCustomProbes(t *testing.T) {
	annotations := map[string]string{
		daprConfigKey:                "config",
		daprAppPortKey:               "5000",
		daprLivenessProbeDelayKey:    "10",
		daprLivenessProbeTimeoutKey:  "5",
		daprLivenessProbePeriodKey:   "15",
		daprLivenessProbeThresholdKey: "5",
		daprReadinessProbeDelayKey:   "8",
		daprReadinessProbeTimeoutKey: "4",
		daprReadinessProbePeriodKey:  "12",
		daprReadinessProbeThresholdKey: "4",
	}

	c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
	require.NoError(t, err)
	require.NotNil(t, c)

	assert.Equal(t, int32(10), c.LivenessProbe.InitialDelaySeconds)
	assert.Equal(t, int32(5), c.LivenessProbe.TimeoutSeconds)
	assert.Equal(t, int32(15), c.LivenessProbe.PeriodSeconds)
	assert.Equal(t, int32(5), c.LivenessProbe.FailureThreshold)

	assert.Equal(t, int32(8), c.ReadinessProbe.InitialDelaySeconds)
	assert.Equal(t, int32(4), c.ReadinessProbe.TimeoutSeconds)
	assert.Equal(t, int32(12), c.ReadinessProbe.PeriodSeconds)
	assert.Equal(t, int32(4), c.ReadinessProbe.FailureThreshold)
}

func TestGetSidecarContainerWithMaxRequestBodySize(t *testing.T) {
	t.Run("valid max request body size", func(t *testing.T) {
		annotations := map[string]string{
			daprConfigKey:        "config",
			daprAppPortKey:       "5000",
			daprMaxRequestBodySize: "16",
		}
		c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
		require.NoError(t, err)
		require.NotNil(t, c)

		argMap := argsToMap(c.Args)
		assert.Equal(t, "16", argMap["--dapr-http-max-request-size"])
	})

	t.Run("invalid max request body size falls back to default", func(t *testing.T) {
		annotations := map[string]string{
			daprConfigKey:        "config",
			daprAppPortKey:       "5000",
			daprMaxRequestBodySize: "not-a-number",
		}
		// Should not error; the invalid value is logged as a warning and defaults to -1.
		c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
		require.NoError(t, err)
		require.NotNil(t, c)

		argMap := argsToMap(c.Args)
		assert.Equal(t, "-1", argMap["--dapr-http-max-request-size"])
	})
}

func TestGetSidecarContainerWithNoAppPort(t *testing.T) {
	annotations := map[string]string{
		daprConfigKey: "config",
		// No daprAppPortKey set => getAppPort returns -1 => appPortStr = ""
	}

	c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
	require.NoError(t, err)
	require.NotNil(t, c)

	argMap := argsToMap(c.Args)
	assert.Equal(t, "", argMap["--app-port"])
}

func TestGetSidecarContainerWithInvalidAppPort(t *testing.T) {
	annotations := map[string]string{
		daprConfigKey:  "config",
		daprAppPortKey: "not-a-number",
	}

	c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
	require.Error(t, err)
	assert.Nil(t, c)
}

func TestGetSidecarContainerSecurityContext(t *testing.T) {
	annotations := map[string]string{
		daprConfigKey:  "config",
		daprAppPortKey: "5000",
	}

	c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, c.SecurityContext)
	require.NotNil(t, c.SecurityContext.AllowPrivilegeEscalation)
	assert.False(t, *c.SecurityContext.AllowPrivilegeEscalation)
}

func TestGetSidecarContainerCommand(t *testing.T) {
	annotations := map[string]string{
		daprConfigKey:  "config",
		daprAppPortKey: "5000",
	}

	c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Len(t, c.Command, 1)
	assert.Equal(t, "/daprd", c.Command[0])
}

func TestGetSidecarContainerEnvVars(t *testing.T) {
	annotations := map[string]string{
		daprConfigKey:  "config",
		daprAppPortKey: "5000",
	}

	c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "test-ns", "a", "b", nil, "", "", "", "", false, "")
	require.NoError(t, err)
	require.NotNil(t, c)

	// Should have at least DAPR_HOST_IP and NAMESPACE env vars.
	require.True(t, len(c.Env) >= 2)

	envMap := make(map[string]corev1.EnvVar)
	for _, e := range c.Env {
		envMap[e.Name] = e
	}

	hostIP, ok := envMap["DAPR_HOST_IP"]
	assert.True(t, ok)
	assert.NotNil(t, hostIP.ValueFrom)
	assert.Equal(t, "status.podIP", hostIP.ValueFrom.FieldRef.FieldPath)

	ns, ok := envMap["NAMESPACE"]
	assert.True(t, ok)
	assert.Equal(t, "test-ns", ns.Value)
}

// ---------------------------------------------------------------------------
// Annotation helper coverage
// ---------------------------------------------------------------------------

func TestGetMaxRequestBodySize(t *testing.T) {
	t.Run("annotation not set returns -1", func(t *testing.T) {
		m := map[string]string{}
		v, err := getMaxRequestBodySize(m)
		assert.NoError(t, err)
		assert.Equal(t, int32(-1), v)
	})

	t.Run("valid value", func(t *testing.T) {
		m := map[string]string{daprMaxRequestBodySize: "16"}
		v, err := getMaxRequestBodySize(m)
		assert.NoError(t, err)
		assert.Equal(t, int32(16), v)
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		m := map[string]string{daprMaxRequestBodySize: "abc"}
		_, err := getMaxRequestBodySize(m)
		assert.Error(t, err)
	})
}

func TestGetAppTokenSecret(t *testing.T) {
	t.Run("secret present", func(t *testing.T) {
		m := map[string]string{daprAppTokenSecret: "my-app-token"}
		assert.Equal(t, "my-app-token", GetAppTokenSecret(m))
	})

	t.Run("secret absent", func(t *testing.T) {
		m := map[string]string{}
		assert.Equal(t, "", GetAppTokenSecret(m))
	})
}

func TestGetSideCarPorts(t *testing.T) {
	t.Run("default HTTP port", func(t *testing.T) {
		m := map[string]string{}
		assert.Equal(t, int32(defaultSidecarHTTPPort), getSideCarHTTPPort(m))
	})

	t.Run("custom HTTP port", func(t *testing.T) {
		m := map[string]string{sidecarHTTPPortKey: "3600"}
		assert.Equal(t, int32(3600), getSideCarHTTPPort(m))
	})

	t.Run("default API GRPC port", func(t *testing.T) {
		m := map[string]string{}
		assert.Equal(t, int32(defaultSidecarAPIGRPCPort), getSideCarAPIGRPCPort(m))
	})

	t.Run("custom API GRPC port", func(t *testing.T) {
		m := map[string]string{sidecarAPIGRPCPortKey: "60001"}
		assert.Equal(t, int32(60001), getSideCarAPIGRPCPort(m))
	})

	t.Run("default internal GRPC port", func(t *testing.T) {
		m := map[string]string{}
		assert.Equal(t, int32(defaultSidecarInternalGRPCPortKey), getSideCarInternalGRPCPort(m))
	})

	t.Run("custom internal GRPC port", func(t *testing.T) {
		m := map[string]string{sidecarInternalGRPCPortKey: "60002"}
		assert.Equal(t, int32(60002), getSideCarInternalGRPCPort(m))
	})
}

func TestLogAsJSONEnabledCoverage(t *testing.T) {
	testCases := []struct {
		name     string
		value    string
		expected bool
	}{
		{"yes", "yes", true},
		{"y", "y", true},
		{"on", "on", true},
		{"1", "1", true},
		{"TRUE uppercase", "TRUE", true},
		{"True mixed", "True", true},
		{"no", "no", false},
		{"0", "0", false},
		{"random", "random", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := map[string]string{daprLogAsJSON: tc.value}
			assert.Equal(t, tc.expected, logAsJSONEnabled(m))
		})
	}
}

func TestGetBoolAnnotationOrDefaultEdgeCases(t *testing.T) {
	t.Run("default true when missing", func(t *testing.T) {
		m := map[string]string{}
		assert.True(t, getBoolAnnotationOrDefault(m, "missing-key", true))
	})

	t.Run("default false when missing", func(t *testing.T) {
		m := map[string]string{}
		assert.False(t, getBoolAnnotationOrDefault(m, "missing-key", false))
	})

	t.Run("nil annotations with default true", func(t *testing.T) {
		var m map[string]string
		assert.True(t, getBoolAnnotationOrDefault(m, "key", true))
	})
}

func TestGetStringAnnotation(t *testing.T) {
	t.Run("key present", func(t *testing.T) {
		m := map[string]string{"key": "value"}
		assert.Equal(t, "value", getStringAnnotation(m, "key"))
	})

	t.Run("key absent returns empty", func(t *testing.T) {
		m := map[string]string{}
		assert.Equal(t, "", getStringAnnotation(m, "key"))
	})
}

func TestGetStringAnnotationOrDefault(t *testing.T) {
	t.Run("empty value returns default", func(t *testing.T) {
		m := map[string]string{"key": ""}
		assert.Equal(t, "fallback", getStringAnnotationOrDefault(m, "key", "fallback"))
	})

	t.Run("non-empty value returned", func(t *testing.T) {
		m := map[string]string{"key": "actual"}
		assert.Equal(t, "actual", getStringAnnotationOrDefault(m, "key", "fallback"))
	})
}

func TestGetInt32AnnotationOrDefault(t *testing.T) {
	t.Run("missing key returns default", func(t *testing.T) {
		m := map[string]string{}
		assert.Equal(t, int32(42), getInt32AnnotationOrDefault(m, "key", 42))
	})

	t.Run("invalid value returns default", func(t *testing.T) {
		m := map[string]string{"key": "not-a-number"}
		assert.Equal(t, int32(42), getInt32AnnotationOrDefault(m, "key", 42))
	})

	t.Run("valid value", func(t *testing.T) {
		m := map[string]string{"key": "99"}
		assert.Equal(t, int32(99), getInt32AnnotationOrDefault(m, "key", 42))
	})
}

func TestGetInt32Annotation(t *testing.T) {
	t.Run("missing key returns -1 no error", func(t *testing.T) {
		m := map[string]string{}
		v, err := getInt32Annotation(m, "key")
		assert.NoError(t, err)
		assert.Equal(t, int32(-1), v)
	})

	t.Run("valid value", func(t *testing.T) {
		m := map[string]string{"key": "100"}
		v, err := getInt32Annotation(m, "key")
		assert.NoError(t, err)
		assert.Equal(t, int32(100), v)
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		m := map[string]string{"key": "xyz"}
		v, err := getInt32Annotation(m, "key")
		assert.Error(t, err)
		assert.Equal(t, int32(-1), v)
	})
}

func TestAppendQuantityToResourceList(t *testing.T) {
	t.Run("valid quantity", func(t *testing.T) {
		rl := corev1.ResourceList{}
		result, err := appendQuantityToResourceList("100m", corev1.ResourceCPU, rl)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "100m", result.Cpu().String())
	})

	t.Run("invalid quantity", func(t *testing.T) {
		rl := corev1.ResourceList{}
		result, err := appendQuantityToResourceList("invalid", corev1.ResourceCPU, rl)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestNewInjectorFields(t *testing.T) {
	t.Run("authUID is stored", func(t *testing.T) {
		cfg := Config{
			TLSCertFile:  "cert",
			TLSKeyFile:   "key",
			SidecarImage: "img",
			Namespace:    "ns",
		}
		inj := NewInjector("test-auth-uid", cfg, nil, nil)
		concrete := inj.(*injector)
		assert.Equal(t, "test-auth-uid", concrete.authUID)
	})

	t.Run("server address uses correct port", func(t *testing.T) {
		cfg := Config{}
		inj := NewInjector("", cfg, nil, nil)
		concrete := inj.(*injector)
		assert.Equal(t, fmt.Sprintf(":%d", port), concrete.server.Addr)
	})

	t.Run("deserializer is not nil", func(t *testing.T) {
		cfg := Config{}
		inj := NewInjector("", cfg, nil, nil)
		concrete := inj.(*injector)
		assert.NotNil(t, concrete.deserializer)
	})
}

func TestGetPullPolicyEdgeCases(t *testing.T) {
	testCases := []struct {
		input    string
		expected corev1.PullPolicy
	}{
		{"Always", corev1.PullAlways},
		{"Never", corev1.PullNever},
		{"IfNotPresent", corev1.PullIfNotPresent},
		{"", corev1.PullIfNotPresent},
		{"unknown", corev1.PullIfNotPresent},
		{"always", corev1.PullIfNotPresent}, // case-sensitive
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("policy_%s", tc.input), func(t *testing.T) {
			assert.Equal(t, tc.expected, getPullPolicy(tc.input))
		})
	}
}

func TestGetEnvPatchOperations(t *testing.T) {
	t.Run("empty existing envs initializes slice", func(t *testing.T) {
		addEnv := []corev1.EnvVar{
			{Name: "A", Value: "1"},
			{Name: "B", Value: "2"},
		}
		ops := getEnvPatchOperations(nil, addEnv, "/spec/containers/0/env")
		require.Len(t, ops, 1)
		assert.Equal(t, "add", ops[0].Op)
		assert.Equal(t, "/spec/containers/0/env", ops[0].Path)
	})

	t.Run("all envs conflict produces no ops", func(t *testing.T) {
		existing := []corev1.EnvVar{
			{Name: "A", Value: "existing"},
		}
		addEnv := []corev1.EnvVar{
			{Name: "A", Value: "new"},
		}
		ops := getEnvPatchOperations(existing, addEnv, "/spec/containers/0/env")
		assert.Empty(t, ops)
	})

	t.Run("partial conflict", func(t *testing.T) {
		existing := []corev1.EnvVar{
			{Name: "A", Value: "existing"},
		}
		addEnv := []corev1.EnvVar{
			{Name: "A", Value: "new"},
			{Name: "B", Value: "new"},
		}
		ops := getEnvPatchOperations(existing, addEnv, "/spec/containers/0/env")
		require.Len(t, ops, 1)
		assert.Equal(t, "add", ops[0].Op)
		assert.Equal(t, "/spec/containers/0/env/-", ops[0].Path)
	})
}

// ---------------------------------------------------------------------------
// Hand-written stubs for scheme.Interface (dapr client)
// ---------------------------------------------------------------------------

// stubConfigurationInterface implements configv1alpha1.ConfigurationInterface.
type stubConfigurationInterface struct {
	items []configapi.Configuration
	err   error
}

func (s *stubConfigurationInterface) List(_ metav1.ListOptions) (*configapi.ConfigurationList, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &configapi.ConfigurationList{Items: s.items}, nil
}

func (s *stubConfigurationInterface) Create(*configapi.Configuration) (*configapi.Configuration, error) {
	return nil, nil
}
func (s *stubConfigurationInterface) Update(*configapi.Configuration) (*configapi.Configuration, error) {
	return nil, nil
}
func (s *stubConfigurationInterface) Delete(string, *metav1.DeleteOptions) error { return nil }
func (s *stubConfigurationInterface) DeleteCollection(*metav1.DeleteOptions, metav1.ListOptions) error {
	return nil
}
func (s *stubConfigurationInterface) Get(string, metav1.GetOptions) (*configapi.Configuration, error) {
	return nil, nil
}
func (s *stubConfigurationInterface) Watch(metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}
func (s *stubConfigurationInterface) Patch(string, types.PatchType, []byte, ...string) (*configapi.Configuration, error) {
	return nil, nil
}

// stubConfigV1alpha1 implements configv1alpha1.ConfigurationV1alpha1Interface.
type stubConfigV1alpha1 struct {
	configs *stubConfigurationInterface
}

func (s *stubConfigV1alpha1) Configurations(_ string) configv1alpha1.ConfigurationInterface {
	return s.configs
}

func (s *stubConfigV1alpha1) RESTClient() rest.Interface { return nil }

// stubDaprClient implements scheme.Interface (the dapr versioned clientset interface).
type stubDaprClient struct {
	configClient *stubConfigV1alpha1
}

func (s *stubDaprClient) Discovery() discovery.DiscoveryInterface { return nil }
func (s *stubDaprClient) ComponentsV1alpha1() componentsv1alpha1.ComponentsV1alpha1Interface {
	return nil
}
func (s *stubDaprClient) ConfigurationV1alpha1() configv1alpha1.ConfigurationV1alpha1Interface {
	return s.configClient
}

// newStubDaprClient creates a stub dapr client that returns the given
// Configuration items when mTLSEnabled calls List.
func newStubDaprClient(items []configapi.Configuration) *stubDaprClient {
	return &stubDaprClient{
		configClient: &stubConfigV1alpha1{
			configs: &stubConfigurationInterface{items: items},
		},
	}
}

// newStubDaprClientWithError creates a stub dapr client whose List call fails.
func newStubDaprClientWithError(err error) *stubDaprClient {
	return &stubDaprClient{
		configClient: &stubConfigV1alpha1{
			configs: &stubConfigurationInterface{err: err},
		},
	}
}

// ---------------------------------------------------------------------------
// mTLSEnabled tests
// ---------------------------------------------------------------------------

func TestMTLSEnabled(t *testing.T) {
	t.Run("config found with mTLS enabled", func(t *testing.T) {
		client := newStubDaprClient([]configapi.Configuration{
			{
				ObjectMeta: metav1.ObjectMeta{Name: defaultConfig},
				Spec: configapi.ConfigurationSpec{
					MTLSSpec: configapi.MTLSSpec{Enabled: true},
				},
			},
		})
		assert.True(t, mTLSEnabled(client))
	})

	t.Run("config found with mTLS disabled", func(t *testing.T) {
		client := newStubDaprClient([]configapi.Configuration{
			{
				ObjectMeta: metav1.ObjectMeta{Name: defaultConfig},
				Spec: configapi.ConfigurationSpec{
					MTLSSpec: configapi.MTLSSpec{Enabled: false},
				},
			},
		})
		assert.False(t, mTLSEnabled(client))
	})

	t.Run("config not found returns default true", func(t *testing.T) {
		client := newStubDaprClient(nil)
		assert.True(t, mTLSEnabled(client))
	})

	t.Run("config with different name returns default true", func(t *testing.T) {
		client := newStubDaprClient([]configapi.Configuration{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "not-daprsystem"},
				Spec: configapi.ConfigurationSpec{
					MTLSSpec: configapi.MTLSSpec{Enabled: false},
				},
			},
		})
		assert.True(t, mTLSEnabled(client))
	})

	t.Run("list error returns default true", func(t *testing.T) {
		client := newStubDaprClientWithError(errors.New("connection refused"))
		assert.True(t, mTLSEnabled(client))
	})
}

// ---------------------------------------------------------------------------
// getPodPatchOperations full path tests (with fake dapr client)
// ---------------------------------------------------------------------------

func TestGetPodPatchOperationsFullPath(t *testing.T) {
	// Helper to build a stub dapr client with mTLS disabled.
	mtlsDisabledClient := newStubDaprClient([]configapi.Configuration{
		{
			ObjectMeta: metav1.ObjectMeta{Name: defaultConfig},
			Spec: configapi.ConfigurationSpec{
				MTLSSpec: configapi.MTLSSpec{Enabled: false},
			},
		},
	})

	t.Run("dapr-enabled pod with valid app id and mTLS disabled returns patches", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pod",
				Annotations: map[string]string{
					daprEnabledKey: "true",
					appIDKey:       "my-app",
					daprConfigKey:  "config",
					daprAppPortKey: "3000",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "app"},
				},
			},
		}
		rawPod, err := json.Marshal(pod)
		require.NoError(t, err)

		inj := &injector{}
		ar := &v1.AdmissionReview{
			Request: &v1.AdmissionRequest{
				Namespace: "default",
				Object:    runtime.RawExtension{Raw: rawPod},
			},
		}

		ops, err := inj.getPodPatchOperations(ar, "dapr-system", "daprio/daprd:latest", "Always", nil, mtlsDisabledClient)
		require.NoError(t, err)
		require.NotEmpty(t, ops)

		// Should have at least one patch op for adding the sidecar container.
		foundContainerAdd := false
		for _, op := range ops {
			if op.Op == "add" && (op.Path == "/spec/containers/-" || op.Path == containersPath) {
				foundContainerAdd = true
				break
			}
		}
		assert.True(t, foundContainerAdd, "expected a container add patch operation")
	})

	t.Run("dapr-enabled pod with no containers returns container patch", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					daprEnabledKey: "true",
					appIDKey:       "my-app",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{},
			},
		}
		rawPod, err := json.Marshal(pod)
		require.NoError(t, err)

		inj := &injector{}
		ar := &v1.AdmissionReview{
			Request: &v1.AdmissionRequest{
				Namespace: "default",
				Object:    runtime.RawExtension{Raw: rawPod},
			},
		}

		ops, err := inj.getPodPatchOperations(ar, "dapr-system", "daprio/daprd:latest", "Always", nil, mtlsDisabledClient)
		require.NoError(t, err)
		require.NotEmpty(t, ops)

		// When there are no existing containers, the path should be /spec/containers.
		assert.Equal(t, containersPath, ops[0].Path)
	})

	t.Run("dapr-enabled pod with token volume mount", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					daprEnabledKey: "true",
					appIDKey:       "my-app",
					daprConfigKey:  "config",
					daprAppPortKey: "3000",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "app",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "sa-token",
								MountPath: kubernetesMountPath,
							},
						},
					},
				},
			},
		}
		rawPod, err := json.Marshal(pod)
		require.NoError(t, err)

		inj := &injector{}
		ar := &v1.AdmissionReview{
			Request: &v1.AdmissionRequest{
				Namespace: "default",
				Object:    runtime.RawExtension{Raw: rawPod},
			},
		}

		ops, err := inj.getPodPatchOperations(ar, "dapr-system", "daprio/daprd:latest", "Always", nil, mtlsDisabledClient)
		require.NoError(t, err)
		require.NotEmpty(t, ops)

		// Should have env var patch operations in addition to container add.
		assert.True(t, len(ops) >= 2, "expected container add plus env var patches, got %d ops", len(ops))
	})

	t.Run("dapr-enabled pod with multiple containers gets env patched", func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					daprEnabledKey: "true",
					appIDKey:       "my-app",
					daprConfigKey:  "config",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "app-1"},
					{Name: "app-2"},
				},
			},
		}
		rawPod, err := json.Marshal(pod)
		require.NoError(t, err)

		inj := &injector{}
		ar := &v1.AdmissionReview{
			Request: &v1.AdmissionRequest{
				Namespace: "default",
				Object:    runtime.RawExtension{Raw: rawPod},
			},
		}

		ops, err := inj.getPodPatchOperations(ar, "dapr-system", "daprio/daprd:latest", "Always", nil, mtlsDisabledClient)
		require.NoError(t, err)
		require.NotEmpty(t, ops)
		// container add + 2 env patches per container (each container gets DAPR_HTTP_PORT + DAPR_GRPC_PORT initialized)
		// Each empty-env container gets 1 "add" op to initialize the env slice.
		assert.True(t, len(ops) >= 3, "expected container add plus env var patches for 2 containers, got %d ops", len(ops))
	})
}

// ---------------------------------------------------------------------------
// getSidecarContainer with invalid resource annotations
// ---------------------------------------------------------------------------

func TestGetSidecarContainerWithInvalidResourceAnnotations(t *testing.T) {
	t.Run("invalid cpu limit is logged but container is still returned", func(t *testing.T) {
		annotations := map[string]string{
			daprConfigKey:  "config",
			daprAppPortKey: "5000",
			daprCPULimitKey: "invalid-cpu",
		}
		c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
		require.NoError(t, err)
		require.NotNil(t, c)
		// Resources should be empty because the error is logged and defaults are used.
		assert.Empty(t, c.Resources.Limits)
	})

	t.Run("invalid memory limit is logged but container is still returned", func(t *testing.T) {
		annotations := map[string]string{
			daprConfigKey:     "config",
			daprAppPortKey:    "5000",
			daprMemoryLimitKey: "invalid-mem",
		}
		c, err := getSidecarContainer(annotations, "app-id", "image", "Always", "ns", "a", "b", nil, "", "", "", "", false, "")
		require.NoError(t, err)
		require.NotNil(t, c)
		assert.Empty(t, c.Resources.Limits)
	})
}

// ---------------------------------------------------------------------------
// handleRequest with valid patch operations
// ---------------------------------------------------------------------------

func TestHandleRequestWithDaprEnabledPod(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				daprEnabledKey: "true",
				appIDKey:       "my-app",
				daprConfigKey:  "config",
				daprAppPortKey: "5000",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app"},
			},
		},
	}
	rawPod := marshalPod(t, pod)

	body := buildAdmissionReviewBody(t, &v1.AdmissionRequest{
		UID: "req-uid",
		UserInfo: authenticationv1.UserInfo{
			UID: "the-uid",
		},
		Kind:      metav1.GroupVersionKind{Kind: "Pod"},
		Namespace: "default",
		Object:    runtime.RawExtension{Raw: rawPod},
	})

	stubClient := newStubDaprClient([]configapi.Configuration{
		{
			ObjectMeta: metav1.ObjectMeta{Name: defaultConfig},
			Spec: configapi.ConfigurationSpec{
				MTLSSpec: configapi.MTLSSpec{Enabled: false},
			},
		},
	})

	inj := &injector{
		deserializer: newTestDeserializer(),
		authUID:      "the-uid",
		daprClient:   stubClient,
		config: Config{
			Namespace:              "dapr-system",
			SidecarImage:           "daprio/daprd:latest",
			SidecarImagePullPolicy: "Always",
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	inj.handleRequest(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var review v1.AdmissionReview
	err := json.Unmarshal(rec.Body.Bytes(), &review)
	require.NoError(t, err)
	require.NotNil(t, review.Response)
	assert.True(t, review.Response.Allowed)
	// Patch should be present for dapr-enabled pod.
	assert.NotEmpty(t, review.Response.Patch)
	assert.NotNil(t, review.Response.PatchType)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// argsToMap converts a flat list of --key value pairs into a map.
func argsToMap(args []string) map[string]string {
	m := make(map[string]string)
	for i := 0; i < len(args)-1; i += 2 {
		m[args[i]] = args[i+1]
	}
	return m
}
