package group

import (
	"testing"

	mock_instance "github.com/docker/infrakit/pkg/mock/spi/instance"
	"github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

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

	config := types.Spec{}
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

	config := types.Spec{}
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
