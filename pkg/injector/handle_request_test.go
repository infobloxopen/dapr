// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package injector

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func TestHandleRequestEmptyBody(t *testing.T) {
	i := &injector{}

	t.Run("nil body returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mutate", nil)
		rec := httptest.NewRecorder()

		i.handleRequest(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "empty body")
	})

	t.Run("empty string body returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(""))
		rec := httptest.NewRecorder()

		i.handleRequest(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "empty body")
	})
}

func TestHandleRequestWrongContentType(t *testing.T) {
	i := &injector{}

	t.Run("text/plain content type returns 415", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader("some body"))
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()

		i.handleRequest(rec, req)

		assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid Content-Type")
	})

	t.Run("missing content type returns 415", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader("some body"))
		// Do not set Content-Type header; default is empty string
		rec := httptest.NewRecorder()

		i.handleRequest(rec, req)

		assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid Content-Type")
	})

	t.Run("application/xml content type returns 415", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader("<xml/>"))
		req.Header.Set("Content-Type", "application/xml")
		rec := httptest.NewRecorder()

		i.handleRequest(rec, req)

		assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid Content-Type")
	})
}

func TestHandleRequestMalformedBody(t *testing.T) {
	i := &injector{
		deserializer: serializer.NewCodecFactory(
			runtime.NewScheme(),
		).UniversalDeserializer(),
	}

	t.Run("invalid JSON body returns error in admission response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader("not valid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		i.handleRequest(rec, req)

		// The handler does not return an HTTP error for decode failures;
		// it wraps the error inside an AdmissionResponse and writes it as
		// JSON with a 200 status code.
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.NotEmpty(t, rec.Body.String())
	})

	t.Run("random bytes body returns error in admission response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader("{invalid"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		i.handleRequest(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	})
}
