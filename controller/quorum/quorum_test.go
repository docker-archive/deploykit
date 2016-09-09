package quorum

import (
	"encoding/json"
	mock_instance "github.com/docker/libmachete/mock/spi/instance"
	"github.com/docker/libmachete/spi/instance"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const (
	requestTemplate = `{"Group": "test-group", "IP": "{{.IP}}"}`
)

var (
	tags = map[string]string{"group": "test-group"}
	a    = instance.Description{ID: instance.ID("a"), PrivateIPAddress: "10.0.0.2"}
	b    = instance.Description{ID: instance.ID("b"), PrivateIPAddress: "10.0.0.3"}
	c    = instance.Description{ID: instance.ID("c"), PrivateIPAddress: "10.0.0.4"}
	d    = instance.Description{ID: instance.ID("d"), PrivateIPAddress: "10.0.0.5"}

	quorumAddresses = []string{
		a.PrivateIPAddress,
		b.PrivateIPAddress,
		c.PrivateIPAddress,
	}
)

func TestQuorumOK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockPlugin(ctrl)
	quorum, err := NewQuorum(
		1*time.Millisecond,
		provisioner,
		requestTemplate,
		quorumAddresses,
	)
	require.NoError(t, err)

	gomock.InOrder(
		provisioner.EXPECT().DescribeInstances(tags).Return([]instance.Description{a, b, c}, nil),
		provisioner.EXPECT().DescribeInstances(tags).Do(func(_ map[string]string) {
			go quorum.Stop()
		}).Return([]instance.Description{a, b, c}, nil),
		// Allow subsequent calls to DescribeInstances() to mitigate ordering flakiness of async Stop() call.
		provisioner.EXPECT().DescribeInstances(tags).Return([]instance.Description{a, b, c}, nil).AnyTimes(),
	)

	quorum.Run()
}

func TestRestoreQuorum(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockPlugin(ctrl)
	quorum, err := NewQuorum(
		1*time.Millisecond,
		provisioner,
		requestTemplate,
		quorumAddresses,
	)
	require.NoError(t, err)

	volume := instance.VolumeID("10.0.0.4")
	gomock.InOrder(
		provisioner.EXPECT().DescribeInstances(tags).Return([]instance.Description{a, b, c}, nil),
		provisioner.EXPECT().DescribeInstances(tags).Return([]instance.Description{a, b, c}, nil),
		provisioner.EXPECT().DescribeInstances(tags).Return([]instance.Description{a, b}, nil),
		provisioner.EXPECT().Provision(
			json.RawMessage(`{"Group": "test-group", "IP": "10.0.0.4"}`), &volume, tags).Return(&c.ID, nil),
		provisioner.EXPECT().DescribeInstances(tags).Do(func(_ map[string]string) {
			go quorum.Stop()
		}).Return([]instance.Description{a, b, c}, nil),
		// Allow subsequent calls to DescribeInstances() to mitigate ordering flakiness of async Stop() call.
		provisioner.EXPECT().DescribeInstances(tags).Return([]instance.Description{a, b, c}, nil).AnyTimes(),
	)

	quorum.Run()
}

func TestRemoveUnknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockPlugin(ctrl)
	quorum, err := NewQuorum(
		1*time.Millisecond,
		provisioner,
		requestTemplate,
		quorumAddresses,
	)
	require.NoError(t, err)

	gomock.InOrder(
		provisioner.EXPECT().DescribeInstances(tags).Return([]instance.Description{a, c, b}, nil),
		provisioner.EXPECT().DescribeInstances(tags).Return([]instance.Description{c, a, d, b}, nil),
		provisioner.EXPECT().DescribeInstances(tags).Do(func(_ map[string]string) {
			go quorum.Stop()
		}).Return([]instance.Description{a, b, c}, nil),
		// Allow subsequent calls to DescribeInstances() to mitigate ordering flakiness of async Stop() call.
		provisioner.EXPECT().DescribeInstances(tags).Return([]instance.Description{a, b, c}, nil).AnyTimes(),
	)

	provisioner.EXPECT().Destroy(d.ID).Return(nil)

	quorum.Run()
}
