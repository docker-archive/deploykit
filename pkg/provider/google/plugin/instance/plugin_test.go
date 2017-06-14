package instance

import (
	"errors"
	"math/rand"
	"testing"

	mock_gcloud "github.com/docker/infrakit/pkg/provider/google/mock/gcloud"
	"github.com/docker/infrakit/pkg/provider/google/plugin/gcloud"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	compute "google.golang.org/api/compute/v1"
)

func NewMockGCloud(t *testing.T) (*mock_gcloud.MockAPI, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	return mock_gcloud.NewMockAPI(ctrl), ctrl
}

func NewPlugin(api gcloud.API, namespace map[string]string) instance.Plugin {
	return &plugin{API: api, namespace: namespace}
}

func TestProvision(t *testing.T) {
	properties := types.AnyString(`{
		"NamePrefix":"worker",
		"MachineType":"n1-standard-1",
                "PrivateIP" : "10.20.2.100",
		"Network":"NETWORK",
		"Subnetwork":"SUB_EUROPE",
		"Tags":["TAG1", "TAG2"],
		"Disks":[{
			"SizeGb":100,
			"Image":"docker-image",
			"Type":"ssd"
		}],
		"Scopes":["SCOPE1", "SCOPE2"],
		"TargetPools":["POOL1", "POOL2"],
		"Preemptible":true,
		"Description":"vm"}`)
	tags := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	rand.Seed(0)
	api, ctrl := NewMockGCloud(t)
	defer ctrl.Finish()
	api.EXPECT().CreateInstance("worker-ssnk9q", &gcloud.InstanceSettings{
		Description: "vm",
		MachineType: "n1-standard-1",
		PrivateIP:   "10.20.2.100",
		Network:     "NETWORK",
		Subnetwork:  "SUB_EUROPE",
		Tags:        []string{"TAG1", "TAG2"},
		Scopes:      []string{"SCOPE1", "SCOPE2"},
		Preemptible: true,
		Disks: []gcloud.DiskSettings{
			{
				Boot:          true,
				SizeGb:        100,
				Image:         "docker-image",
				Type:          "ssd",
				AutoDelete:    true,
				ReuseExisting: false,
			},
		},
		MetaData: gcloud.TagsToMetaData(map[string]string{
			"key1":                 "value1",
			"key2":                 "value2",
			"startup-script":       "echo 'Startup'",
			"userdata":             "echo 'Startup'",
			"infrakit-gcp-version": "1",
		}),
	}).Return(nil)
	api.EXPECT().AddInstanceToTargetPool("POOL1", "worker-ssnk9q").Return(nil)
	api.EXPECT().AddInstanceToTargetPool("POOL2", "worker-ssnk9q").Return(nil)

	plugin := NewPlugin(api, nil)
	id, err := plugin.Provision(instance.Spec{
		Tags:       tags,
		Properties: properties,
		Init:       "echo 'Startup'",
	})

	require.NoError(t, err)
	require.Equal(t, *id, instance.ID("worker-ssnk9q"))
}

func TestProvisionLogicalID(t *testing.T) {
	properties := types.AnyString(`{
		"Disks":[{
			"AutoDelete":false,
			"ReuseExisting":true
		}]}`)
	tags := map[string]string{}

	api, ctrl := NewMockGCloud(t)
	defer ctrl.Finish()
	api.EXPECT().CreateInstance("LOGICAL-ID", &gcloud.InstanceSettings{
		MachineType: "g1-small",
		Network:     "default",
		Preemptible: false,
		Disks: []gcloud.DiskSettings{
			{
				Boot:          true,
				SizeGb:        10,
				Image:         "docker",
				Type:          "pd-standard",
				AutoDelete:    false,
				ReuseExisting: true,
			},
		},
		MetaData: gcloud.TagsToMetaData(map[string]string{
			"infrakit-logical-id":  "LOGICAL-ID",
			"infrakit-gcp-version": "1",
		}),
	}).Return(nil)

	logicalID := instance.LogicalID("LOGICAL-ID")

	plugin := NewPlugin(api, nil)
	id, err := plugin.Provision(instance.Spec{
		LogicalID:  &logicalID,
		Tags:       tags,
		Properties: properties,
	})

	require.NoError(t, err)
	require.Equal(t, *id, instance.ID("LOGICAL-ID"))
}

