package group

import (
	"fmt"
	"testing"

	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	mock_flavor "github.com/docker/infrakit/pkg/mock/spi/flavor"
	mock_instance "github.com/docker/infrakit/pkg/mock/spi/instance"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestDestroyInstanceDestroyError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	flavorPlugin := mock_flavor.NewMockPlugin(ctrl)
	flavorProps := types.AnyString("flavorProps")
	scaled := &scaledGroup{
		settings: groupSettings{
			instancePlugin: instancePlugin,
			flavorPlugin:   flavorPlugin,
			config: group_types.Spec{
				Flavor: group_types.FlavorPlugin{
					Properties: flavorProps,
				},
			},
		},
	}

	instID := instance.ID("id")
	inst := instance.Description{ID: instID}
	gomock.InOrder(
		flavorPlugin.EXPECT().Drain(flavorProps, inst).Return(nil),
		instancePlugin.EXPECT().Destroy(instID, instance.RollingUpdate).Return(fmt.Errorf("instance-error")),
	)

	err := scaled.Destroy(inst, instance.RollingUpdate)
	require.Error(t, err)
	require.EqualError(t, err, "instance-error")
}

func TestDestroyRollingUpdateNoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	flavorPlugin := mock_flavor.NewMockPlugin(ctrl)
	flavorProps := types.AnyString("flavorProps")
	scaled := &scaledGroup{
		settings: groupSettings{
			instancePlugin: instancePlugin,
			flavorPlugin:   flavorPlugin,
			config: group_types.Spec{
				Flavor: group_types.FlavorPlugin{
					Properties: flavorProps,
				},
			},
		},
	}

	instID := instance.ID("id")
	inst := instance.Description{ID: instID}
	gomock.InOrder(
		flavorPlugin.EXPECT().Drain(flavorProps, inst).Return(nil),
		instancePlugin.EXPECT().Destroy(instID, instance.RollingUpdate).Return(nil),
	)

	err := scaled.Destroy(inst, instance.RollingUpdate)
	require.NoError(t, err)
}

func TestDestroyRollingUpdateDrainError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	flavorPlugin := mock_flavor.NewMockPlugin(ctrl)
	flavorProps := types.AnyString("flavorProps")
	scaled := &scaledGroup{
		settings: groupSettings{
			instancePlugin: instancePlugin,
			flavorPlugin:   flavorPlugin,
			config: group_types.Spec{
				Flavor: group_types.FlavorPlugin{
					Properties: flavorProps,
				},
			},
		},
	}

	instID := instance.ID("id")
	inst := instance.Description{ID: instID}
	// Drain errors during a rolling update are exposed, instance Destroy is not invoked
	gomock.InOrder(
		flavorPlugin.EXPECT().Drain(flavorProps, inst).Return(fmt.Errorf("drain-error")),
	)

	err := scaled.Destroy(inst, instance.RollingUpdate)
	require.Error(t, err)
	require.EqualError(t, err, "drain-error")
}

func TestDestroyTerminateDrainError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	flavorPlugin := mock_flavor.NewMockPlugin(ctrl)
	flavorProps := types.AnyString("flavorProps")
	scaled := &scaledGroup{
		settings: groupSettings{
			instancePlugin: instancePlugin,
			flavorPlugin:   flavorPlugin,
			config: group_types.Spec{
				Flavor: group_types.FlavorPlugin{
					Properties: flavorProps,
				},
			},
		},
	}

	instID := instance.ID("id")
	inst := instance.Description{ID: instID}
	// Drain errors during a termination are not exposed, instance Destroy is still invoked
	gomock.InOrder(
		flavorPlugin.EXPECT().Drain(flavorProps, inst).Return(fmt.Errorf("drain-error")),
		instancePlugin.EXPECT().Destroy(instID, instance.Termination).Return(nil),
	)

	err := scaled.Destroy(inst, instance.Termination)
	require.NoError(t, err)
}

