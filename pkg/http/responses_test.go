// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

func TestHeaders(t *testing.T) {
	t.Run("Respond with JSON", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{Request: fasthttp.Request{}}
		respondWithJSON(ctx, 200, nil)

		assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))
	})

	t.Run("Respond with JSON overrides custom content-type", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{Request: fasthttp.Request{}}
		ctx.Response.Header.SetContentType("custom")
		respondWithJSON(ctx, 200, nil)

		assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))
	})

	t.Run("Respond with ETag JSON", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{Request: fasthttp.Request{}}
		etagValue := "etagValue"
		respondWithETaggedJSON(ctx, 200, nil, etagValue)

		assert.Equal(t, etagValue, string(ctx.Response.Header.Peek(etagHeader)))
	})

	t.Run("Respond with custom content type", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{Request: fasthttp.Request{}}
		customContentType := "custom"
		ctx.Response.Header.SetContentType(customContentType)
		respond(ctx, 200, nil)

		assert.Equal(t, customContentType, string(ctx.Response.Header.ContentType()))
	})

	t.Run("Respond with default content type", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{Request: fasthttp.Request{}}
		respond(ctx, 200, nil)

		assert.Equal(t, "text/plain; charset=utf-8", string(ctx.Response.Header.ContentType()))
	})
}

func TestRespondWithError(t *testing.T) {
	ctx := &fasthttp.RequestCtx{Request: fasthttp.Request{}}
	respondWithError(ctx, fasthttp.StatusBadRequest, ErrorResponse{
		ErrorCode: "ERR_BAD_REQUEST",
		Message:   "bad request",
	})

	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))
	assert.Contains(t, string(ctx.Response.Body()), "ERR_BAD_REQUEST")
	assert.Contains(t, string(ctx.Response.Body()), "bad request")
}

func TestRespondEmpty(t *testing.T) {
	ctx := &fasthttp.RequestCtx{Request: fasthttp.Request{}}
	respondEmpty(ctx)

	assert.Equal(t, fasthttp.StatusNoContent, ctx.Response.StatusCode())
	assert.Empty(t, ctx.Response.Body())
}

func TestRespondSetsBodyAndStatusCode(t *testing.T) {
	ctx := &fasthttp.RequestCtx{Request: fasthttp.Request{}}
	body := []byte(`{"key":"value"}`)
	respond(ctx, fasthttp.StatusCreated, body)

	assert.Equal(t, fasthttp.StatusCreated, ctx.Response.StatusCode())
	assert.Equal(t, body, ctx.Response.Body())
}

func TestRespondWithETaggedJSONSetsAllFields(t *testing.T) {
	ctx := &fasthttp.RequestCtx{Request: fasthttp.Request{}}
	body := []byte(`{"state":"value"}`)
	respondWithETaggedJSON(ctx, fasthttp.StatusOK, body, "etag-123")

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, body, ctx.Response.Body())
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))
	assert.Equal(t, "etag-123", string(ctx.Response.Header.Peek(etagHeader)))
}
