package instance

import (
	"github.com/codedellemc/gorackhd/client/nodes"
	"github.com/codedellemc/gorackhd/client/skus"
	"github.com/codedellemc/gorackhd/client/tags"
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

var nodeMock *mock.MockNodeIface

var _ = Describe("Infrakit.Rackhd.Plugin.Instance", func() {
	var (
		auth       runtime.ClientAuthInfoWriter
		instanceID instance.ID
		mockCtrl   *gomock.Controller
		mockClient *mock.MockIface
		pluginImpl instance.Plugin
	)

	BeforeEach(func() {
		auth = jwt.BearerJWTToken("test-jwt-token")
		instanceID = instance.ID("58b028330b0d3789044b2ba3")
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
				nodeMock = mock.NewMockNodeIface(mockCtrl)
				mockClient.EXPECT().
					Nodes().Times(1).Return(nodeMock)
			})

			Context("before provisioning", func() {
				BeforeEach(func() {
					nodeMock.EXPECT().
						PostNodesIdentifierWorkflows(gomock.Any(), gomock.Any()).
						Times(1).
						Return(nil, nil)

					skuMock := mock.NewMockSkuIface(mockCtrl)
					skuMock.EXPECT().
						GetSkus(gomock.Any(), gomock.Any()).
						Times(1).
						Return(&skus.GetSkusOK{Payload: _Skus()}, nil)

					skuMock.EXPECT().
						GetSkusIdentifierNodes(gomock.Any(), gomock.Any()).
						Times(1).
						Return(&skus.GetSkusIdentifierNodesOK{Payload: _Nodes(false)}, nil)

					tagsMock := mock.NewMockTagIface(mockCtrl)
					tagsMock.EXPECT().
						PatchNodesIdentifierTags(gomock.Any(), gomock.Any()).
						Times(1).
						Return(&tags.PatchNodesIdentifierTagsOK{}, nil)

					mockClient.EXPECT().
						Tags().Times(1).Return(tagsMock)
					mockClient.EXPECT().
						Skus().Times(2).Return(skuMock)
				})

				It("should return an ID without error", func() {
					id, err := pluginImpl.Provision(instance.Spec{Properties: inputJSON})
					Expect(err).To(BeNil())
					Expect(*id).To(Equal(instanceID))
				})
			})

			Context("after provisioning", func() {
				BeforeEach(func() {
					nodeMock.EXPECT().
						GetNodes(gomock.Any(), gomock.Any()).
						Times(1).
						Return(&nodes.GetNodesOK{Payload: _Nodes(true)}, nil)
				})

				It("should read tags back during a describe operation", func() {
					tags := make(map[string]string)
					tags["tier"] = "web"
					descriptions, err := pluginImpl.DescribeInstances(tags)
					Expect(len(descriptions)).To(Equal(1))
					Expect(descriptions[0].ID).To(Equal(instanceID))
					// Expect(descriptions[0].LogicalID).To(Exist())
					Expect(descriptions[0].Tags["infrakit.config_sha"]).To(Equal("006438mMXW8gXeYtUxgf9Zbg94Y"))
					Expect(descriptions[0].Tags["infrakit.group"]).To(Equal("cattle"))
					Expect(descriptions[0].Tags["project"]).To(Equal("infrakit"))
					Expect(descriptions[0].Tags["tier"]).To(Equal("web"))
					Expect(err).To(BeNil())
				})
			})

			Context("when destroying", func() {

				BeforeEach(func() {
					nodeMock.EXPECT().
						PostNodesIdentifierWorkflows(gomock.Any(), gomock.Any()).
						Times(1).
						Return(nil, nil)
				})

				It("should delete without error", func() {
					err := pluginImpl.Destroy(instanceID)
					Expect(err).To(BeNil())
				})
			})

		})

		Context("when writing tags", func() {
			BeforeEach(func() {
				tagsMock := mock.NewMockTagIface(mockCtrl)
				tagsMock.EXPECT().
					PatchNodesIdentifierTags(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&tags.PatchNodesIdentifierTagsOK{}, nil)

				mockClient.EXPECT().
					Tags().Times(1).Return(tagsMock)
			})

			It("should tag a node during a label operation", func() {
				labels := make(map[string]string)
				labels["infrakit.config_sha"] = "006438mMXW8gXeYtUxgf9Zbg94Y"
				labels["infrakit.group"] = "cattle"
				labels["project"] = "infrakit"
				labels["tier"] = "web"
				err := pluginImpl.Label(instanceID, labels)
				Expect(err).To(BeNil())
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

func _Nodes(provisioned bool) []*models.Node {
	names := []string{
		"Enclosure Node QTFCJ05160195",
		"52:54:be:ef:81:6d",
		"52:54:be:ef:81:6e",
	}
	var tags1 []interface{}
	var tags2 []interface{}
	if provisioned {
		tags1 = append(tags1, "infrakit.config_sha=006438mMXW8gXeYtUxgf9Zbg94Y")
		tags1 = append(tags1, "infrakit.group=cattle")
		tags1 = append(tags1, "project=infrakit")
		tags1 = append(tags1, "tier=web")
		tags2 = append(tags1, "infrakit.config_sha=007429nMYW5gWExtUxfG8AcaF2Z")
		tags2 = append(tags1, "infrakit.group=cattle")
		tags2 = append(tags1, "project=infrakit")
		tags2 = append(tags1, "tier=app")
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
			Tags:         tags1,
		},
		{
			AutoDiscover: false,
			ID:           "58b028330b0d3789044b2ba4",
			Name:         &names[2],
			Type:         "compute",
			Tags:         tags2,
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
