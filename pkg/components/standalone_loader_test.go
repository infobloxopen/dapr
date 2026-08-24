package components

import (
	"io/ioutil"
	"os"
	"testing"

	config "github.com/dapr/dapr/pkg/config/modes"
	"github.com/stretchr/testify/assert"
)

func TestIsYaml(t *testing.T) {
	request := &StandaloneComponents{
		config: config.StandaloneConfig{
			ComponentsPath: "test_component_path",
		},
	}

	assert.True(t, request.isYaml("test.yaml"))
	assert.True(t, request.isYaml("test.YAML"))
	assert.True(t, request.isYaml("test.yml"))
	assert.True(t, request.isYaml("test.YML"))
	assert.False(t, request.isYaml("test.md"))
	assert.False(t, request.isYaml("test.txt"))
	assert.False(t, request.isYaml("test.sh"))
}

func TestStandaloneDecodeValidYaml(t *testing.T) {
	request := &StandaloneComponents{
		config: config.StandaloneConfig{
			ComponentsPath: "test_component_path",
		},
	}
	yaml := `
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
   name: statestore
spec:
   type: state.couchbase
   metadata:
   - name: prop1
     value: value1
   - name: prop2
     value: value2
`
	components, errs := request.decodeYaml("components/messagebus.yaml", []byte(yaml))
	assert.Len(t, components, 1)
	assert.Empty(t, errs)
	assert.Equal(t, "statestore", components[0].Name)
	assert.Equal(t, "state.couchbase", components[0].Spec.Type)
	assert.Len(t, components[0].Spec.Metadata, 2)
	assert.Equal(t, "prop1", components[0].Spec.Metadata[0].Name)
	assert.Equal(t, "value1", components[0].Spec.Metadata[0].Value.String())
}

func TestStandaloneDecodeInvalidComponent(t *testing.T) {
	request := &StandaloneComponents{
		config: config.StandaloneConfig{
			ComponentsPath: "test_component_path",
		},
	}
	yaml := `
apiVersion: dapr.io/v1alpha1
kind: Subscription
metadata:
   name: testsub
spec:
   metadata:
   - name: prop1
     value: value1
   - name: prop2
     value: value2
`
	components, errs := request.decodeYaml("components/messagebus.yaml", []byte(yaml))
	assert.Len(t, components, 0)
	assert.Len(t, errs, 0)
}

func TestStandaloneDecodeUnsuspectingFile(t *testing.T) {
	request := &StandaloneComponents{
		config: config.StandaloneConfig{
			ComponentsPath: "test_component_path",
		},
	}

	components, errs := request.decodeYaml("components/messagebus.yaml", []byte("hey there"))
	assert.Len(t, components, 0)
	assert.Len(t, errs, 0)
}

func TestStandaloneDecodeInvalidYaml(t *testing.T) {
	request := &StandaloneComponents{
		config: config.StandaloneConfig{
			ComponentsPath: "test_component_path",
		},
	}
	yaml := `
INVALID_YAML_HERE
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
name: statestore`
	components, errs := request.decodeYaml("components/messagebus.yaml", []byte(yaml))
	assert.Len(t, components, 0)
	assert.Len(t, errs, 0)
}

func TestStandaloneDecodeValidMultiYaml(t *testing.T) {
	request := &StandaloneComponents{
		config: config.StandaloneConfig{
			ComponentsPath: "test_component_path",
		},
	}
	yaml := `
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
    name: statestore1
spec:
  type: state.couchbase
  metadata:
    - name: prop1
      value: value1
    - name: prop2
      value: value2
---
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
    name: statestore2
spec:
  type: state.redis
  metadata:
    - name: prop3
      value: value3
`
	components, errs := request.decodeYaml("components/messagebus.yaml", []byte(yaml))
	assert.Len(t, components, 2)
	assert.Empty(t, errs)
	assert.Equal(t, "statestore1", components[0].Name)
	assert.Equal(t, "state.couchbase", components[0].Spec.Type)
	assert.Len(t, components[0].Spec.Metadata, 2)
	assert.Equal(t, "prop1", components[0].Spec.Metadata[0].Name)
	assert.Equal(t, "value1", components[0].Spec.Metadata[0].Value.String())
	assert.Equal(t, "prop2", components[0].Spec.Metadata[1].Name)
	assert.Equal(t, "value2", components[0].Spec.Metadata[1].Value.String())

	assert.Equal(t, "statestore2", components[1].Name)
	assert.Equal(t, "state.redis", components[1].Spec.Type)
	assert.Len(t, components[1].Spec.Metadata, 1)
	assert.Equal(t, "prop3", components[1].Spec.Metadata[0].Name)
	assert.Equal(t, "value3", components[1].Spec.Metadata[0].Value.String())
}

