package instance

import (
	"testing"

	"github.com/codedellemc/gorackhd/client/nodes"
	"github.com/codedellemc/gorackhd/models"
	"github.com/codedellemc/infrakit.rackhd/mock"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInstanceLifecycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	instanceID := "58b028330b0d3789044b2ba3"

	nodeMock := mock.NewMockNodeIface(ctrl)
	nodeMock.EXPECT().GetNodes(nil, nil).
		Return(&nodes.GetNodesOK{Payload: _Nodes()}, nil)

	clientMock := mock.NewMockIface(ctrl)
	clientMock.EXPECT().Nodes().Return(nodeMock)

	pluginImpl := rackHDInstancePlugin{client: clientMock}
	inputJSON := types.AnyString("{}")
	id, err := pluginImpl.Provision(instance.Spec{Properties: inputJSON})

	require.NoError(t, err)
	require.Equal(t, instanceID, string(*id))
}

func _Nodes() []*models.Node {
	names := []string{"Enclosure Node QTFCJ05160195", "52:54:be:ef:81:6d"}
	nodes := []*models.Node{
		{
			AutoDiscover: false,
			ID:           "58b02931ca8c52d204e60388",
			Name:         &names[0],
			Type:         "enclosure",
		},
		{
			AutoDiscover: false,
			ID:           "58b028330b0d3789044b2ba3",
			Name:         &names[1],
			Type:         "compute",
		},
	}
	return nodes
}
