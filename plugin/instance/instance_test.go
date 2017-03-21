package instance

import (
	"github.com/codedellemc/gorackhd/client/skus"
	"github.com/codedellemc/gorackhd/models"
	"github.com/codedellemc/infrakit.rackhd/jwt"
	"github.com/codedellemc/infrakit.rackhd/mock"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/go-openapi/runtime"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Infrakit.Rackhd.Instance", func() {
	var (
		auth       runtime.ClientAuthInfoWriter
		instanceID string
		mockCtrl   *gomock.Controller
		mockClient *mock.MockIface
		pluginImpl instance.Plugin
	)

	BeforeEach(func() {
		auth = jwt.BearerJWTToken("test-jwt-token")
		instanceID = "58b028330b0d3789044b2ba3"
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mock.NewMockIface(mockCtrl)

		pluginImpl = rackHDInstancePlugin{
			Client:   mockClient,
			Username: "admin",
			Password: "admin123",
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("While logged in", func() {
		BeforeEach(func() {
			mockClient.EXPECT().Login("admin", "admin123").Return(auth, nil)
		})

		Context("when working with nodes", func() {
			BeforeEach(func() {
				nodeMock := mock.NewMockNodeIface(mockCtrl)
				nodeMock.EXPECT().
					PostNodesIdentifierWorkflows(gomock.Any(), gomock.Any()).
					Return(nil, nil)

				mockClient.EXPECT().
					Nodes().Times(1).Return(nodeMock)
			})

			Context("when provisioning", func() {
				BeforeEach(func() {
					skuMock := mock.NewMockSkuIface(mockCtrl)
					skuMock.EXPECT().
						GetSkus(gomock.Any(), gomock.Any()).
						Return(&skus.GetSkusOK{Payload: _Skus()}, nil)

					skuMock.EXPECT().
						GetSkusIdentifierNodes(gomock.Any(), gomock.Any()).
						Return(&skus.GetSkusIdentifierNodesOK{Payload: _Nodes()}, nil)

					mockClient.EXPECT().
						Skus().Times(2).Return(skuMock)
				})

				It("should return an ID without error", func() {
					id, err := pluginImpl.Provision(instance.Spec{Properties: inputJSON})
					Expect(err).To(BeNil())
					Expect(string(*id)).To(Equal(instanceID))
				})
			})

			Context("when destroying", func() {
				It("should delete without error", func() {
					err := pluginImpl.Destroy(instance.ID(instanceID))
					Expect(err).To(BeNil())
				})
			})

		})

	})

})

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
	"Workflow": {
	"name": "Graph.InstallCentOS",
	"options": {
		"install-os": {
		"version": "7.0",
		"repo": "{{file.server}}/Centos/7.0",
		"rootPassword": "root"
		}
	}
	},
	"SKUName": "vQuanta D51 SKU"
}`)
