package group

import (
	"errors"
	"testing"
	"time"

	mock_group "github.com/docker/infrakit/pkg/mock/plugin/group"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/golang/mock/gomock"
)

var (
	withLabel = instance.Description{
		ID:   instance.ID("withLabel"),
		Tags: map[string]string{},
	}
	withoutLabel = instance.Description{
		ID: instance.ID("withoutLabel"),
		Tags: map[string]string{
			"infrakit.config_sha": "bootstrap",
		},
	}
)

func TestScaleUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")

	scaled := mock_group.NewMockScaled(ctrl)
	scaler := NewScalingGroup(groupID, scaled, 3, 1*time.Millisecond, 0)

	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil),
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil),
		scaled.EXPECT().List().Return([]instance.Description{a, b}, nil),
		scaled.EXPECT().CreateOne(nil).Return(),
		scaled.EXPECT().List().Do(func() {
			go scaler.Stop()
		}).Return([]instance.Description{a, b, c}, nil),
		// Allow subsequent calls to DescribeInstances() to mitigate ordering flakiness of async Stop() call.
		scaled.EXPECT().List().Return([]instance.Description{a, b, c, d}, nil).AnyTimes(),
	)

	scaler.Run()
}

func TestBufferScaleUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")

	scaled := mock_group.NewMockScaled(ctrl)
	scaler := NewScalingGroup(groupID, scaled, 3, 1*time.Millisecond, 1)

	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil),
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil),
		scaled.EXPECT().List().Return([]instance.Description{a, b}, nil),
		scaled.EXPECT().CreateOne(nil).Return(),
		scaled.EXPECT().List().Do(func() {
			go scaler.Stop()
		}).Return([]instance.Description{a, b, c}, nil),
		// Allow subsequent calls to DescribeInstances() to mitigate ordering flakiness of async Stop() call.
		scaled.EXPECT().List().Return([]instance.Description{a, b, c, d}, nil).AnyTimes(),
	)

	scaler.Run()
}

func TestScaleDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")

	scaled := mock_group.NewMockScaled(ctrl)
	scaler := NewScalingGroup(groupID, scaled, 2, 1*time.Millisecond, 0)

	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{c, b}, nil),
		scaled.EXPECT().List().Return([]instance.Description{c, a, d, b}, nil),
		scaled.EXPECT().List().Do(func() {
			go scaler.Stop()
		}).Return([]instance.Description{a, b}, nil),
		// Allow subsequent calls to DescribeInstances() to mitigate ordering flakiness of async Stop() call.
		scaled.EXPECT().List().Return([]instance.Description{c, d}, nil).AnyTimes(),
	)

	scaled.EXPECT().Destroy(a)
	scaled.EXPECT().Destroy(b)

	scaler.Run()
}

func TestBufferScaleDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")

	scaled := mock_group.NewMockScaled(ctrl)
	scaler := NewScalingGroup(groupID, scaled, 2, 1*time.Millisecond, 1)

	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{c, b}, nil),
		scaled.EXPECT().List().Return([]instance.Description{c, a, d, b}, nil),
		scaled.EXPECT().List().Do(func() {
			go scaler.Stop()
		}).Return([]instance.Description{a, b}, nil),
		// Allow subsequent calls to DescribeInstances() to mitigate ordering flakiness of async Stop() call.
		scaled.EXPECT().List().Return([]instance.Description{c, d}, nil).AnyTimes(),
	)

	scaled.EXPECT().Destroy(a)
	scaled.EXPECT().Destroy(b)

	scaler.Run()
}

func TestLabel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")

	scaled := mock_group.NewMockScaled(ctrl)
	scaler := NewScalingGroup(groupID, scaled, 2, 1*time.Millisecond, 1)

	gomock.InOrder(
		scaled.EXPECT().List().Do(func() {
			go scaler.Stop()
		}).Return([]instance.Description{withLabel, withoutLabel}, nil),
		scaled.EXPECT().Label().Return(nil),
		scaled.EXPECT().List().Return([]instance.Description{withLabel, withoutLabel}, nil),
	)

	scaler.Run()
}

func TestFailToLabel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")

	scaled := mock_group.NewMockScaled(ctrl)
	scaler := NewScalingGroup(groupID, scaled, 2, 1*time.Millisecond, 1)

	gomock.InOrder(
		scaled.EXPECT().List().Do(func() {
			go scaler.Stop()
		}).Return([]instance.Description{withLabel, withoutLabel}, nil),
		scaled.EXPECT().Label().Return(errors.New("Unable to label")),
	)

	scaler.Run()
}

func TestFailToList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")

	scaled := mock_group.NewMockScaled(ctrl)
	scaler := NewScalingGroup(groupID, scaled, 2, 1*time.Millisecond, 1)

	gomock.InOrder(
		scaled.EXPECT().List().Return(nil, errors.New("Unable to list")),
		scaled.EXPECT().List().Do(func() {
			go scaler.Stop()
		}).Return([]instance.Description{a, b}, nil),
	)

	scaler.Run()
}
