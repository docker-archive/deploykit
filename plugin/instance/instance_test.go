package instance

import (
	"testing"

	"github.com/codedellemc/gorackhd/client/skus"
	"github.com/codedellemc/gorackhd/models"
	"github.com/codedellemc/infrakit.rackhd/jwt"
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

	auth := jwt.BearerJWTToken("test-jwt-token")

	clientMock := mock.NewMockIface(ctrl)
	clientMock.EXPECT().Login("admin", "admin123").Return(auth, nil)

	nodeMock := mock.NewMockNodeIface(ctrl)
	nodeMock.EXPECT().PostNodesIdentifierWorkflows(gomock.Any(), gomock.Any()).
		Return(nil, nil)

	skuMock := mock.NewMockSkuIface(ctrl)
	skuMock.EXPECT().GetSkus(gomock.Any(), gomock.Any()).
		Return(&skus.GetSkusOK{Payload: _Skus()}, nil)

	skuMock.EXPECT().GetSkusIdentifierNodes(gomock.Any(), gomock.Any()).
		Return(&skus.GetSkusIdentifierNodesOK{Payload: _Nodes()}, nil)

	clientMock.EXPECT().Skus().Times(2).Return(skuMock)
	clientMock.EXPECT().Nodes().Times(1).Return(nodeMock)

	pluginImpl := rackHDInstancePlugin{
		Client:   clientMock,
		Username: "admin",
		Password: "admin123",
	}
	id, err := pluginImpl.Provision(instance.Spec{Properties: inputJSON})

	require.NoError(t, err)
	require.Equal(t, instanceID, string(*id))
}

func _Skus() []*models.Sku {
	skus := []*models.Sku{
		{
			Name:               "vQuanta D51 SKU",
			DiscoveryGraphName: "Graph.vQuanta.Default",
			ID:                 "023cede5-10a3-4480-8193-3979e723f766",
		},
	}
	return skus
}

func _Nodes() []*models.Node {
	names := []string{
		"Enclosure Node QTFCJ05160195",
		"52:54:be:ef:81:6d",
		"52:54:be:ef:81:6e",
	}
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
		{
			AutoDiscover: false,
			ID:           "58b028330b0d3789044b2ba4",
			Name:         &names[2],
			Type:         "compute",
		},
	}
	return nodes
}

var inputJSON = types.AnyString(`{
	"Tags": {"cluster": "infrakit-example2"},
	"Properties": {
		"WorkflowName": "Graph.InstallCentOS",
		"SkuName": "vQuanta D51 SKU"
	}
}`)
