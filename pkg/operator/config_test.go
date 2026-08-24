package operator

import (
	"os"
	"testing"

	"github.com/dapr/dapr/pkg/apis/configuration/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fake_client "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLoadConfiguration(t *testing.T) {
	s := runtime.NewScheme()
	err := v1alpha1.AddToScheme(s)
	require.NoError(t, err)

	t.Run("returns config with mTLS enabled", func(t *testing.T) {
		conf := &v1alpha1.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-config",
				Namespace: "test-ns",
			},
			Spec: v1alpha1.ConfigurationSpec{
				MTLSSpec: v1alpha1.MTLSSpec{
					Enabled: true,
				},
			},
		}

		client := fake_client.NewClientBuilder().WithScheme(s).WithObjects(conf).Build()
		os.Setenv("NAMESPACE", "test-ns")
		defer os.Unsetenv("NAMESPACE")

		cfg, err := LoadConfiguration("test-config", client)
		require.NoError(t, err)
		assert.True(t, cfg.MTLSEnabled)
	})

	t.Run("returns config with mTLS disabled", func(t *testing.T) {
		conf := &v1alpha1.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-config",
				Namespace: "default",
			},
			Spec: v1alpha1.ConfigurationSpec{
				MTLSSpec: v1alpha1.MTLSSpec{
					Enabled: false,
				},
			},
		}

		client := fake_client.NewClientBuilder().WithScheme(s).WithObjects(conf).Build()
		os.Setenv("NAMESPACE", "default")
		defer os.Unsetenv("NAMESPACE")

		cfg, err := LoadConfiguration("test-config", client)
		require.NoError(t, err)
		assert.False(t, cfg.MTLSEnabled)
	})

	t.Run("returns error when config not found", func(t *testing.T) {
		client := fake_client.NewClientBuilder().WithScheme(s).Build()
		os.Setenv("NAMESPACE", "test-ns")
		defer os.Unsetenv("NAMESPACE")

		cfg, err := LoadConfiguration("nonexistent", client)
		assert.Error(t, err)
		assert.Nil(t, cfg)
	})
}