func TestStandaloneDecodeInValidDocInMultiYaml(t *testing.T) {
	request := &StandaloneComponents{
		config: config.StandaloneConfig{
			ComponentsPath: "test_component_path",
		},
	}
	yaml := `
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
    name: statestore1
spec:
  type: state.couchbase
  metadata:
    - name: prop1
      value: value1
    - name: prop2
      value: value2
---
INVALID_YAML_HERE
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
name: invalidyaml
---
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
    name: statestore2
spec:
  type: state.redis
  metadata:
    - name: prop3
      value: value3
`
	components, errs := request.decodeYaml("components/messagebus.yaml", []byte(yaml))
	assert.Len(t, components, 2)
	assert.Len(t, errs, 0)

	assert.Equal(t, "statestore1", components[0].Name)
	assert.Equal(t, "state.couchbase", components[0].Spec.Type)
	assert.Len(t, components[0].Spec.Metadata, 2)
	assert.Equal(t, "prop1", components[0].Spec.Metadata[0].Name)
	assert.Equal(t, "value1", components[0].Spec.Metadata[0].Value.String())
	assert.Equal(t, "prop2", components[0].Spec.Metadata[1].Name)
	assert.Equal(t, "value2", components[0].Spec.Metadata[1].Value.String())

	assert.Equal(t, "statestore2", components[1].Name)
	assert.Equal(t, "state.redis", components[1].Spec.Type)
	assert.Len(t, components[1].Spec.Metadata, 1)
	assert.Equal(t, "prop3", components[1].Spec.Metadata[0].Name)
	assert.Equal(t, "value3", components[1].Spec.Metadata[0].Value.String())
}

func TestNewStandaloneComponents(t *testing.T) {
	t.Run("creates standalone loader", func(t *testing.T) {
		cfg := config.StandaloneConfig{ComponentsPath: "/tmp/components"}
		loader := NewStandaloneComponents(cfg)
		assert.NotNil(t, loader)
		assert.Equal(t, "/tmp/components", loader.config.ComponentsPath)
	})
}

func TestStandaloneLoadComponents(t *testing.T) {
	t.Run("loads components from directory", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "components-test")
		assert.NoError(t, err)
		defer os.RemoveAll(dir)

		yamlContent := `
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
   name: statestore
spec:
   type: state.redis
   metadata:
   - name: host
     value: localhost
`
		err = ioutil.WriteFile(dir+"/component.yaml", []byte(yamlContent), 0644)
		assert.NoError(t, err)

		loader := NewStandaloneComponents(config.StandaloneConfig{ComponentsPath: dir})
		components, err := loader.LoadComponents()
		assert.NoError(t, err)
		assert.Len(t, components, 1)
		assert.Equal(t, "statestore", components[0].Name)
	})

	t.Run("empty directory returns empty list", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "components-empty")
		assert.NoError(t, err)
		defer os.RemoveAll(dir)

		loader := NewStandaloneComponents(config.StandaloneConfig{ComponentsPath: dir})
		components, err := loader.LoadComponents()
		assert.NoError(t, err)
		assert.Empty(t, components)
	})

	t.Run("invalid directory returns error", func(t *testing.T) {
		loader := NewStandaloneComponents(config.StandaloneConfig{ComponentsPath: "/nonexistent/path"})
		_, err := loader.LoadComponents()
		assert.Error(t, err)
	})

	t.Run("skips non-yaml files", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "components-nonyaml")
		assert.NoError(t, err)
		defer os.RemoveAll(dir)

		err = ioutil.WriteFile(dir+"/readme.md", []byte("# readme"), 0644)
		assert.NoError(t, err)

		loader := NewStandaloneComponents(config.StandaloneConfig{ComponentsPath: dir})
		components, err := loader.LoadComponents()
		assert.NoError(t, err)
		assert.Empty(t, components)
	})

	t.Run("loads multiple yaml files", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "components-multi")
		assert.NoError(t, err)
		defer os.RemoveAll(dir)

		yaml1 := `
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
   name: store1
spec:
   type: state.redis
`
		yaml2 := `
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
   name: store2
spec:
   type: state.couchbase
`
		err = ioutil.WriteFile(dir+"/comp1.yaml", []byte(yaml1), 0644)
		assert.NoError(t, err)
		err = ioutil.WriteFile(dir+"/comp2.yml", []byte(yaml2), 0644)
		assert.NoError(t, err)

		loader := NewStandaloneComponents(config.StandaloneConfig{ComponentsPath: dir})
		components, err := loader.LoadComponents()
		assert.NoError(t, err)
		assert.Len(t, components, 2)
	})

	t.Run("skips subdirectories", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "components-subdir")
		assert.NoError(t, err)
		defer os.RemoveAll(dir)

		err = os.Mkdir(dir+"/subdir", 0755)
		assert.NoError(t, err)

		yamlContent := `
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
   name: statestore
spec:
   type: state.redis
`
		err = ioutil.WriteFile(dir+"/comp.yaml", []byte(yamlContent), 0644)
		assert.NoError(t, err)
		// Also put a yaml in the subdir - it should not be picked up
		err = ioutil.WriteFile(dir+"/subdir/nested.yaml", []byte(yamlContent), 0644)
		assert.NoError(t, err)

		loader := NewStandaloneComponents(config.StandaloneConfig{ComponentsPath: dir})
		components, err := loader.LoadComponents()
		assert.NoError(t, err)
		assert.Len(t, components, 1)
	})

	t.Run("loads multi-doc yaml file", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "components-multidoc")
		assert.NoError(t, err)
		defer os.RemoveAll(dir)

		yamlContent := `
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
   name: store1
spec:
   type: state.redis
---
apiVersion: dapr.io/v1alpha1
kind: Component
metadata:
   name: store2
spec:
   type: state.couchbase
`
		err = ioutil.WriteFile(dir+"/multi.yaml", []byte(yamlContent), 0644)
		assert.NoError(t, err)

		loader := NewStandaloneComponents(config.StandaloneConfig{ComponentsPath: dir})
		components, err := loader.LoadComponents()
		assert.NoError(t, err)
		assert.Len(t, components, 2)
		assert.Equal(t, "store1", components[0].Name)
		assert.Equal(t, "store2", components[1].Name)
	})
}

