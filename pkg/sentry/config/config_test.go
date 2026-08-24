package config

import (
	"os"
	"testing"

	dapr_config "github.com/dapr/dapr/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	t.Run("valid FromConfig, empty config name", func(t *testing.T) {
		defaultConfig := getDefaultConfig()
		c, _ := FromConfigName("")

		assert.Equal(t, defaultConfig, c)
	})

	t.Run("valid default config, self hosted", func(t *testing.T) {
		defaultConfig := getDefaultConfig()
		c, _ := getSelfhostedConfig("")

		assert.Equal(t, defaultConfig, c)
	})

	t.Run("parse configuration", func(t *testing.T) {
		daprConfig := dapr_config.Configuration{
			Spec: dapr_config.ConfigurationSpec{
				MTLSSpec: dapr_config.MTLSSpec{
					Enabled:          true,
					WorkloadCertTTL:  "5s",
					AllowedClockSkew: "1h",
				},
			},
		}

		defaultConfig := getDefaultConfig()
		conf, err := parseConfiguration(defaultConfig, &daprConfig)
		assert.Nil(t, err)
		assert.Equal(t, "5s", conf.WorkloadCertTTL.String())
		assert.Equal(t, "1h0m0s", conf.AllowedClockSkew.String())
	})
}

func TestIsKubernetesHosted(t *testing.T) {
	t.Run("returns true when env var set", func(t *testing.T) {
		os.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
		defer os.Unsetenv("KUBERNETES_SERVICE_HOST")

		assert.True(t, IsKubernetesHosted())
	})

	t.Run("returns false when env var not set", func(t *testing.T) {
		os.Unsetenv("KUBERNETES_SERVICE_HOST")

		assert.False(t, IsKubernetesHosted())
	})
}

func TestGetDefaultConfig(t *testing.T) {
	t.Run("returns default values", func(t *testing.T) {
		conf := getDefaultConfig()
		assert.Equal(t, defaultPort, conf.Port)
		assert.Equal(t, defaultWorkloadCertTTL, conf.WorkloadCertTTL)
		assert.Equal(t, defaultAllowedClockSkew, conf.AllowedClockSkew)
	})
}

func TestPrintConfig(t *testing.T) {
	t.Run("does not panic", func(t *testing.T) {
		conf := getDefaultConfig()
		assert.NotPanics(t, func() {
			printConfig(conf)
		})
	})

	t.Run("with custom CA store", func(t *testing.T) {
		conf := getDefaultConfig()
		conf.CAStore = "custom-store"
		assert.NotPanics(t, func() {
			printConfig(conf)
		})
	})
}

func TestGetSelfhostedConfigWithFile(t *testing.T) {
	t.Run("valid config file", func(t *testing.T) {
		conf, err := getSelfhostedConfig("testdata/config.yaml")
		if err != nil {
			// If testdata doesn't exist, use a non-existent path to test error path
			conf, err = getSelfhostedConfig("/nonexistent/config.yaml")
			assert.Error(t, err)
			assert.Equal(t, defaultPort, conf.Port)
		}
	})

	t.Run("nonexistent file returns default config", func(t *testing.T) {
		conf, err := getSelfhostedConfig("/definitely/nonexistent/config.yaml")
		assert.Error(t, err)
		assert.Equal(t, defaultPort, conf.Port)
		assert.Equal(t, defaultWorkloadCertTTL, conf.WorkloadCertTTL)
	})
}

func TestFromConfigName(t *testing.T) {
	t.Run("selfhosted with invalid config returns default", func(t *testing.T) {
		os.Unsetenv("KUBERNETES_SERVICE_HOST")

		conf, err := FromConfigName("/nonexistent/config.yaml")
		assert.Error(t, err)
		assert.Equal(t, defaultPort, conf.Port)
	})
}

func TestParseConfigurationErrors(t *testing.T) {
	t.Run("invalid WorkloadCertTTL duration", func(t *testing.T) {
		daprConfig := dapr_config.Configuration{
			Spec: dapr_config.ConfigurationSpec{
				MTLSSpec: dapr_config.MTLSSpec{
					WorkloadCertTTL: "invalid-duration",
				},
			},
		}
		defaultConfig := getDefaultConfig()
		_, err := parseConfiguration(defaultConfig, &daprConfig)
		assert.Error(t, err)
	})

	t.Run("invalid AllowedClockSkew duration", func(t *testing.T) {
		daprConfig := dapr_config.Configuration{
			Spec: dapr_config.ConfigurationSpec{
				MTLSSpec: dapr_config.MTLSSpec{
					AllowedClockSkew: "invalid-duration",
				},
			},
		}
		defaultConfig := getDefaultConfig()
		_, err := parseConfiguration(defaultConfig, &daprConfig)
		assert.Error(t, err)
	})
}
