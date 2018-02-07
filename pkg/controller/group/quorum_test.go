package group

import (
	"testing"
	"time"

	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	mock_group "github.com/docker/infrakit/pkg/mock/plugin/group"
	mock_instance "github.com/docker/infrakit/pkg/mock/spi/instance"
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

	scaled.EXPECT().Destroy(d, instance.Termination)

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
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				LogicalIDs: []instance.LogicalID{
					*a.LogicalID,
				},
			},
		},
	}
	// The same instance hash
	scaled.EXPECT().List().Return([]instance.Description{{
		ID: instance.ID("inst-1"),
		Tags: map[string]string{
			group.ConfigSHATag: settings.config.InstanceHash(),
		},
	}}, nil).Times(1)
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
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				LogicalIDs: []instance.LogicalID{
					*a.LogicalID,
				},
			},
		},
	}
	settingsNew := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: group.AllocationMethod{
				LogicalIDs: []instance.LogicalID{
					*b.LogicalID,
				},
			},
		},
	}
	quorum := NewQuorum(groupID, scaled, logicalIDs, 1*time.Millisecond)
	_, err := quorum.PlanUpdate(scaled, settingsOld, settingsNew)
	require.Error(t, err)
	require.EqualError(t, err, "Logical ID changes to a quorum is not currently supported")
}

func TestQuorumPlanUpdateRollingUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupID := group.ID("quorum")
	scaled := mock_group.NewMockScaled(ctrl)
	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	instanceOld := group_types.InstancePlugin{
		Plugin: "name-old",
	}
	instanceNew := group_types.InstancePlugin{
		Plugin: "name-new",
	}
	allocation := group.AllocationMethod{
		LogicalIDs: []instance.LogicalID{
			*b.LogicalID,
		},
	}
	settingsOld := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: allocation,
			Instance:   instanceOld,
		},
	}
	settingsNew := groupSettings{
		instancePlugin: instancePlugin,
		config: group_types.Spec{
			Allocation: allocation,
			Instance:   instanceNew,
		},
	}
	// At least 1 has a different instance has
	scaled.EXPECT().List().Return([]instance.Description{
		{
			ID: instance.ID("inst-1"),
			Tags: map[string]string{
				group.ConfigSHATag: settingsNew.config.InstanceHash(),
			},
		}, {
			ID: instance.ID("inst-2"),
			Tags: map[string]string{
				group.ConfigSHATag: settingsOld.config.InstanceHash(),
			},
		},
	}, nil).Times(1)
	quorum := NewQuorum(groupID, scaled, logicalIDs, 1*time.Millisecond)
	plan, err := quorum.PlanUpdate(scaled, settingsOld, settingsNew)
	require.NoError(t, err)
	require.IsType(t, &rollingupdate{}, plan)
	r, _ := plan.(*rollingupdate)
	require.Equal(t, "Performing a rolling update on 1 instances", r.desc)
	require.Equal(t, settingsNew, r.updatingTo)
	require.Equal(t, settingsOld, r.updatingFrom)
	require.Equal(t, scaled, r.scaled)
}
