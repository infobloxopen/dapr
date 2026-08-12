// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package v1

import (
	"testing"

	commonv1pb "github.com/dapr/dapr/pkg/proto/common/v1"
	internalv1pb "github.com/dapr/dapr/pkg/proto/internals/v1"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestInvokeRequest(t *testing.T) {
	req := NewInvokeMethodRequest("test_method")

	assert.Equal(t, internalv1pb.APIVersion_V1, req.r.GetVer())
	assert.Equal(t, "test_method", req.r.Message.GetMethod())
}

func TestFromInvokeRequestMessage(t *testing.T) {
	pb := &commonv1pb.InvokeRequest{Method: "frominvokerequestmessage"}
	req := FromInvokeRequestMessage(pb)

	assert.Equal(t, internalv1pb.APIVersion_V1, req.r.GetVer())
	assert.Equal(t, "frominvokerequestmessage", req.r.Message.GetMethod())
}

func TestInternalInvokeRequest(t *testing.T) {
	t.Run("valid internal invoke request", func(t *testing.T) {
		m := &commonv1pb.InvokeRequest{
			Method:      "invoketest",
			ContentType: "application/json",
			Data:        &anypb.Any{Value: []byte("test")},
		}
		pb := internalv1pb.InternalInvokeRequest{
			Ver:     internalv1pb.APIVersion_V1,
			Message: m,
		}

		ir, err := InternalInvokeRequest(&pb)
		assert.NoError(t, err)
		assert.NotNil(t, ir.r.Message)
		assert.Equal(t, "invoketest", ir.r.Message.GetMethod())
		assert.NotNil(t, ir.r.Message.GetData())
	})

	t.Run("nil message field", func(t *testing.T) {
		pb := internalv1pb.InternalInvokeRequest{
			Ver:     internalv1pb.APIVersion_V1,
			Message: nil,
		}

		_, err := InternalInvokeRequest(&pb)
		assert.Error(t, err)
	})
}

func TestMetadata(t *testing.T) {
	t.Run("gRPC headers", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method")
		md := map[string][]string{
			"test1": {"val1", "val2"},
			"test2": {"val3", "val4"},
		}
		req.WithMetadata(md)
		mdata := req.Metadata()

		assert.Equal(t, "val1", mdata["test1"].GetValues()[0])
		assert.Equal(t, "val2", mdata["test1"].GetValues()[1])
		assert.Equal(t, "val3", mdata["test2"].GetValues()[0])
		assert.Equal(t, "val4", mdata["test2"].GetValues()[1])
	})

	t.Run("HTTP headers", func(t *testing.T) {
		var req = fasthttp.AcquireRequest()
		req.Header.Set("Header1", "Value1")
		req.Header.Set("Header2", "Value2")
		req.Header.Set("Header3", "Value3")

		re := NewInvokeMethodRequest("test_method")
		re.WithFastHTTPHeaders(&req.Header)
		mheader := re.Metadata()

		assert.Equal(t, "Value1", mheader["Header1"].GetValues()[0])
		assert.Equal(t, "Value2", mheader["Header2"].GetValues()[0])
		assert.Equal(t, "Value3", mheader["Header3"].GetValues()[0])
	})
}

func TestData(t *testing.T) {
	t.Run("contenttype is set", func(t *testing.T) {
		resp := NewInvokeMethodRequest("test_method")
		resp.WithRawData([]byte("test"), "application/json")
		contentType, bData := resp.RawData()
		assert.Equal(t, "application/json", contentType)
		assert.Equal(t, []byte("test"), bData)
	})

	t.Run("contenttype is unset", func(t *testing.T) {
		resp := NewInvokeMethodRequest("test_method")
		resp.WithRawData([]byte("test"), "")
		contentType, bData := resp.RawData()
		assert.Equal(t, "application/json", contentType)
		assert.Equal(t, []byte("test"), bData)
	})

	t.Run("typeurl is set but content_type is unset", func(t *testing.T) {
		resp := NewInvokeMethodRequest("test_method")
		resp.r.Message.Data = &anypb.Any{TypeUrl: "fake", Value: []byte("fake")}
		contentType, bData := resp.RawData()
		assert.Equal(t, "", contentType)
		assert.Equal(t, []byte("fake"), bData)
	})
}

func TestHTTPExtension(t *testing.T) {
	req := NewInvokeMethodRequest("test_method")
	req.WithHTTPExtension("POST", "query1=value1&query2=value2")
	assert.Equal(t, commonv1pb.HTTPExtension_POST, req.Message().GetHttpExtension().GetVerb())
	assert.Equal(t, "query1=value1&query2=value2", req.EncodeHTTPQueryString())
}

