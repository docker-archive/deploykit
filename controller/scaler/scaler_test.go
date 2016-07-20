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

func TestInvalidRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	requireFailsWithRequest := func(request string) {
		scaler, err := NewFixedScaler(
			1*time.Millisecond,
			mock_instance.NewMockProvisioner(ctrl),
			request,
			uint(3),
		)
		require.Error(t, err)
		require.Nil(t, scaler)
	}

	requireFailsWithRequest("")
	requireFailsWithRequest("{}")
	requireFailsWithRequest(`{"Group": ""`)
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
		provisioner.EXPECT().ListGroup(group).Return([]instance.ID{a, b, c}, nil),
		provisioner.EXPECT().ListGroup(group).Return([]instance.ID{a, b, c}, nil),
		provisioner.EXPECT().ListGroup(group).Return([]instance.ID{a, b}, nil),
		provisioner.EXPECT().Provision(testRequest).Return(&d, nil),
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
	scaler, err := NewFixedScaler(
		1*time.Millisecond,
		provisioner,
		testRequest,
		uint(2),
	)
	require.NoError(t, err)

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
