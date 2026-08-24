package http

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputBindingRequestJSON(t *testing.T) {
	r := OutputBindingRequest{
		Metadata:  map[string]string{"key": "value"},
		Data:      "test-data",
		Operation: "create",
	}

	b, err := json.Marshal(r)
	require.NoError(t, err)

	var decoded OutputBindingRequest
	err = json.Unmarshal(b, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "create", decoded.Operation)
	assert.Equal(t, "value", decoded.Metadata["key"])
}

func TestBulkGetRequestJSON(t *testing.T) {
	r := BulkGetRequest{
		Metadata:    map[string]string{"partitionKey": "pk1"},
		Keys:        []string{"key1", "key2", "key3"},
		Parallelism: 10,
	}

	b, err := json.Marshal(r)
	require.NoError(t, err)

	var decoded BulkGetRequest
	err = json.Unmarshal(b, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 10, decoded.Parallelism)
	assert.Equal(t, []string{"key1", "key2", "key3"}, decoded.Keys)
	assert.Equal(t, "pk1", decoded.Metadata["partitionKey"])
}
