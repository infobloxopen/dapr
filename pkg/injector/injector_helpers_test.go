// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package injector

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
)

func TestToAdmissionResponse(t *testing.T) {
	t.Run("error message is set", func(t *testing.T) {
		err := errors.New("test error message")
		resp := toAdmissionResponse(err)

		require.NotNil(t, resp)
		require.NotNil(t, resp.Result)
		assert.Equal(t, "test error message", resp.Result.Message)
	})

	t.Run("Allowed is false by default", func(t *testing.T) {
		err := errors.New("some failure")
		resp := toAdmissionResponse(err)

		require.NotNil(t, resp)
		assert.False(t, resp.Allowed)
	})
}

func TestPodContainsSidecarContainer(t *testing.T) {
	t.Run("pod with daprd container returns true", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "app"},
					{Name: sidecarContainerName},
				},
			},
		}
		assert.True(t, podContainsSidecarContainer(pod))
	})

	t.Run("pod without daprd container returns false", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "app"},
					{Name: "other"},
				},
			},
		}
		assert.False(t, podContainsSidecarContainer(pod))
	})

	t.Run("pod with no containers returns false", func(t *testing.T) {
		pod := &corev1.Pod{}
		assert.False(t, podContainsSidecarContainer(pod))
	})
}

func TestIsResourceDaprEnabled(t *testing.T) {
	testCases := []struct {
		testName    string
		annotations map[string]string
		expected    bool
	}{
		{
			testName:    "enabled is true",
			annotations: map[string]string{daprEnabledKey: "true"},
			expected:    true,
		},
		{
			testName:    "enabled is false",
			annotations: map[string]string{daprEnabledKey: "false"},
			expected:    false,
		},
		{
			testName:    "annotation missing",
			annotations: map[string]string{},
			expected:    false,
		},
		{
			testName:    "nil annotations",
			annotations: nil,
			expected:    false,
		},
		{
			testName:    "enabled is yes",
			annotations: map[string]string{daprEnabledKey: "yes"},
			expected:    true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			assert.Equal(t, tc.expected, isResourceDaprEnabled(tc.annotations))
		})
	}
}

func TestGetTokenVolumeMount(t *testing.T) {
	t.Run("pod with service account token volume", func(t *testing.T) {
		pod := corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "app",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "token-vol",
								MountPath: kubernetesMountPath,
							},
						},
					},
				},
			},
		}
		mount := getTokenVolumeMount(pod)
		require.NotNil(t, mount)
		assert.Equal(t, kubernetesMountPath, mount.MountPath)
		assert.Equal(t, "token-vol", mount.Name)
	})

	t.Run("pod without service account token volume", func(t *testing.T) {
		pod := corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "app",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "config-vol",
								MountPath: "/etc/config",
							},
						},
					},
				},
			},
		}
		mount := getTokenVolumeMount(pod)
		assert.Nil(t, mount)
	})

	t.Run("pod with no volume mounts", func(t *testing.T) {
		pod := corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "app"},
				},
			},
		}
		mount := getTokenVolumeMount(pod)
		assert.Nil(t, mount)
	})

	t.Run("pod with no containers", func(t *testing.T) {
		pod := corev1.Pod{}
		mount := getTokenVolumeMount(pod)
		assert.Nil(t, mount)
	})

	t.Run("token volume in second container", func(t *testing.T) {
		pod := corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "first",
						VolumeMounts: []corev1.VolumeMount{
							{Name: "other", MountPath: "/other"},
						},
					},
					{
						Name: "second",
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
		mount := getTokenVolumeMount(pod)
		require.NotNil(t, mount)
		assert.Equal(t, "sa-token", mount.Name)
	})
}