func TestProvisionLogicalIDIsIPAddress(t *testing.T) {
	properties := types.AnyString(`{
		"PrivateIP" : "10.20.1.0",
		"Disks":[{
			"AutoDelete":true,
			"ReuseExisting":false
		}]}`) // PrivateIP to be overwritten by LogicalID
	tags := map[string]string{}

	api, ctrl := NewMockGCloud(t)
	defer ctrl.Finish()

	api.EXPECT().CreateInstance(gomock.Any(), &gcloud.InstanceSettings{
		MachineType: "g1-small",
		Network:     "default",
		Disks: []gcloud.DiskSettings{
			{
				Boot:          true,
				SizeGb:        10,
				Image:         "docker",
				Type:          "pd-standard",
				AutoDelete:    true,
				ReuseExisting: false,
			},
		},
		Preemptible: false,
		PrivateIP:   "10.20.1.100",
		MetaData: gcloud.TagsToMetaData(map[string]string{
			"infrakit-logical-id":  "10.20.1.100",
			"infrakit-gcp-version": "1",
		}),
	}).Return(nil)

	logicalID := instance.LogicalID("10.20.1.100")

	plugin := NewPlugin(api, nil)
	id, err := plugin.Provision(instance.Spec{
		LogicalID:  &logicalID,
		Tags:       tags,
		Properties: properties,
	})

	require.NoError(t, err)
	require.Equal(t, "instance-10-20-1-100", string(*id))
}

func TestProvisionFails(t *testing.T) {
	properties := types.AnyString(`{}`)
	tags := map[string]string{
		"key1": "value1",
	}

	rand.Seed(0)
	api, _ := NewMockGCloud(t)
	api.EXPECT().CreateInstance("instance-ssnk9q", &gcloud.InstanceSettings{
		MachineType: "g1-small",
		Network:     "default",
		Disks: []gcloud.DiskSettings{
			{
				Boot:          true,
				SizeGb:        10,
				Image:         "docker",
				Type:          "pd-standard",
				AutoDelete:    true,
				ReuseExisting: false,
			},
		},
		MetaData: gcloud.TagsToMetaData(map[string]string{
			"key1":                 "value1",
			"infrakit-gcp-version": "1",
		}),
	}).Return(errors.New("BUG"))

	plugin := NewPlugin(api, nil)
	id, err := plugin.Provision(instance.Spec{
		Tags:       tags,
		Properties: properties,
	})

	require.EqualError(t, err, "BUG")
	require.Nil(t, id)
}

func TestProvisionFailsToAddToTargetPool(t *testing.T) {
	properties := types.AnyString(`{"TargetPools":["POOL"]}`)
	tags := map[string]string{}

	rand.Seed(0)
	api, _ := NewMockGCloud(t)
	api.EXPECT().CreateInstance("instance-ssnk9q", &gcloud.InstanceSettings{
		MachineType: "g1-small",
		Network:     "default",
		Disks: []gcloud.DiskSettings{
			{
				Boot:          true,
				SizeGb:        10,
				Image:         "docker",
				Type:          "pd-standard",
				AutoDelete:    true,
				ReuseExisting: false,
			},
		},
		MetaData: gcloud.TagsToMetaData(map[string]string{
			"infrakit-gcp-version": "1",
		}),
	}).Return(nil)
	api.EXPECT().AddInstanceToTargetPool("POOL", "instance-ssnk9q").Return(errors.New("BUG"))

	plugin := NewPlugin(api, nil)
	id, err := plugin.Provision(instance.Spec{
		Tags:       tags,
		Properties: properties,
	})

	require.EqualError(t, err, "BUG")
	require.Nil(t, id)
}

func TestProvisionWithInvalidProperties(t *testing.T) {
	properties := types.AnyString("-")

	plugin := &plugin{}
	id, err := plugin.Provision(instance.Spec{
		Properties: properties,
	})

	require.NotNil(t, err)
	require.Nil(t, id)
}

func TestDestroy(t *testing.T) {
	api, _ := NewMockGCloud(t)
	api.EXPECT().DeleteInstance("instance-id").Return(nil)

	plugin := NewPlugin(api, nil)
	err := plugin.Destroy("instance-id", instance.Termination)

	require.NoError(t, err)
}

