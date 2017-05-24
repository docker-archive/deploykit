package group

import (
	"testing"
	"time"

	mock_group "github.com/docker/infrakit/pkg/mock/plugin/group"
	mock_instance "github.com/docker/infrakit/pkg/mock/spi/instance"
	"github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	a = instance.Description{ID: instance.ID("a"), LogicalID: logicalID("one")}
	b = instance.Description{ID: instance.ID("b"), LogicalID: logicalID("two")}
	c = instance.Description{ID: instance.ID("c"), LogicalID: logicalID("three")}
	d = instance.Description{ID: instance.ID("d"), LogicalID: logicalID("four")}

	logicalIDs = []instance.LogicalID{
		*a.LogicalID,
		*b.LogicalID,
		*c.LogicalID,
	}
)

func logicalID(value string) *instance.LogicalID {
	id := instance.LogicalID(value)
	return &id
}

func TestQuorumOK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("quorum")
	scaled := mock_group.NewMockScaled(ctrl)
	quorum := NewQuorum(groupID, scaled, logicalIDs, 1*time.Millisecond)

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

	groupID := group.ID("quorum")

	scaled := mock_group.NewMockScaled(ctrl)
	quorum := NewQuorum(groupID, scaled, logicalIDs, 1*time.Millisecond)

	logicalID := *c.LogicalID
	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil),
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil),
		scaled.EXPECT().List().Return([]instance.Description{a, b}, nil),
		scaled.EXPECT().CreateOne(&logicalID),
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

	groupID := group.ID("quorum")

	scaled := mock_group.NewMockScaled(ctrl)
	quorum := NewQuorum(groupID, scaled, logicalIDs, 1*time.Millisecond)

	gomock.InOrder(
		scaled.EXPECT().List().Return([]instance.Description{a, c, b}, nil),
		scaled.EXPECT().List().Return([]instance.Description{c, a, d, b}, nil),
		scaled.EXPECT().List().Do(func() {
			go quorum.Stop()
		}).Return([]instance.Description{a, b, c}, nil),
		// Allow subsequent calls to List() to mitigate ordering flakiness of async Stop() call.
		scaled.EXPECT().List().Return([]instance.Description{a, b, c}, nil).AnyTimes(),
	)

	scaled.EXPECT().Destroy(d)

	quorum.Run()
}

func TestQuorumPlanUpdateNoChanges(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("quorum")
	scaled := mock_group.NewMockScaled(ctrl)
	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	settings := groupSettings{
		instancePlugin: instancePlugin,
		config: types.Spec{
			Allocation: types.AllocationMethod{
				LogicalIDs: []instance.LogicalID{
					*a.LogicalID,
				},
			},
		},
	}
	quorum := NewQuorum(groupID, scaled, logicalIDs, 1*time.Millisecond)
	plan, err := quorum.PlanUpdate(scaled, settings, settings)
	require.NoError(t, err)
	require.IsType(t, &noopUpdate{}, plan)
}

func TestQuorumPlanUpdateLogicalIDChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("quorum")
	scaled := mock_group.NewMockScaled(ctrl)
	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	settingsOld := groupSettings{
		instancePlugin: instancePlugin,
		config: types.Spec{
			Allocation: types.AllocationMethod{
				LogicalIDs: []instance.LogicalID{
					*a.LogicalID,
				},
			},
		},
	}
	settingsNew := groupSettings{
		instancePlugin: instancePlugin,
		config: types.Spec{
			Allocation: types.AllocationMethod{
				LogicalIDs: []instance.LogicalID{
					*b.LogicalID,
				},
			},
		},
	}
	quorum := NewQuorum(groupID, scaled, logicalIDs, 1*time.Millisecond)
	_, err := quorum.PlanUpdate(scaled, settingsOld, settingsNew)
	require.Error(t, err)
}

func TestQuorumPlanUpdateRollingUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("quorum")
	scaled := mock_group.NewMockScaled(ctrl)
	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	instanceOld := types.InstancePlugin{
		Plugin: "name-old",
	}
	instanceNew := types.InstancePlugin{
		Plugin: "name-new",
	}
	allocation := types.AllocationMethod{
		LogicalIDs: []instance.LogicalID{
			*b.LogicalID,
		},
	}
	settingsOld := groupSettings{
		instancePlugin: instancePlugin,
		config: types.Spec{
			Allocation: allocation,
			Instance:   instanceOld,
		},
	}
	settingsNew := groupSettings{
		instancePlugin: instancePlugin,
		config: types.Spec{
			Allocation: allocation,
			Instance:   instanceNew,
		},
	}
	quorum := NewQuorum(groupID, scaled, logicalIDs, 1*time.Millisecond)
	plan, err := quorum.PlanUpdate(scaled, settingsOld, settingsNew)
	require.NoError(t, err)
	require.IsType(t, &rollingupdate{}, plan)
}
