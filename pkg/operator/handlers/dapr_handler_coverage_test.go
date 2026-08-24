// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package handlers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fake_client "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// newTestScheme registers the core and apps/v1 types needed by the handler.
func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(s))
	require.NoError(t, appsv1.AddToScheme(s))
	return s
}

// newDaprHandlerWithClient builds a DaprHandler wired to the given client and scheme.
func newDaprHandlerWithClient(c client.Client, s *runtime.Scheme) *DaprHandler {
	return &DaprHandler{
		Client: c,
		Scheme: s,
	}
}

// getDeploymentWithSelector returns a Deployment that has a label selector and
// a UID, both of which are required for createDaprService / SetControllerReference.
func getDeploymentWithSelector(appID, daprEnabled, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "app",
			Namespace: namespace,
			UID:       types.UID("test-uid"),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &meta_v1.LabelSelector{
				MatchLabels: map[string]string{"app": "test_app"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta_v1.ObjectMeta{
					Labels: map[string]string{"app": "test_app"},
					Annotations: map[string]string{
						appIDAnnotationKey:       appID,
						daprEnabledAnnotationKey: daprEnabled,
					},
				},
			},
		},
	}
}

// errOnCreateClient wraps a real client.Client and forces Create to fail.
type errOnCreateClient struct {
	client.Client
}

func (c *errOnCreateClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return fmt.Errorf("simulated create error")
}

// errOnGetClient wraps a real client.Client and forces Get to fail with a
// non-NotFound error.
type errOnGetClient struct {
	client.Client
}

func (c *errOnGetClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	return fmt.Errorf("simulated get error")
}

// errOnListClient wraps a real client.Client and forces List to fail.
type errOnListClient struct {
	client.Client
}

func (c *errOnListClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return fmt.Errorf("simulated list error")
}

// errOnDeleteClient wraps a real client.Client and forces Delete to fail with a
// non-NotFound error.
type errOnDeleteClient struct {
	client.Client
}