func TestWithHTTPExtensionVerbs(t *testing.T) {
	tests := []struct {
		name         string
		verb         string
		querystring  string
		expectedVerb commonv1pb.HTTPExtension_Verb
		expectedQS   string
	}{
		{
			name:         "GET verb",
			verb:         "GET",
			querystring:  "key=value",
			expectedVerb: commonv1pb.HTTPExtension_GET,
			expectedQS:   "key=value",
		},
		{
			name:         "PUT verb",
			verb:         "PUT",
			querystring:  "",
			expectedVerb: commonv1pb.HTTPExtension_PUT,
			expectedQS:   "",
		},
		{
			name:         "DELETE verb",
			verb:         "DELETE",
			querystring:  "id=123",
			expectedVerb: commonv1pb.HTTPExtension_DELETE,
			expectedQS:   "id=123",
		},
		{
			name:         "HEAD verb",
			verb:         "HEAD",
			querystring:  "",
			expectedVerb: commonv1pb.HTTPExtension_HEAD,
			expectedQS:   "",
		},
		{
			name:         "OPTIONS verb",
			verb:         "OPTIONS",
			querystring:  "",
			expectedVerb: commonv1pb.HTTPExtension_OPTIONS,
			expectedQS:   "",
		},
		{
			name:         "CONNECT verb",
			verb:         "CONNECT",
			querystring:  "",
			expectedVerb: commonv1pb.HTTPExtension_CONNECT,
			expectedQS:   "",
		},
		{
			name:         "TRACE verb",
			verb:         "TRACE",
			querystring:  "",
			expectedVerb: commonv1pb.HTTPExtension_TRACE,
			expectedQS:   "",
		},
		{
			name:         "lowercase verb is uppercased",
			verb:         "get",
			querystring:  "key=val",
			expectedVerb: commonv1pb.HTTPExtension_GET,
			expectedQS:   "key=val",
		},
		{
			name:         "mixed case verb",
			verb:         "Post",
			querystring:  "",
			expectedVerb: commonv1pb.HTTPExtension_POST,
			expectedQS:   "",
		},
		{
			name:         "invalid verb defaults to POST",
			verb:         "INVALID",
			querystring:  "",
			expectedVerb: commonv1pb.HTTPExtension_POST,
			expectedQS:   "",
		},
		{
			name:         "empty verb defaults to POST",
			verb:         "",
			querystring:  "",
			expectedVerb: commonv1pb.HTTPExtension_POST,
			expectedQS:   "",
		},
		{
			name:         "querystring with special characters",
			verb:         "GET",
			querystring:  "name=hello+world&value=foo%20bar&special=%26%3D",
			expectedVerb: commonv1pb.HTTPExtension_GET,
			expectedQS:   "name=hello+world&value=foo%20bar&special=%26%3D",
		},
		{
			name:         "querystring with unicode",
			verb:         "GET",
			querystring:  "name=%E4%B8%AD%E6%96%87",
			expectedVerb: commonv1pb.HTTPExtension_GET,
			expectedQS:   "name=%E4%B8%AD%E6%96%87",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewInvokeMethodRequest("test_method")
			req.WithHTTPExtension(tt.verb, tt.querystring)

			assert.Equal(t, tt.expectedVerb, req.Message().GetHttpExtension().GetVerb())
			assert.Equal(t, tt.expectedQS, req.EncodeHTTPQueryString())
		})
	}
}

func TestEncodeHTTPQueryStringEdgeCases(t *testing.T) {
	t.Run("nil message returns empty string", func(t *testing.T) {
		req := &InvokeMethodRequest{
			r: &internalv1pb.InternalInvokeRequest{
				Ver:     DefaultAPIVersion,
				Message: nil,
			},
		}
		assert.Equal(t, "", req.EncodeHTTPQueryString())
	})

	t.Run("nil http extension returns empty string", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method")
		// Message exists but no HttpExtension set
		assert.Equal(t, "", req.EncodeHTTPQueryString())
	})

	t.Run("empty querystring", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method")
		req.WithHTTPExtension("GET", "")
		assert.Equal(t, "", req.EncodeHTTPQueryString())
	})

	t.Run("querystring with multiple params", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method")
		req.WithHTTPExtension("GET", "a=1&b=2&c=3")
		assert.Equal(t, "a=1&b=2&c=3", req.EncodeHTTPQueryString())
	})

	t.Run("querystring with special characters", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method")
		qs := "key=value%20with%20spaces&arr[]=1&arr[]=2"
		req.WithHTTPExtension("GET", qs)
		assert.Equal(t, qs, req.EncodeHTTPQueryString())
	})
}

func TestRawDataEdgeCases(t *testing.T) {
	t.Run("nil message returns empty", func(t *testing.T) {
		req := &InvokeMethodRequest{
			r: &internalv1pb.InternalInvokeRequest{
				Ver:     DefaultAPIVersion,
				Message: nil,
			},
		}
		contentType, data := req.RawData()
		assert.Equal(t, "", contentType)
		assert.Nil(t, data)
	})

	t.Run("nil data returns empty", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method")
		req.r.Message.Data = nil
		contentType, data := req.RawData()
		assert.Equal(t, "", contentType)
		assert.Nil(t, data)
	})

	t.Run("data with value but no type url and no content type defaults to json", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method")
		req.r.Message.ContentType = ""
		req.r.Message.Data = &anypb.Any{Value: []byte("test")}
		contentType, data := req.RawData()
		assert.Equal(t, JSONContentType, contentType)
		assert.Equal(t, []byte("test"), data)
	})
}

func TestAPIVersion(t *testing.T) {
	req := NewInvokeMethodRequest("test_method")
	assert.Equal(t, DefaultAPIVersion, req.APIVersion())
}

func TestMessage(t *testing.T) {
	req := NewInvokeMethodRequest("test_method")
	msg := req.Message()
	assert.NotNil(t, msg)
	assert.Equal(t, "test_method", msg.GetMethod())
}

func TestActor(t *testing.T) {
	req := NewInvokeMethodRequest("test_method")
	req.WithActor("testActor", "1")
	assert.Equal(t, "testActor", req.Actor().GetActorType())
	assert.Equal(t, "1", req.Actor().GetActorId())
}

func TestProto(t *testing.T) {
	m := &commonv1pb.InvokeRequest{
		Method:      "invoketest",
		ContentType: "application/json",
		Data:        &anypb.Any{Value: []byte("test")},
	}
	pb := internalv1pb.InternalInvokeRequest{
		Ver:     internalv1pb.APIVersion_V1,
		Message: m,
	}

	ir, err := InternalInvokeRequest(&pb)
	assert.NoError(t, err)
	req2 := ir.Proto()

	assert.Equal(t, "application/json", req2.GetMessage().ContentType)
	assert.Equal(t, []byte("test"), req2.GetMessage().Data.Value)
}