func TestDestroyFails(t *testing.T) {
	api, _ := NewMockGCloud(t)
	api.EXPECT().DeleteInstance("instance-wrong-id").Return(errors.New("BUG"))

	plugin := NewPlugin(api, nil)
	err := plugin.Destroy("instance-wrong-id", instance.Termination)

	require.EqualError(t, err, "BUG")
}

func TestDescribeEmptyInstances(t *testing.T) {
	api, _ := NewMockGCloud(t)
	api.EXPECT().ListInstances().Return([]*compute.Instance{}, nil)

	plugin := NewPlugin(api, nil)
	instances, err := plugin.DescribeInstances(nil, false)

	require.NoError(t, err)
	require.Empty(t, instances)
}

func NewMetadataItems(key, value string) *compute.MetadataItems {
	return &compute.MetadataItems{
		Key:   key,
		Value: &value,
	}
}

func TestDescribeInstances(t *testing.T) {
	tags := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	namespace := map[string]string{"scope": "test"}

	api, _ := NewMockGCloud(t)
	api.EXPECT().ListInstances().Return([]*compute.Instance{
		{
			Name: "instance-pet-valid",
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{
					NewMetadataItems("key1", "value1"),
					NewMetadataItems("key2", "value2"),
					NewMetadataItems("scope", "test"),
					NewMetadataItems("infrakit-logical-id", "instance-pet-valid"),
					NewMetadataItems("infrakit-gcp-version", "1"),
				},
			},
			Disks: []*compute.AttachedDisk{
				{
					Source: "/projects/p/zones/z/disks/instance-pet-valid",
				},
			},
		},
		{
			Name: "instance-pet-valid-with-old-version",
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{
					NewMetadataItems("key1", "value1"),
					NewMetadataItems("key2", "value2"),
					NewMetadataItems("scope", "test"),
				},
			},
			Disks: []*compute.AttachedDisk{
				{
					Source:     "/projects/p/zones/z/disks/instance-pet-valid-with-old-version",
					AutoDelete: false,
				},
			},
		},
		{
			Name: "instance-cattle-valid",
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{
					NewMetadataItems("key1", "value1"),
					NewMetadataItems("key2", "value2"),
					NewMetadataItems("scope", "test"),
				},
			},
			Disks: []*compute.AttachedDisk{
				{
					AutoDelete: true,
				},
			},
		},
		{
			Name: "instance-missing-key",
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{
					NewMetadataItems("key2", "value2"),
					NewMetadataItems("scope", "test"),
				},
			},
		},
		{
			Name: "instance-invalid-value",
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{
					NewMetadataItems("key1", "invalid"),
					NewMetadataItems("key2", "value2"),
					NewMetadataItems("scope", "test"),
				},
			},
		},
	}, nil)

	plugin := NewPlugin(api, namespace)
	instances, err := plugin.DescribeInstances(tags, false)

	require.NoError(t, err)
	require.Equal(t, len(instances), 3)
	require.Equal(t, "instance-pet-valid", string(instances[0].ID))
	require.Equal(t, "instance-pet-valid", string(*instances[0].LogicalID))
	require.Equal(t, "instance-pet-valid-with-old-version", string(instances[1].ID))
	require.Equal(t, "instance-pet-valid-with-old-version", string(*instances[1].LogicalID))
	require.Equal(t, "instance-cattle-valid", string(instances[2].ID))
	require.Nil(t, instances[2].LogicalID)
}

func TestDescribeInstancesFails(t *testing.T) {
	api, _ := NewMockGCloud(t)
	api.EXPECT().ListInstances().Return(nil, errors.New("BUG"))

	plugin := NewPlugin(api, nil)
	instances, err := plugin.DescribeInstances(nil, false)

	require.EqualError(t, err, "BUG")
	require.Nil(t, instances)
}

func TestValidate(t *testing.T) {
	plugin := &plugin{}
	err := plugin.Validate(types.AnyString(`{"MachineType":"g1-small", "Network":"default"}`))

	require.NoError(t, err)
}

func TestValidateFails(t *testing.T) {
	plugin := &plugin{}
	err := plugin.Validate(types.AnyString("-"))

	require.Error(t, err)
}