func (c *errOnDeleteClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return fmt.Errorf("simulated delete error")
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestDaprServiceName(t *testing.T) {
	tests := []struct {
		name  string
		appID string
		want  string
	}{
		{"simple app name", "myapp", "myapp-dapr"},
		{"hyphenated name", "my-app", "my-app-dapr"},
		{"empty string", "", "-dapr"},
		{"numeric prefix", "123app", "123app-dapr"},
	}

	h := getTestDaprHandler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.daprServiceName(tt.appID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateDaprService(t *testing.T) {
	t.Run("creates service successfully", func(t *testing.T) {
		s := newTestScheme(t)
		c := fake_client.NewClientBuilder().WithScheme(s).Build()
		h := newDaprHandlerWithClient(c, s)

		dep := getDeploymentWithSelector("myapp", "true", "default")
		svcName := types.NamespacedName{Namespace: "default", Name: "myapp-dapr"}

		err := h.createDaprService(context.TODO(), svcName, dep)
		require.NoError(t, err)

		// Verify the service was actually created in the fake store.
		var svc corev1.Service
		err = c.Get(context.TODO(), svcName, &svc)
		require.NoError(t, err)
		assert.Equal(t, "myapp-dapr", svc.Name)
		assert.Equal(t, "default", svc.Namespace)
		assert.Equal(t, clusterIPNone, svc.Spec.ClusterIP)
		assert.Len(t, svc.Spec.Ports, 4)

		// Verify annotations.
		assert.Equal(t, "myapp", svc.Annotations[appIDAnnotationKey])
		assert.Equal(t, "true", svc.Annotations["prometheus.io/scrape"])
	})

	t.Run("returns error when client create fails", func(t *testing.T) {
		s := newTestScheme(t)
		real := fake_client.NewClientBuilder().WithScheme(s).Build()
		c := &errOnCreateClient{Client: real}
		h := newDaprHandlerWithClient(c, s)

		dep := getDeploymentWithSelector("myapp", "true", "default")
		svcName := types.NamespacedName{Namespace: "default", Name: "myapp-dapr"}

		err := h.createDaprService(context.TODO(), svcName, dep)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "simulated create error")
	})
}

func TestEnsureDaprServiceAbsent(t *testing.T) {
	t.Run("deletes existing service", func(t *testing.T) {
		s := newTestScheme(t)

		// Pre-create a service that looks like a Dapr sidecar service.
		existingSvc := &corev1.Service{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      "myapp-dapr",
				Namespace: "default",
				Annotations: map[string]string{
					appIDAnnotationKey: "myapp",
				},
			},
		}

		c := fake_client.NewClientBuilder().WithScheme(s).WithObjects(existingSvc).Build()
		h := newDaprHandlerWithClient(c, s)

		key := types.NamespacedName{Namespace: "default", Name: "app"}
		err := h.ensureDaprServiceAbsent(context.TODO(), key)
		require.NoError(t, err)

		// The fake client's List ignores MatchingFields (no indexer in
		// v0.7.0), so it returns all services in the namespace and the
		// handler deletes them. Verify it is gone.
		var svc corev1.Service
		getErr := c.Get(context.TODO(), types.NamespacedName{
			Namespace: "default", Name: "myapp-dapr",
		}, &svc)
		assert.Error(t, getErr, "service should have been deleted")
	})

	t.Run("no-op when no services exist", func(t *testing.T) {
		s := newTestScheme(t)
		c := fake_client.NewClientBuilder().WithScheme(s).Build()
		h := newDaprHandlerWithClient(c, s)

		key := types.NamespacedName{Namespace: "default", Name: "app"}
		err := h.ensureDaprServiceAbsent(context.TODO(), key)
		assert.NoError(t, err)
	})
}

func TestReconcile(t *testing.T) {
	t.Run("deployment with dapr annotations creates service", func(t *testing.T) {
		s := newTestScheme(t)
		dep := getDeploymentWithSelector("myapp", "true", "default")

		c := fake_client.NewClientBuilder().WithScheme(s).WithObjects(dep).Build()
		h := newDaprHandlerWithClient(c, s)

		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "app",
			},
		}

		result, err := h.Reconcile(context.TODO(), req)
		require.NoError(t, err)
		assert.False(t, result.Requeue)

		// The service should now exist.
		var svc corev1.Service
		err = c.Get(context.TODO(), types.NamespacedName{
			Namespace: "default", Name: "myapp-dapr",
		}, &svc)
		require.NoError(t, err)
		assert.Equal(t, "myapp-dapr", svc.Name)
	})

	t.Run("deployment without dapr annotations triggers absent path", func(t *testing.T) {
		s := newTestScheme(t)
		dep := getDeploymentWithSelector("myapp", "false", "default")

		c := fake_client.NewClientBuilder().WithScheme(s).WithObjects(dep).Build()
		h := newDaprHandlerWithClient(c, s)

		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "app",
			},
		}

		result, err := h.Reconcile(context.TODO(), req)
		require.NoError(t, err)
		assert.False(t, result.Requeue)
	})

	t.Run("missing deployment is not an error", func(t *testing.T) {
		s := newTestScheme(t)
		c := fake_client.NewClientBuilder().WithScheme(s).Build()
		h := newDaprHandlerWithClient(c, s)

		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "nonexistent",
			},
		}

		result, err := h.Reconcile(context.TODO(), req)
		assert.NoError(t, err)
		assert.False(t, result.Requeue)
	})

	t.Run("non-NotFound Get error is propagated", func(t *testing.T) {
		s := newTestScheme(t)
		real := fake_client.NewClientBuilder().WithScheme(s).Build()
		c := &errOnGetClient{Client: real}
		h := newDaprHandlerWithClient(c, s)

		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "app",
			},
		}

		result, err := h.Reconcile(context.TODO(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "simulated get error")
		assert.False(t, result.Requeue)
	})

	t.Run("deployment being deleted is a no-op", func(t *testing.T) {
		s := newTestScheme(t)
		dep := getDeploymentWithSelector("myapp", "true", "default")
		now := meta_v1.Now()
		dep.DeletionTimestamp = &now

		c := fake_client.NewClientBuilder().WithScheme(s).WithObjects(dep).Build()
		h := newDaprHandlerWithClient(c, s)

		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "app",
			},
		}

		result, err := h.Reconcile(context.TODO(), req)
		assert.NoError(t, err)
		assert.False(t, result.Requeue)

		// No service should have been created.
		var svc corev1.Service
		getErr := c.Get(context.TODO(), types.NamespacedName{
			Namespace: "default", Name: "myapp-dapr",
		}, &svc)
		assert.Error(t, getErr, "no service should be created for a deleting deployment")
	})
}

