package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestSetAPIAuthenticationMiddlewareUnary(t *testing.T) {
	testToken := "secret-token"
	testHeader := "dapr-api-token"

	interceptor := setAPIAuthenticationMiddlewareUnary(testToken, testHeader)

	fakeHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	t.Run("missing metadata returns error", func(t *testing.T) {
		ctx := context.Background()
		resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, fakeHandler)
		assert.Nil(t, resp)
		assert.Error(t, err)
	})

	t.Run("missing token returns error", func(t *testing.T) {
		md := metadata.New(map[string]string{})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, fakeHandler)
		assert.Nil(t, resp)
		assert.Error(t, err)
	})

	t.Run("wrong token returns error", func(t *testing.T) {
		md := metadata.New(map[string]string{testHeader: "wrong-token"})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, fakeHandler)
		assert.Nil(t, resp)
		assert.Error(t, err)
	})

	t.Run("valid token passes through to handler", func(t *testing.T) {
		md := metadata.New(map[string]string{testHeader: testToken})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, fakeHandler)
		assert.NoError(t, err)
		assert.Equal(t, "ok", resp)
	})
}