func TestSplitYamlDoc(t *testing.T) {
	s := &StandaloneComponents{
		config: config.StandaloneConfig{
			ComponentsPath: "test_component_path",
		},
	}

	t.Run("empty data at EOF", func(t *testing.T) {
		advance, token, err := s.splitYamlDoc([]byte{}, true)
		assert.NoError(t, err)
		assert.Equal(t, 0, advance)
		assert.Nil(t, token)
	})

	t.Run("empty data not at EOF", func(t *testing.T) {
		advance, token, err := s.splitYamlDoc([]byte{}, false)
		assert.NoError(t, err)
		assert.Equal(t, 0, advance)
		assert.Nil(t, token)
	})

	t.Run("single document at EOF", func(t *testing.T) {
		data := []byte("key: value")
		advance, token, err := s.splitYamlDoc(data, true)
		assert.NoError(t, err)
		assert.Equal(t, len(data), advance)
		assert.Equal(t, data, token)
	})

	t.Run("single document not at EOF requests more data", func(t *testing.T) {
		data := []byte("key: value")
		advance, token, err := s.splitYamlDoc(data, false)
		assert.NoError(t, err)
		assert.Equal(t, 0, advance)
		assert.Nil(t, token)
	})

	t.Run("multi-doc with separator and newline", func(t *testing.T) {
		data := []byte("doc1\n---\ndoc2")
		advance, token, err := s.splitYamlDoc(data, false)
		assert.NoError(t, err)
		assert.True(t, advance > 0)
		assert.Equal(t, []byte("doc1"), token)
	})

	t.Run("separator at end of data at EOF", func(t *testing.T) {
		data := []byte("doc1\n---")
		advance, token, err := s.splitYamlDoc(data, true)
		assert.NoError(t, err)
		assert.Equal(t, len(data), advance)
		assert.Equal(t, []byte("doc1"), token)
	})

	t.Run("separator at end of data not at EOF", func(t *testing.T) {
		data := []byte("doc1\n---")
		advance, token, err := s.splitYamlDoc(data, false)
		assert.NoError(t, err)
		assert.Equal(t, 0, advance)
		assert.Nil(t, token)
	})

	t.Run("separator with no newline after and not at EOF", func(t *testing.T) {
		data := []byte("doc1\n---extra")
		advance, token, err := s.splitYamlDoc(data, false)
		assert.NoError(t, err)
		assert.Equal(t, 0, advance)
		assert.Nil(t, token)
	})
}
