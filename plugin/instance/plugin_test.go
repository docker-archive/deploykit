package instance

import (
	"encoding/json"
	"errors"
	"math/rand"
	"testing"

	compute "google.golang.org/api/compute/v1"

	mock_gcloud "github.com/docker/infrakit.gcp/mock/gcloud"
	"github.com/docker/infrakit.gcp/plugin/instance/gcloud"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func NewMockGCloud(t *testing.T) (*mock_gcloud.MockGCloud, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	return mock_gcloud.NewMockGCloud(ctrl), ctrl
}

func TestProvision(t *testing.T) {
	properties := json.RawMessage(`{
		"NamePrefix":"worker",
		"MachineType":"n1-standard-1",
		"Network":"NETWORK",
		"Tags":["TAG1", "TAG2"],
		"DiskSizeMb":100,
		"DiskImage":"docker-image",
		"DiskType":"ssd",
		"Scopes":["SCOPE1", "SCOPE2"],
		"TargetPool":"POOL",
		"Description":"vm"}`)
	tags := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	rand.Seed(0)
	api, ctrl := NewMockGCloud(t)
	defer ctrl.Finish()
	api.EXPECT().CreateInstance("worker-8717895732742165505", &gcloud.InstanceSettings{
		Description: "vm",
		MachineType: "n1-standard-1",
		Network:     "NETWORK",
		Tags:        []string{"TAG1", "TAG2"},
		DiskSizeMb:  100,
		DiskImage:   "docker-image",
		DiskType:    "ssd",
		Scopes:      []string{"SCOPE1", "SCOPE2"},
		MetaData: gcloud.TagsToMetaData(map[string]string{
			"key1":           "value1",
			"key2":           "value2",
			"startup-script": "echo 'Startup'",
		}),
	}).Return(nil)
	api.EXPECT().AddInstanceToTargetPool("POOL", "worker-8717895732742165505").Return(nil)

	plugin := &plugin{func() (gcloud.GCloud, error) { return api, nil }}
	id, err := plugin.Provision(instance.Spec{
		Tags:       tags,
		Properties: &properties,
		Init:       "echo 'Startup'",
	})

	require.NoError(t, err)
	require.Equal(t, *id, instance.ID("worker-8717895732742165505"))
}

func TestProvisionLogicalID(t *testing.T) {
	properties := json.RawMessage(`{}`)
	tags := map[string]string{}

	rand.Seed(0)
	api, ctrl := NewMockGCloud(t)
	defer ctrl.Finish()
	api.EXPECT().CreateInstance("LOGICAL-ID", &gcloud.InstanceSettings{
		MachineType: "g1-small",
		Network:     "default",
		DiskSizeMb:  10,
		DiskImage:   "docker",
		DiskType:    "pd-standard",
		MetaData:    gcloud.TagsToMetaData(map[string]string{}),
	}).Return(nil)

	logicalID := instance.LogicalID("LOGICAL-ID")

	plugin := &plugin{func() (gcloud.GCloud, error) { return api, nil }}
	id, err := plugin.Provision(instance.Spec{
		LogicalID:  &logicalID,
		Tags:       tags,
		Properties: &properties,
	})

	require.NoError(t, err)
	require.Equal(t, *id, instance.ID("LOGICAL-ID"))
}

func TestProvisionFails(t *testing.T) {
	properties := json.RawMessage(`{}`)
	tags := map[string]string{
		"key1": "value1",
	}

	rand.Seed(0)
	api, _ := NewMockGCloud(t)
	api.EXPECT().CreateInstance("instance-8717895732742165505", &gcloud.InstanceSettings{
		MachineType: "g1-small",
		Network:     "default",
		DiskSizeMb:  10,
		DiskImage:   "docker",
		DiskType:    "pd-standard",
		MetaData:    gcloud.TagsToMetaData(tags),
	}).Return(errors.New("BUG"))

	plugin := &plugin{func() (gcloud.GCloud, error) { return api, nil }}
	id, err := plugin.Provision(instance.Spec{
		Tags:       tags,
		Properties: &properties,
	})

	require.EqualError(t, err, "BUG")
	require.Nil(t, id)
}

func TestProvisionFailsToAddToTargetPool(t *testing.T) {
	properties := json.RawMessage(`{"TargetPool":"POOL"}`)
	tags := map[string]string{}

	rand.Seed(0)
	api, _ := NewMockGCloud(t)
	api.EXPECT().CreateInstance("instance-8717895732742165505", &gcloud.InstanceSettings{
		MachineType: "g1-small",
		Network:     "default",
		DiskSizeMb:  10,
		DiskImage:   "docker",
		DiskType:    "pd-standard",
		MetaData:    gcloud.TagsToMetaData(tags),
	}).Return(nil)
	api.EXPECT().AddInstanceToTargetPool("POOL", "instance-8717895732742165505").Return(errors.New("BUG"))

	plugin := &plugin{func() (gcloud.GCloud, error) { return api, nil }}
	id, err := plugin.Provision(instance.Spec{
		Tags:       tags,
		Properties: &properties,
	})

	require.EqualError(t, err, "BUG")
	require.Nil(t, id)
}

func TestProvisionWithInvalidProperties(t *testing.T) {
	properties := json.RawMessage(``)

	plugin := &plugin{}
	id, err := plugin.Provision(instance.Spec{
		Properties: &properties,
	})

	require.NotNil(t, err)
	require.Nil(t, id)
}

func TestDestroy(t *testing.T) {
	api, _ := NewMockGCloud(t)
	api.EXPECT().DeleteInstance("instance-id").Return(nil)

	plugin := &plugin{func() (gcloud.GCloud, error) { return api, nil }}
	err := plugin.Destroy("instance-id")

	require.NoError(t, err)
}

func TestDestroyFails(t *testing.T) {
	api, _ := NewMockGCloud(t)
	api.EXPECT().DeleteInstance("instance-wrong-id").Return(errors.New("BUG"))

	plugin := &plugin{func() (gcloud.GCloud, error) { return api, nil }}
	err := plugin.Destroy("instance-wrong-id")

	require.EqualError(t, err, "BUG")
}

func TestDescribeEmptyInstances(t *testing.T) {
	api, _ := NewMockGCloud(t)
	api.EXPECT().ListInstances().Return([]*compute.Instance{}, nil)

	plugin := &plugin{func() (gcloud.GCloud, error) { return api, nil }}
	instances, err := plugin.DescribeInstances(nil)

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

	api, _ := NewMockGCloud(t)
	api.EXPECT().ListInstances().Return([]*compute.Instance{
		{
			Name: "instance-valid",
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{
					NewMetadataItems("key1", "value1"),
					NewMetadataItems("key2", "value2"),
				},
			},
		},
		{
			Name: "instance-missing-key",
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{
					NewMetadataItems("key2", "value2"),
				},
			},
		},
		{
			Name: "instance-invalid-value",
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{
					NewMetadataItems("key1", "invalid"),
					NewMetadataItems("key2", "value2"),
				},
			},
		},
	}, nil)

	plugin := &plugin{func() (gcloud.GCloud, error) { return api, nil }}
	instances, err := plugin.DescribeInstances(tags)

	require.NoError(t, err)
	require.Equal(t, len(instances), 1)
}

func TestDescribeInstancesFails(t *testing.T) {
	api, _ := NewMockGCloud(t)
	api.EXPECT().ListInstances().Return(nil, errors.New("BUG"))

	plugin := &plugin{func() (gcloud.GCloud, error) { return api, nil }}
	instances, err := plugin.DescribeInstances(nil)

	require.EqualError(t, err, "BUG")
	require.Nil(t, instances)
}

func TestValidate(t *testing.T) {
	plugin := &plugin{}
	err := plugin.Validate(json.RawMessage(`{"MachineType":"g1-small", "Network":"default"}`))

	require.NoError(t, err)
}

func TestValidateFails(t *testing.T) {
	plugin := &plugin{}
	err := plugin.Validate(json.RawMessage{})

	require.Error(t, err)
}
