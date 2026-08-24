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
			name: "Subscription kind",
			kind: "Subscription",
			expected: schema.GroupKind{
				Group: "dapr.io",
				Kind:  "Subscription",
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
			name:     "subscriptions resource",
			resource: "subscriptions",
			expected: schema.GroupResource{
				Group:    "dapr.io",
				Resource: "subscriptions",
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
	t.Run("registers Subscription and SubscriptionList types", func(t *testing.T) {
		s := runtime.NewScheme()
		err := SchemeBuilder.AddToScheme(s)
		require.NoError(t, err)

		// Verify Subscription is registered
		gvk := SchemeGroupVersion.WithKind("Subscription")
		obj, err := s.New(gvk)
		require.NoError(t, err)
		assert.IsType(t, &Subscription{}, obj)

		// Verify SubscriptionList is registered
		gvkList := SchemeGroupVersion.WithKind("SubscriptionList")
		objList, err := s.New(gvkList)
		require.NoError(t, err)
		assert.IsType(t, &SubscriptionList{}, objList)
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
