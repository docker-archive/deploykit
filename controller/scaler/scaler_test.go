package scaler

import (
	mock_instance "github.com/docker/libmachete/mock/spi/instance"
	"github.com/docker/libmachete/spi/instance"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const (
	testRequest = `{"Group": "test-group"}`
)

var (
	group = instance.GroupID("test-group")
	a     = instance.ID("a")
	b     = instance.ID("b")
	c     = instance.ID("c")
	d     = instance.ID("d")
)

func desc(ids []instance.ID) []instance.Description {
	descriptions := []instance.Description{}
	for _, id := range ids {
		descriptions = append(descriptions, instance.Description{ID: id})
	}
	return descriptions
}

func TestScaleUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockProvisioner(ctrl)
	scaler, err := NewFixedScaler(
		1*time.Millisecond,
		provisioner,
		testRequest,
		uint(3),
	)
	require.NoError(t, err)

	gomock.InOrder(
		provisioner.EXPECT().DescribeInstances(group).Return(desc([]instance.ID{a, b, c}), nil),
		provisioner.EXPECT().DescribeInstances(group).Return(desc([]instance.ID{a, b, c}), nil),
		provisioner.EXPECT().DescribeInstances(group).Return(desc([]instance.ID{a, b}), nil),
		provisioner.EXPECT().Provision(testRequest).Return(&d, nil),
		provisioner.EXPECT().DescribeInstances(group).Do(func(_ instance.GroupID) {
			go scaler.Stop()
		}).Return(desc([]instance.ID{a, b, c}), nil),
		// Allow subsequent calls to DescribeInstances() to mitigate ordering flakiness of async Stop() call.
		provisioner.EXPECT().DescribeInstances(group).Return(desc([]instance.ID{a, b, c, d}), nil).AnyTimes(),
	)

	scaler.Run()
}

func TestScaleDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockProvisioner(ctrl)
	scaler, err := NewFixedScaler(
		1*time.Millisecond,
		provisioner,
		testRequest,
		uint(2),
	)
	require.NoError(t, err)

	gomock.InOrder(
		provisioner.EXPECT().DescribeInstances(group).Return(desc([]instance.ID{c, b}), nil),
		provisioner.EXPECT().DescribeInstances(group).Return(desc([]instance.ID{c, a, d, b}), nil),
		provisioner.EXPECT().DescribeInstances(group).Do(func(_ instance.GroupID) {
			go scaler.Stop()
		}).Return(desc([]instance.ID{a, b}), nil),
		// Allow subsequent calls to DescribeInstances() to mitigate ordering flakiness of async Stop() call.
		provisioner.EXPECT().DescribeInstances(group).Return(desc([]instance.ID{c, d}), nil).AnyTimes(),
	)

	provisioner.EXPECT().Destroy(a).Return(nil)
	provisioner.EXPECT().Destroy(b).Return(nil)

	scaler.Run()
}