func TestEnsureDaprServicePresent_AlreadyExists(t *testing.T) {
	s := newTestScheme(t)
	dep := getDeploymentWithSelector("myapp", "true", "default")

	// Pre-create the service that ensureDaprServicePresent would create.
	existingSvc := &corev1.Service{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "myapp-dapr",
			Namespace: "default",
		},
	}

	c := fake_client.NewClientBuilder().WithScheme(s).WithObjects(existingSvc).Build()
	h := newDaprHandlerWithClient(c, s)

	err := h.ensureDaprServicePresent(context.TODO(), "default", dep)
	assert.NoError(t, err)

	// Verify the existing service is still there and was not recreated
	// (i.e. no extra ports were added by createDaprService).
	var svc corev1.Service
	require.NoError(t, c.Get(context.TODO(), types.NamespacedName{
		Namespace: "default", Name: "myapp-dapr",
	}, &svc))
	assert.Empty(t, svc.Spec.Ports, "pre-existing service should not have been mutated")
}

func TestEnsureDaprServicePresent_GetError(t *testing.T) {
	s := newTestScheme(t)
	real := fake_client.NewClientBuilder().WithScheme(s).Build()
	c := &errOnGetClient{Client: real}
	h := newDaprHandlerWithClient(c, s)

	dep := getDeploymentWithSelector("myapp", "true", "default")

	err := h.ensureDaprServicePresent(context.TODO(), "default", dep)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated get error")
}

func TestEnsureDaprServiceAbsent_ListError(t *testing.T) {
	s := newTestScheme(t)
	real := fake_client.NewClientBuilder().WithScheme(s).Build()
	c := &errOnListClient{Client: real}
	h := newDaprHandlerWithClient(c, s)

	key := types.NamespacedName{Namespace: "default", Name: "app"}
	err := h.ensureDaprServiceAbsent(context.TODO(), key)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated list error")
}

func TestEnsureDaprServiceAbsent_DeleteError(t *testing.T) {
	s := newTestScheme(t)

	existingSvc := &corev1.Service{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "myapp-dapr",
			Namespace: "default",
			Annotations: map[string]string{
				appIDAnnotationKey: "myapp",
			},
		},
	}

	real := fake_client.NewClientBuilder().WithScheme(s).WithObjects(existingSvc).Build()
	c := &errOnDeleteClient{Client: real}
	h := newDaprHandlerWithClient(c, s)

	key := types.NamespacedName{Namespace: "default", Name: "app"}
	err := h.ensureDaprServiceAbsent(context.TODO(), key)
	// The function logs the delete error but does not return it.
	assert.NoError(t, err)

	// The service should still exist because Delete was intercepted.
	var svc corev1.Service
	getErr := real.Get(context.TODO(), types.NamespacedName{
		Namespace: "default", Name: "myapp-dapr",
	}, &svc)
	assert.NoError(t, getErr, "service should still exist because delete failed")
}
