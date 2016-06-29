package scaler

import (
	mock_spi "github.com/docker/libmachete/mock/provisioners/spi"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var (
	group = spi.GroupID("1")
	a     = spi.InstanceID("a")
	b     = spi.InstanceID("b")
	c     = spi.InstanceID("c")
	d     = spi.InstanceID("d")
)

func TestScaleUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_spi.NewMockProvisioner(ctrl)
	scaler := scaler{pollInterval: 1 * time.Millisecond}

	finished := make(chan bool)

	gomock.InOrder(
		provisioner.EXPECT().GetInstances(group).Return([]spi.InstanceID{a, b, c}, nil),
		provisioner.EXPECT().GetInstances(group).Return([]spi.InstanceID{a, b, c}, nil),
		provisioner.EXPECT().GetInstances(group).Return([]spi.InstanceID{a, b}, nil),
		provisioner.EXPECT().AddGroupInstances(group, uint(1)).Return(nil),
		provisioner.EXPECT().GetInstances(group).Do(func(_ spi.GroupID) {
			finished <- true
		}).Return([]spi.InstanceID{a, b, c}, nil),
	)

	require.NoError(t, scaler.MaintainCount(provisioner, group, uint(3)))

	<-finished
	require.NoError(t, scaler.Stop())
	require.Error(t, scaler.Stop())
}

func TestScaleDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_spi.NewMockProvisioner(ctrl)
	scaler := scaler{pollInterval: 1 * time.Millisecond}
	group := spi.GroupID("1")
	finished := make(chan bool)

	a := spi.InstanceID("a")
	b := spi.InstanceID("b")
	c := spi.InstanceID("c")

	gomock.InOrder(
		provisioner.EXPECT().GetInstances(group).Return([]spi.InstanceID{c, b}, nil),
		provisioner.EXPECT().GetInstances(group).Return([]spi.InstanceID{c, a, d, b}, nil),
		provisioner.EXPECT().GetInstances(group).Do(func(_ spi.GroupID) {
			finished <- true
		}).Return([]spi.InstanceID{a, b}, nil),
	)

	eventsC := make(chan spi.DestroyInstanceEvent)
	eventsD := make(chan spi.DestroyInstanceEvent)

	provisioner.EXPECT().DestroyInstance(string(a)).Return(eventsC, nil)
	provisioner.EXPECT().DestroyInstance(string(b)).Return(eventsD, nil)

	go func() {
		close(eventsC)
		close(eventsD)
	}()

	require.NoError(t, scaler.MaintainCount(provisioner, group, uint(2)))

	<-finished
	require.NoError(t, scaler.Stop())
}
