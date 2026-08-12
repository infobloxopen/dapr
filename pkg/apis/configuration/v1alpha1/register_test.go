// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestKind(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		expected schema.GroupKind
	}{
		{
			name: "Configuration kind",
			kind: "Configuration",
			expected: schema.GroupKind{
				Group: "dapr.io",
				Kind:  "Configuration",
			},
		},
		{
			name: "empty kind",
			kind: "",
			expected: schema.GroupKind{
				Group: "dapr.io",
				Kind:  "",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Kind(tc.kind)
			assert.Equal(t, tc.expected.Group, result.Group)
			assert.Equal(t, tc.expected.Kind, result.Kind)
		})
	}
}

func TestResource(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		expected schema.GroupResource
	}{
		{
			name:     "configurations resource",
			resource: "configurations",
			expected: schema.GroupResource{
				Group:    "dapr.io",
				Resource: "configurations",
			},
		},
		{
			name:     "empty resource",
			resource: "",
			expected: schema.GroupResource{
				Group:    "dapr.io",
				Resource: "",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Resource(tc.resource)
			assert.Equal(t, tc.expected.Group, result.Group)
			assert.Equal(t, tc.expected.Resource, result.Resource)
		})
	}
}

func TestSchemeGroupVersion(t *testing.T) {
	t.Run("has correct group and version", func(t *testing.T) {
		assert.Equal(t, "dapr.io", SchemeGroupVersion.Group)
		assert.Equal(t, "v1alpha1", SchemeGroupVersion.Version)
	})
}

func TestAddKnownTypes(t *testing.T) {
	t.Run("registers Configuration and ConfigurationList types", func(t *testing.T) {
		s := runtime.NewScheme()
		err := SchemeBuilder.AddToScheme(s)
		require.NoError(t, err)

		// Verify Configuration is registered
		gvk := SchemeGroupVersion.WithKind("Configuration")
		obj, err := s.New(gvk)
		require.NoError(t, err)
		assert.IsType(t, &Configuration{}, obj)

		// Verify ConfigurationList is registered
		gvkList := SchemeGroupVersion.WithKind("ConfigurationList")
		objList, err := s.New(gvkList)
		require.NoError(t, err)
		assert.IsType(t, &ConfigurationList{}, objList)
	})

	t.Run("unregistered kind returns error", func(t *testing.T) {
		s := runtime.NewScheme()
		err := SchemeBuilder.AddToScheme(s)
		require.NoError(t, err)

		gvk := SchemeGroupVersion.WithKind("Unknown")
		_, err = s.New(gvk)
		assert.Error(t, err)
	})
}
