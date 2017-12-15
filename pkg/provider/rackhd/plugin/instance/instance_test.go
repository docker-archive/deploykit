package instance

import (
	"strings"

	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/go-openapi/runtime"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spiegela/gorackhd/client/nodes"
	"github.com/spiegela/gorackhd/client/skus"
	"github.com/spiegela/gorackhd/jwt"
	"github.com/spiegela/gorackhd/mock"
	"github.com/spiegela/gorackhd/models"
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
						NodesPostWorkflowByID(gomock.Any(), gomock.Any()).
						Times(1).
						Return(nil, nil)

					nodeMock.EXPECT().
						NodesPatchTagByID(gomock.Any(), gomock.Any()).
						Times(1).
						Return(&nodes.NodesPatchTagByIDOK{}, nil)

					skuMock := mock.NewMockSkuIface(mockCtrl)
					skuMock.EXPECT().
						SkusGet(gomock.Any(), gomock.Any()).
						Times(1).
						Return(&skus.SkusGetOK{Payload: _Skus()}, nil)

					skuMock.EXPECT().
						SkusIDGetNodes(gomock.Any(), gomock.Any()).
						Times(1).
						Return(&skus.SkusIDGetNodesOK{Payload: _SkuNodes(false)}, nil)

					mockClient.EXPECT().
						Nodes().Times(1).Return(nodeMock)

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
						NodesGetAll(gomock.Any(), gomock.Any()).
						Times(1).
						Return(&nodes.NodesGetAllOK{Payload: _Nodes(true)}, nil)
				})

				It("should read tags back during a describe operation", func() {
					tags := make(map[string]string)
					tags["tier"] = "web"
					descriptions, err := pluginImpl.DescribeInstances(tags, false)
					Expect(len(descriptions)).To(Equal(1))
					Expect(descriptions[0].ID).To(Equal(instanceID))
					// Expect(descriptions[0].LogicalID).To(Exist())
					Expect(descriptions[0].Tags[group.ConfigSHATag]).To(Equal("006438mMXW8gXeYtUxgf9Zbg94Y"))
					Expect(descriptions[0].Tags[group.GroupTag]).To(Equal("cattle"))
					Expect(descriptions[0].Tags["project"]).To(Equal("infrakit"))
					Expect(descriptions[0].Tags["tier"]).To(Equal("web"))
					Expect(err).To(BeNil())
				})
			})

			Context("when destroying", func() {

				BeforeEach(func() {
					nodeMock.EXPECT().
						NodesPostWorkflowByID(gomock.Any(), gomock.Any()).
						Times(1).
						Return(nil, nil)
				})

				It("should delete without error", func() {
					err := pluginImpl.Destroy(instanceID, instance.Termination)
					Expect(err).To(BeNil())
				})
			})

		})

		Context("when writing tags", func() {
			BeforeEach(func() {
				nodesMock := mock.NewMockNodeIface(mockCtrl)
				nodesMock.EXPECT().
					NodesPatchTagByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&nodes.NodesPatchTagByIDOK{}, nil)

				mockClient.EXPECT().
					Nodes().Times(1).Return(nodesMock)
			})

			It("should tag a node during a label operation", func() {
				labels := make(map[string]string)
				labels[group.ConfigSHATag] = "006438mMXW8gXeYtUxgf9Zbg94Y"
				labels[group.GroupTag] = "cattle"
				labels["project"] = "infrakit"
				labels["tier"] = "web"
				err := pluginImpl.Label(instanceID, labels)
				Expect(err).To(BeNil())
			})
		})
	})

	It("should return no error when validating valid specs", func() {
		err := pluginImpl.Validate(inputJSON)
		Expect(err).To(BeNil())
	})

	It("should error when no workflow is provided", func() {
		var errorJSON = types.AnyString(`{
			"SKUName": "vQuanta D51 SKU"
		}`)
		err := pluginImpl.Validate(errorJSON)
		Expect(strings.HasPrefix(err.Error(), "no-workflow")).To(BeTrue())
	})

	It("should error when no SKU name is provided", func() {
		var errorJSON = types.AnyString(`{
			"Workflow": {
				"name": "Graph.InstallCentOS"
			}
		}`)
		err := pluginImpl.Validate(errorJSON)
		Expect(strings.HasPrefix(err.Error(), "no-sku-name")).To(BeTrue())
	})
})

func _Skus() []*models.Skus20Sku {
	skus := []*models.Skus20Sku{
		{
			Name:               "vQuanta D51 SKU",
			DiscoveryGraphName: "Graph.vQuanta.Default",
			ID:                 "023cede5-10a3-4480-8193-3979e723f766",
		},
	}
	return skus
}

func _Nodes(provisioned bool) []*models.Node20Node {
	names := []string{
		"Enclosure Node QTFCJ05160195",
		"52:54:be:ef:81:6d",
		"52:54:be:ef:81:6e",
	}
	var tags1 []string
	var tags2 []string
	if provisioned {
		tags1 = append(tags1, group.ConfigSHATag+"=006438mMXW8gXeYtUxgf9Zbg94Y")
		tags1 = append(tags1, group.GroupTag+"=cattle")
		tags1 = append(tags1, "project=infrakit")
		tags1 = append(tags1, "tier=web")
		tags2 = append(tags1, group.ConfigSHATag+"=007429nMYW5gWExtUxfG8AcaF2Z")
		tags2 = append(tags1, group.GroupTag+"=cattle")
		tags2 = append(tags1, "project=infrakit")
		tags2 = append(tags1, "tier=app")
	}
	nodes := []*models.Node20Node{
		{
			AutoDiscover: "false",
			ID:           "58b02931ca8c52d204e60388",
			Name:         names[0],
			Type:         "enclosure",
		},
		{
			AutoDiscover: "false",
			ID:           "58b028330b0d3789044b2ba3",
			Name:         names[1],
			Type:         "compute",
			Tags:         strings.Join(tags1, ","),
		},
		{
			AutoDiscover: "false",
			ID:           "58b028330b0d3789044b2ba4",
			Name:         names[2],
			Type:         "compute",
			Tags:         strings.Join(tags2, ","),
		},
	}
	return nodes
}

func _SkuNodes(provisioned bool) []*models.Node20SkuNode {
	names := []string{
		"Enclosure Node QTFCJ05160195",
		"52:54:be:ef:81:6d",
		"52:54:be:ef:81:6e",
	}
	var tags1 []string
	var tags2 []string
	if provisioned {
		tags1 = append(tags1, group.ConfigSHATag+"=006438mMXW8gXeYtUxgf9Zbg94Y")
		tags1 = append(tags1, group.GroupTag+"=cattle")
		tags1 = append(tags1, "project=infrakit")
		tags1 = append(tags1, "tier=web")
		tags2 = append(tags1, group.ConfigSHATag+"=007429nMYW5gWExtUxfG8AcaF2Z")
		tags2 = append(tags1, group.GroupTag+"=cattle")
		tags2 = append(tags1, "project=infrakit")
		tags2 = append(tags1, "tier=app")
	}
	nodes := []*models.Node20SkuNode{
		{
			AutoDiscover: false,
			ID:           "58b02931ca8c52d204e60388",
			Name:         names[0],
			Type:         "enclosure",
		},
		{
			AutoDiscover: false,
			ID:           "58b028330b0d3789044b2ba3",
			Name:         names[1],
			Type:         "compute",
			Tags:         tags1,
		},
		{
			AutoDiscover: false,
			ID:           "58b028330b0d3789044b2ba4",
			Name:         names[2],
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