func TestDestroyRollingUpdateSkipDrain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	flavorPlugin := mock_flavor.NewMockPlugin(ctrl)
	flavorProps := types.AnyString("flavorProps")
	scaled := &scaledGroup{
		settings: groupSettings{
			instancePlugin: instancePlugin,
			flavorPlugin:   flavorPlugin,
			config: group_types.Spec{
				Flavor: group_types.FlavorPlugin{
					Properties: flavorProps,
				},
				Updating: group_types.Updating{
					SkipBeforeInstanceDestroy: &group_types.SkipBeforeInstanceDestroyDrain,
				},
			},
		},
	}

	instID := instance.ID("id")
	inst := instance.Description{ID: instID}
	// Drain not invoked on rolling update
	gomock.InOrder(
		instancePlugin.EXPECT().Destroy(instID, instance.RollingUpdate).Return(nil),
	)

	err := scaled.Destroy(inst, instance.RollingUpdate)
	require.NoError(t, err)
}

func TestDestroyTerminateSkipDrain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	flavorPlugin := mock_flavor.NewMockPlugin(ctrl)
	flavorProps := types.AnyString("flavorProps")
	scaled := &scaledGroup{
		settings: groupSettings{
			instancePlugin: instancePlugin,
			flavorPlugin:   flavorPlugin,
			config: group_types.Spec{
				Flavor: group_types.FlavorPlugin{
					Properties: flavorProps,
				},
				Updating: group_types.Updating{
					SkipBeforeInstanceDestroy: &group_types.SkipBeforeInstanceDestroyDrain,
				},
			},
		},
	}

	instID := instance.ID("id")
	inst := instance.Description{ID: instID}
	// Drain is still invoked since it's a termination
	gomock.InOrder(
		flavorPlugin.EXPECT().Drain(flavorProps, inst).Return(nil),
		instancePlugin.EXPECT().Destroy(instID, instance.Termination).Return(nil),
	)

	err := scaled.Destroy(inst, instance.Termination)
	require.NoError(t, err)
}

func TestIsSkipDrain(t *testing.T) {
	// Update not defined
	scaled := &scaledGroup{
		settings: groupSettings{
			config: group_types.Spec{},
		},
	}
	require.False(t, scaled.isSkipDrain())

	// SkipBeforeInstanceDestroy not defined
	scaled = &scaledGroup{
		settings: groupSettings{
			config: group_types.Spec{
				Updating: group_types.Updating{},
			},
		},
	}
	require.False(t, scaled.isSkipDrain())

	// SkipBeforeInstanceDestroy defined but empty
	scaled = &scaledGroup{
		settings: groupSettings{
			config: group_types.Spec{
				Updating: group_types.Updating{
					SkipBeforeInstanceDestroy: nil,
				},
			},
		},
	}
	require.False(t, scaled.isSkipDrain())

	// Defined and set to skip drain
	scaled = &scaledGroup{
		settings: groupSettings{
			config: group_types.Spec{
				Updating: group_types.Updating{
					SkipBeforeInstanceDestroy: &group_types.SkipBeforeInstanceDestroyDrain,
				},
			},
		},
	}
	require.True(t, scaled.isSkipDrain())
}

func TestHealthError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	flavorPlugin := mock_flavor.NewMockPlugin(ctrl)
	flavorProps := types.AnyString("flavorProps")
	scaled := &scaledGroup{
		settings: groupSettings{
			flavorPlugin: flavorPlugin,
			config: group_types.Spec{
				Flavor: group_types.FlavorPlugin{
					Properties: flavorProps,
				},
			},
		},
	}

	instID := instance.ID("id")
	inst := instance.Description{ID: instID}
	gomock.InOrder(
		flavorPlugin.EXPECT().Healthy(flavorProps, inst).Return(flavor.Healthy, fmt.Errorf("health-error")),
	)

	health := scaled.Health(inst)
	require.Equal(t, health, flavor.Unknown)
}

func TestHealth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	flavorPlugin := mock_flavor.NewMockPlugin(ctrl)
	flavorProps := types.AnyString("flavorProps")
	scaled := &scaledGroup{
		settings: groupSettings{
			flavorPlugin: flavorPlugin,
			config: group_types.Spec{
				Flavor: group_types.FlavorPlugin{
					Properties: flavorProps,
				},
			},
		},
	}

	instID := instance.ID("id")
	inst := instance.Description{ID: instID}
	gomock.InOrder(
		flavorPlugin.EXPECT().Healthy(flavorProps, inst).Return(flavor.Healthy, nil),
	)

	health := scaled.Health(inst)
	require.Equal(t, health, flavor.Healthy)
}

func TestLabelEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tags := map[string]string{
		"key": "value",
	}

	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	scaled := &scaledGroup{
		settings: groupSettings{
			instancePlugin: instancePlugin,
		},
		memberTags: tags,
	}

	gomock.InOrder(
		instancePlugin.EXPECT().DescribeInstances(tags, false).Return([]instance.Description{}, nil),
	)

	err := scaled.Label()

	require.NoError(t, err)
}

func TestLabelError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tags := map[string]string{
		"key": "value",
	}

	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	scaled := &scaledGroup{
		settings: groupSettings{
			instancePlugin: instancePlugin,
		},
		memberTags: tags,
	}

	gomock.InOrder(
		instancePlugin.EXPECT().DescribeInstances(tags, false).Return(nil, errors.New("BUG")),
	)

	err := scaled.Label()

	require.Error(t, err)
}

func TestLabelAllLabelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tags := map[string]string{
		"key": "value",
	}

	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	scaled := &scaledGroup{
		settings: groupSettings{
			instancePlugin: instancePlugin,
		},
		memberTags: tags,
	}

	gomock.InOrder(
		instancePlugin.EXPECT().DescribeInstances(tags, false).Return([]instance.Description{
			{
				ID: instance.ID("labbeled1"),
				Tags: map[string]string{
					"key":              "value",
					group.ConfigSHATag: "SHA",
				},
			},
			{
				ID: instance.ID("labbeled2"),
				Tags: map[string]string{
					"key":              "value",
					group.ConfigSHATag: "SHA",
				},
			},
		}, nil),
	)

	err := scaled.Label()

	require.NoError(t, err)
}

func TestLabelOneUnlabelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := group_types.Spec{}
	tags := map[string]string{
		"key": "value",
	}

	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	scaled := &scaledGroup{
		settings: groupSettings{
			instancePlugin: instancePlugin,
			config:         config,
		},
		memberTags: tags,
	}

	gomock.InOrder(
		instancePlugin.EXPECT().DescribeInstances(tags, false).Return([]instance.Description{
			{
				ID: instance.ID("labbeled"),
				Tags: map[string]string{
					"key":              "value",
					group.ConfigSHATag: config.InstanceHash(),
				},
			},
			{
				ID: instance.ID("unlabelled"),
				Tags: map[string]string{
					"key":              "value",
					group.ConfigSHATag: bootstrapConfigTag,
				},
			},
		}, nil),
		instancePlugin.EXPECT().Label(instance.ID("unlabelled"), map[string]string{
			"key":              "value",
			group.ConfigSHATag: config.InstanceHash(),
		}).Return(nil),
	)

	err := scaled.Label()

	require.NoError(t, err)
}

func TestUnableToLabel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := group_types.Spec{}
	tags := map[string]string{
		"key": "value",
	}

	instancePlugin := mock_instance.NewMockPlugin(ctrl)
	scaled := &scaledGroup{
		settings: groupSettings{
			instancePlugin: instancePlugin,
			config:         config,
		},
		memberTags: tags,
	}

	gomock.InOrder(
		instancePlugin.EXPECT().DescribeInstances(tags, false).Return([]instance.Description{
			{
				ID: instance.ID("labbeled"),
				Tags: map[string]string{
					"key":              "value",
					group.ConfigSHATag: config.InstanceHash(),
				},
			},
			{
				ID: instance.ID("unlabelled"),
				Tags: map[string]string{
					"key":              "value",
					group.ConfigSHATag: bootstrapConfigTag,
				},
			},
		}, nil),
		instancePlugin.EXPECT().Label(instance.ID("unlabelled"), map[string]string{
			"key":              "value",
			group.ConfigSHATag: config.InstanceHash(),
		}).Return(errors.New("BUG")),
	)

	err := scaled.Label()

	require.Error(t, err)
}
