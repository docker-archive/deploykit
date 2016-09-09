package scaler

import (
	mock_group "github.com/docker/libmachete/mock/plugin/group"
	"github.com/docker/libmachete/spi/instance"
	"github.com/golang/mock/gomock"
	"testing"
	"time"
)

var (
	a = instance.ID("a")
	b = instance.ID("b")
	c = instance.ID("c")
	d = instance.ID("d")
)

func TestScaleUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scaled := mock_group.NewMockScaled(ctrl)
	scaler := NewAdjustableScaler(scaled, 3, 1*time.Millisecond)

	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.ID{a, b, c}, nil),
		scaled.EXPECT().List().Return([]instance.ID{a, b, c}, nil),
		scaled.EXPECT().List().Return([]instance.ID{a, b}, nil),
		scaled.EXPECT().CreateOne().Return(),
		scaled.EXPECT().List().Do(func() {
			go scaler.Stop()
		}).Return([]instance.ID{a, b, c}, nil),
		// Allow subsequent calls to DescribeInstances() to mitigate ordering flakiness of async Stop() call.
		scaled.EXPECT().List().Return([]instance.ID{a, b, c, d}, nil).AnyTimes(),
	)

	scaler.Run()
}

func TestScaleDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scaled := mock_group.NewMockScaled(ctrl)
	scaler := NewAdjustableScaler(scaled, 2, 1*time.Millisecond)

	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.ID{c, b}, nil),
		scaled.EXPECT().List().Return([]instance.ID{c, a, d, b}, nil),
		scaled.EXPECT().List().Do(func() {
			go scaler.Stop()
		}).Return([]instance.ID{a, b}, nil),
		// Allow subsequent calls to DescribeInstances() to mitigate ordering flakiness of async Stop() call.
		scaled.EXPECT().List().Return([]instance.ID{c, d}, nil).AnyTimes(),
	)

	scaled.EXPECT().Destroy(a).Return(nil)
	scaled.EXPECT().Destroy(b).Return(nil)

	scaler.Run()
}
