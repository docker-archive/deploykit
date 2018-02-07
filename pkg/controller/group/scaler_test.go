package group

import (
	"errors"
	"testing"
	"time"

	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	mock_group "github.com/docker/infrakit/pkg/mock/plugin/group"
	mock_instance "github.com/docker/infrakit/pkg/mock/spi/instance"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	testutil "github.com/docker/infrakit/pkg/testing"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	withLabel = instance.Description{
		ID:   instance.ID("withLabel"),
		Tags: map[string]string{},
	}
	withoutLabel = instance.Description{
		ID: instance.ID("withoutLabel"),
		Tags: map[string]string{
			group.ConfigSHATag: "bootstrap",
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

	if testutil.SkipTests("flaky") {
		t.SkipNow()
	}

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

	scaled.EXPECT().Destroy(a, instance.Termination)
	scaled.EXPECT().Destroy(b, instance.Termination)

	scaler.Run()
}

func TestBufferScaleDown(t *testing.T) {

	if testutil.SkipTests("flaky") {
		t.SkipNow()
	}

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

	scaled.EXPECT().Destroy(a, instance.Termination)
	scaled.EXPECT().Destroy(b, instance.Termination)

	scaler.Run()
}

func TestLabel(t *testing.T) {

	if testutil.SkipTests("flaky") {
		t.SkipNow()
	}

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

func TestScalerPlanUpdateNoChanges(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")
	scaled := mock_group.NewMockScaled(ctrl)
	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	settings := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				Size: 1,
			},
		},
	}
	scaler := NewScalingGroup(groupID, scaled, 1, 1*time.Millisecond, 1)
	existingInst := instance.Description{
		ID: instance.ID("id1"),
		Tags: map[string]string{
			group.ConfigSHATag: settings.config.InstanceHash(),
		},
	}
	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{existingInst}, nil),
	)
	plan, err := scaler.PlanUpdate(scaled, settings, settings)
	require.NoError(t, err)
	require.IsType(t, &noopUpdate{}, plan)
}

func TestScalerPlanUpdateRollingUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")
	scaled := mock_group.NewMockScaled(ctrl)
	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	settings := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				Size: 1,
			},
		},
	}
	scaler := NewScalingGroup(groupID, scaled, 1, 1*time.Millisecond, 1)
	existingInst := instance.Description{
		ID: instance.ID("id1"),
		Tags: map[string]string{
			group.ConfigSHATag: "different-hash",
		},
	}
	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{existingInst}, nil),
	)
	plan, err := scaler.PlanUpdate(scaled, settings, settings)
	require.NoError(t, err)
	require.IsType(t, scalerUpdatePlan{}, plan)
	require.Equal(t,
		"Performing a rolling update on 1 instances",
		plan.(scalerUpdatePlan).desc,
	)
}

func TestScalerPlanUpdateScaleDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")
	scaled := mock_group.NewMockScaled(ctrl)
	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	settingsOld := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				Size: 2,
			},
		},
	}
	settingsNew := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				Size: 1,
			},
		},
	}
	scaler := NewScalingGroup(groupID, scaled, 1, 1*time.Millisecond, 1)
	existingInst := instance.Description{
		ID: instance.ID("id1"),
		Tags: map[string]string{
			group.ConfigSHATag: settingsOld.config.InstanceHash(),
		},
	}
	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{existingInst, existingInst}, nil),
	)
	plan, err := scaler.PlanUpdate(scaled, settingsOld, settingsNew)
	require.NoError(t, err)
	require.IsType(t, scalerUpdatePlan{}, plan)
	require.Equal(t,
		"Terminating 1 instances to reduce the group size to 1",
		plan.(scalerUpdatePlan).desc,
	)
}

func TestScalerPlanUpdateScaleDownRollingUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")
	scaled := mock_group.NewMockScaled(ctrl)
	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	settingsOld := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				Size: 2,
			},
		},
	}
	settingsNew := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				Size: 1,
			},
		},
	}
	scaler := NewScalingGroup(groupID, scaled, 1, 1*time.Millisecond, 1)
	existingInst := instance.Description{
		ID: instance.ID("id1"),
		Tags: map[string]string{
			group.ConfigSHATag: "different-hash",
		},
	}
	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{existingInst, existingInst}, nil),
	)
	plan, err := scaler.PlanUpdate(scaled, settingsOld, settingsNew)
	require.NoError(t, err)
	require.IsType(t, scalerUpdatePlan{}, plan)
	require.Equal(t,
		"Terminating 1 instances to reduce the group size to 1, then performing a rolling update on 1 instances",
		plan.(scalerUpdatePlan).desc,
	)
}

func TestScalerPlanUpdateScaleUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")
	scaled := mock_group.NewMockScaled(ctrl)
	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	settingsOld := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				Size: 1,
			},
		},
	}
	settingsNew := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				Size: 2,
			},
		},
	}
	scaler := NewScalingGroup(groupID, scaled, 1, 1*time.Millisecond, 1)
	existingInst := instance.Description{
		ID: instance.ID("id1"),
		Tags: map[string]string{
			group.ConfigSHATag: settingsOld.config.InstanceHash(),
		},
	}
	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{existingInst}, nil),
	)
	plan, err := scaler.PlanUpdate(scaled, settingsOld, settingsNew)
	require.NoError(t, err)
	require.IsType(t, scalerUpdatePlan{}, plan)
	require.Equal(t,
		"Adding 1 instances to increase the group size to 2",
		plan.(scalerUpdatePlan).desc,
	)
}

func TestScalerPlanUpdateScaleUpRollingUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("scaler")
	scaled := mock_group.NewMockScaled(ctrl)
	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	settingsOld := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				Size: 1,
			},
		},
	}
	settingsNew := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				Size: 2,
			},
		},
	}
	scaler := NewScalingGroup(groupID, scaled, 1, 1*time.Millisecond, 1)
	existingInst := instance.Description{
		ID: instance.ID("id1"),
		Tags: map[string]string{
			group.ConfigSHATag: "different-hash",
		},
	}
	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{existingInst}, nil),
	)
	plan, err := scaler.PlanUpdate(scaled, settingsOld, settingsNew)
	require.NoError(t, err)
	require.IsType(t, scalerUpdatePlan{}, plan)
	require.Equal(t,
		"Performing a rolling update on 1 instances, then adding 1 instances to increase the group size to 2",
		plan.(scalerUpdatePlan).desc,
	)
}
