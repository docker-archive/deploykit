package instance

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/docker/infrakit/pkg/provider/ibmcloud/client"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func getPlugin(fake *client.FakeSoftlayer, volumeID int) *plugin {
	p := plugin{
		SoftlayerClient: fake,
		VolumeID:        volumeID,
	}
	return &p
}

func TestNewPlugin(t *testing.T) {
	user := "user"
	apiKey := "apiKey"
	volumeID := 123
	authPlugin := NewVolumeAuthPlugin(user, apiKey, volumeID)
	p := authPlugin.(*plugin)
	require.Equal(t, p.VolumeID, 123)
}

func TestValidate(t *testing.T) {
	fake := client.FakeSoftlayer{}
	p := getPlugin(&fake, 1)
	require.NoError(t, p.Validate(nil))
}

func TestLabel(t *testing.T) {
	fake := client.FakeSoftlayer{}
	p := getPlugin(&fake, 1)
	require.NoError(t, p.Label(instance.ID("some-id"), map[string]string{}))
}

func TestProvisionDecodeError(t *testing.T) {
	fake := client.FakeSoftlayer{}
	p := getPlugin(&fake, 1)
	spec := instance.Spec{
		Properties: types.AnyString("no-json"),
	}
	id, err := p.Provision(spec)
	require.Error(t, err)
	require.Nil(t, id)
}

func TestProvisionMissingID(t *testing.T) {
	fake := client.FakeSoftlayer{}
	p := getPlugin(&fake, 1)
	props, err := types.AnyValue(map[string]string{"foo": "bar"})
	require.NoError(t, err)
	spec := instance.Spec{
		Properties: props,
	}
	id, err := p.Provision(spec)
	require.Error(t, err)
	_, expectedErr := strconv.Atoi("")
	require.Equal(t, expectedErr, err)
	require.Nil(t, id)
}

func TestProvisionInvalidID(t *testing.T) {
	fake := client.FakeSoftlayer{}
	p := getPlugin(&fake, 1)
	props, err := types.AnyValue(map[string]string{"id": "NaN"})
	require.NoError(t, err)
	spec := instance.Spec{
		Properties: props,
	}
	id, err := p.Provision(spec)
	require.Error(t, err)
	_, expectedErr := strconv.Atoi("NaN")
	require.Equal(t, expectedErr, err)
	require.Nil(t, id)
}

func TestProvision(t *testing.T) {
	fake := client.FakeSoftlayer{
		AuthorizeToStorageStub: func(storageID, guestID int) error {
			return nil
		},
	}
	p := getPlugin(&fake, 1)
	props, err := types.AnyValue(map[string]string{"id": "123"})
	require.NoError(t, err)
	spec := instance.Spec{
		Properties: props,
	}
	id, err := p.Provision(spec)
	require.NoError(t, err)
	require.Nil(t, id)
	require.Equal(t,
		[]struct {
			StorageID int
			GuestID   int
		}{{1, 123}},
		fake.AuthorizeToStorageArgs)
}

func TestProvisionError(t *testing.T) {
	fake := client.FakeSoftlayer{
		AuthorizeToStorageStub: func(storageID, guestID int) error {
			return fmt.Errorf("custom error")
		},
	}
	p := getPlugin(&fake, 1)
	props, err := types.AnyValue(map[string]string{"id": "123"})
	require.NoError(t, err)
	spec := instance.Spec{
		Properties: props,
	}
	id, err := p.Provision(spec)
	require.Error(t, err)
	require.Equal(t, "custom error", err.Error())
	require.Nil(t, id)
}

func TestDestroy(t *testing.T) {
	fake := client.FakeSoftlayer{
		DeauthorizeFromStorageStub: func(storageID, guestID int) error {
			return nil
		},
	}
	p := getPlugin(&fake, 1)
	err := p.Destroy(instance.ID("123"), instance.RollingUpdate)
	require.NoError(t, err)
	require.Equal(t,
		[]struct {
			StorageID int
			GuestID   int
		}{{1, 123}},
		fake.DeauthorizeFromStorageArgs)
}

func TestDestroyError(t *testing.T) {
	fake := client.FakeSoftlayer{
		DeauthorizeFromStorageStub: func(storageID, guestID int) error {
			return fmt.Errorf("custom error")
		},
	}
	p := getPlugin(&fake, 1)
	err := p.Destroy(instance.ID("123"), instance.RollingUpdate)
	require.Error(t, err)
	require.Equal(t, "custom error", err.Error())
}

func TestDestroyInvalidId(t *testing.T) {
	fake := client.FakeSoftlayer{}
	p := getPlugin(&fake, 1)
	err := p.Destroy(instance.ID("NaN"), instance.RollingUpdate)
	require.Error(t, err)
	_, expectedErr := strconv.Atoi("NaN")
	require.Equal(t, expectedErr, err)
}

func TestDescribe(t *testing.T) {
	fake := client.FakeSoftlayer{
		GetAllowedStorageVirtualGuestsStub: func(storageID int) ([]int, error) {
			return []int{111, 222, 333}, nil
		},
	}
	p := getPlugin(&fake, 1)
	insts, err := p.DescribeInstances(map[string]string{}, true)
	require.NoError(t, err)
	require.Equal(t,
		[]instance.Description{
			{ID: instance.ID("111")},
			{ID: instance.ID("222")},
			{ID: instance.ID("333")},
		},
		insts)
	require.Equal(t,
		[]struct {
			StorageID int
		}{{1}},
		fake.GetAllowedStorageVirtualGuestsArgs)
}

func TestDescribeError(t *testing.T) {
	fake := client.FakeSoftlayer{
		GetAllowedStorageVirtualGuestsStub: func(storageID int) ([]int, error) {
			return []int{}, fmt.Errorf("custom error")
		},
	}
	p := getPlugin(&fake, 1)
	insts, err := p.DescribeInstances(map[string]string{}, true)
	require.Error(t, err)
	require.Equal(t, "custom error", err.Error())
	require.Equal(t, []instance.Description{}, insts)
}
