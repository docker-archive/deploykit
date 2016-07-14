package scaler

import (
	mock_instance "github.com/docker/libmachete/mock/spi/instance"
	"github.com/docker/libmachete/spi/instance"
	"github.com/golang/mock/gomock"
	"testing"
	"time"
)

//go:generate mockgen -package instance -destination ../../mock/spi/instance/instance.go github.com/docker/libmachete/spi/instance Provisioner

var (
	group = instance.GroupID("1")
	a     = instance.ID("a")
	b     = instance.ID("b")
	c     = instance.ID("c")
	d     = instance.ID("d")
)

const (
	provisionRequest = "request body"
)

func TestScaleUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockProvisioner(ctrl)
	scaler := NewFixedScaler(
		1*time.Millisecond,
		provisioner,
		provisionRequest,
		group,
		uint(3),
	)

	gomock.InOrder(
		provisioner.EXPECT().ListGroup(group).Return([]instance.ID{a, b, c}, nil),
		provisioner.EXPECT().ListGroup(group).Return([]instance.ID{a, b, c}, nil),
		provisioner.EXPECT().ListGroup(group).Return([]instance.ID{a, b}, nil),
		provisioner.EXPECT().Provision(provisionRequest).Return(&d, nil),
		provisioner.EXPECT().ListGroup(group).Do(func(_ instance.GroupID) {
			go scaler.Stop()
		}).Return([]instance.ID{a, b, c}, nil),
		// Allow subsequent calls to ListGroup() to mitigate ordering flakiness of async Stop() call.
		provisioner.EXPECT().ListGroup(group).Return([]instance.ID{a, b, c, d}, nil).AnyTimes(),
	)

	scaler.Run()
}

func TestScaleDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	provisioner := mock_instance.NewMockProvisioner(ctrl)
	scaler := NewFixedScaler(
		1*time.Millisecond,
		provisioner,
		"foobar",
		group,
		uint(2),
	)
	group := instance.GroupID("1")

	a := instance.ID("a")
	b := instance.ID("b")
	c := instance.ID("c")

	gomock.InOrder(
		provisioner.EXPECT().ListGroup(group).Return([]instance.ID{c, b}, nil),
		provisioner.EXPECT().ListGroup(group).Return([]instance.ID{c, a, d, b}, nil),
		provisioner.EXPECT().ListGroup(group).Do(func(_ instance.GroupID) {
			go scaler.Stop()
		}).Return([]instance.ID{a, b}, nil),
		// Allow subsequent calls to ListGroup() to mitigate ordering flakiness of async Stop() call.
		provisioner.EXPECT().ListGroup(group).Return([]instance.ID{c, d}, nil).AnyTimes(),
	)

	provisioner.EXPECT().Destroy(a).Return(nil)
	provisioner.EXPECT().Destroy(b).Return(nil)

	scaler.Run()
}
