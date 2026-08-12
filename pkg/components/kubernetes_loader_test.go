package components

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dapr/dapr/pkg/apis/components/v1alpha1"
	subscriptions "github.com/dapr/dapr/pkg/apis/subscriptions/v1alpha1"
	config "github.com/dapr/dapr/pkg/config/modes"
	operatorv1pb "github.com/dapr/dapr/pkg/proto/operator/v1"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type mockOperator struct {
}

func (o *mockOperator) GetConfiguration(ctx context.Context, in *operatorv1pb.GetConfigurationRequest) (*operatorv1pb.GetConfigurationResponse, error) {
	return nil, nil
}

func (o *mockOperator) ListComponents(ctx context.Context, in *emptypb.Empty) (*operatorv1pb.ListComponentResponse, error) {
	component := v1alpha1.Component{}
	component.ObjectMeta.Name = "test"
	component.Spec = v1alpha1.ComponentSpec{
		Type: "testtype",
	}
	b, _ := json.Marshal(&component)

	return &operatorv1pb.ListComponentResponse{
		Components: [][]byte{b},
	}, nil
}

func (o *mockOperator) ListSubscriptions(ctx context.Context, in *emptypb.Empty) (*operatorv1pb.ListSubscriptionsResponse, error) {
	subscription := subscriptions.Subscription{}
	subscription.ObjectMeta.Name = "test"
	subscription.Spec = subscriptions.SubscriptionSpec{
		Topic:      "topic",
		Route:      "route",
		Pubsubname: "pubsub",
	}
	b, _ := json.Marshal(&subscription)

	return &operatorv1pb.ListSubscriptionsResponse{
		Subscriptions: [][]byte{b},
	}, nil
}

func (o *mockOperator) ComponentUpdate(in *emptypb.Empty, srv operatorv1pb.Operator_ComponentUpdateServer) error {
	return nil
}

func getOperatorClient(address string) operatorv1pb.OperatorClient {
	conn, _ := grpc.Dial(address, grpc.WithInsecure())
	return operatorv1pb.NewOperatorClient(conn)
}

func TestNewKubernetesComponents(t *testing.T) {
	t.Run("creates kubernetes loader with config and client", func(t *testing.T) {
		cfg := config.KubernetesConfig{ControlPlaneAddress: "localhost:5000"}
		loader := NewKubernetesComponents(cfg, nil)
		assert.NotNil(t, loader)
		assert.Equal(t, "localhost:5000", loader.config.ControlPlaneAddress)
		assert.Nil(t, loader.client)
	})

	t.Run("creates kubernetes loader with mock client", func(t *testing.T) {
		cfg := config.KubernetesConfig{ControlPlaneAddress: "localhost:6000"}
		client := getOperatorClient("localhost:6000")
		loader := NewKubernetesComponents(cfg, client)
		assert.NotNil(t, loader)
		assert.NotNil(t, loader.client)
	})
}

func TestLoadComponents(t *testing.T) {
	port, _ := freeport.GetFreePort()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	assert.NoError(t, err)

	s := grpc.NewServer()
	operatorv1pb.RegisterOperatorServer(s, &mockOperator{})
	defer s.Stop()

	go func() {
		s.Serve(lis)
	}()

	time.Sleep(time.Second * 1)

	request := &KubernetesComponents{
		client: getOperatorClient(fmt.Sprintf("localhost:%d", port)),
		config: config.KubernetesConfig{
			ControlPlaneAddress: fmt.Sprintf("localhost:%v", port),
		},
	}

	response, err := request.LoadComponents()
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "test", response[0].Name)
	assert.Equal(t, "testtype", response[0].Spec.Type)
}
