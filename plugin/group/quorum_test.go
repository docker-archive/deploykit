package group

import (
	mock_group "github.com/docker/libmachete/mock/plugin/group"
	"github.com/docker/libmachete/spi/instance"
	"github.com/golang/mock/gomock"
	"testing"
	"time"
)

var (
	a = instance.Description{ID: instance.ID("a"), PrivateIPAddress: "10.0.0.2"}
	b = instance.Description{ID: instance.ID("b"), PrivateIPAddress: "10.0.0.3"}
	c = instance.Description{ID: instance.ID("c"), PrivateIPAddress: "10.0.0.4"}
	d = instance.Description{ID: instance.ID("d"), PrivateIPAddress: "10.0.0.5"}

	quorumAddresses = []string{
		a.PrivateIPAddress,
		b.PrivateIPAddress,
		c.PrivateIPAddress,
	}
)

func TestQuorumOK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scaled := mock_group.NewMockScaled(ctrl)
	quorum := NewQuorum(scaled, quorumAddresses, 1*time.Millisecond)

	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil),
		scaled.EXPECT().List().Do(func() {
			go quorum.Stop()
		}).Return([]instance.Description{a, b, c}, nil),
		// Allow subsequent calls to List() to mitigate ordering flakiness of async Stop() call.
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil).AnyTimes(),
	)

	quorum.Run()
}

func TestRestoreQuorum(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scaled := mock_group.NewMockScaled(ctrl)
	quorum := NewQuorum(scaled, quorumAddresses, 1*time.Millisecond)

	volume := instance.VolumeID("10.0.0.4")
	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil),
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil),
		scaled.EXPECT().List().Return([]instance.Description{a, b}, nil),
		scaled.EXPECT().CreateOne(map[string]string{"IP": string(volume)}, &volume),
		scaled.EXPECT().List().Do(func() {
			go quorum.Stop()
		}).Return([]instance.Description{a, b, c}, nil),
		// Allow subsequent calls to List() to mitigate ordering flakiness of async Stop() call.
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil).AnyTimes(),
	)

	quorum.Run()
}

func TestRemoveUnknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scaled := mock_group.NewMockScaled(ctrl)
	quorum := NewQuorum(scaled, quorumAddresses, 1*time.Millisecond)

	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{a, c, b}, nil),
		scaled.EXPECT().List().Return([]instance.Description{c, a, d, b}, nil),
		scaled.EXPECT().List().Do(func() {
			go quorum.Stop()
		}).Return([]instance.Description{a, b, c}, nil),
		// Allow subsequent calls to List() to mitigate ordering flakiness of async Stop() call.
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil).AnyTimes(),
	)

	scaled.EXPECT().Destroy(d.ID)

	quorum.Run()
}
