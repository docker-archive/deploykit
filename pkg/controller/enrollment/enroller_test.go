package enrollment

import (
	"fmt"
	"testing"

	enrollment "github.com/docker/infrakit/pkg/controller/enrollment/types"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	group_test "github.com/docker/infrakit/pkg/testing/group"
	instance_test "github.com/docker/infrakit/pkg/testing/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

type fakeLeader func() (bool, error)

func (f fakeLeader) IsLeader() (bool, error) {
	return f()
}

type fakePlugins map[string]*plugin.Endpoint

func (f fakePlugins) Find(name plugin.Name) (*plugin.Endpoint, error) {
	lookup, _ := name.GetLookupAndType()
	if v, has := f[lookup]; has {
		return v, nil
	}
	return nil, fmt.Errorf("not found")
}

func (f fakePlugins) List() (map[string]*plugin.Endpoint, error) {
	return (map[string]*plugin.Endpoint)(f), nil
}

func TestEnroller(t *testing.T) {

	source := []instance.Description{
		{ID: instance.ID("h1")},
		{ID: instance.ID("h2")},
		{ID: instance.ID("h3")},
	}

	enrolled := []instance.Description{
		{ID: instance.ID("nfs1"), Tags: map[string]string{"infrakit.enrollment.sourceID": "h1"}},
		{ID: instance.ID("nfs2"), Tags: map[string]string{"infrakit.enrollment.sourceID": "h2"}},
		{ID: instance.ID("nfs5"), Tags: map[string]string{"infrakit.enrollment.sourceID": "h5"}},
	}

	seen := make(chan []interface{}, 10)

	enroller := newEnroller(
		func() discovery.Plugins {
			return fakePlugins{
				"test": &plugin.Endpoint{},
			}
		},
		fakeLeader(func() (bool, error) { return false, nil }),
		enrollment.Options{})
	enroller.groupPlugin = &group_test.Plugin{
		DoDescribeGroup: func(gid group.ID) (group.Description, error) {
			result := group.Description{Instances: source}
			return result, nil
		},
	}
	enroller.instancePlugin = &instance_test.Plugin{
		DoDescribeInstances: func(t map[string]string, p bool) ([]instance.Description, error) {
			return enrolled, nil
		},
		DoProvision: func(spec instance.Spec) (*instance.ID, error) {

			seen <- []interface{}{spec, "Provision"}
			return nil, nil
		},
		DoDestroy: func(id instance.ID, ctx instance.Context) error {

			seen <- []interface{}{id, ctx, "Destroy"}
			return nil
		},
	}

	require.False(t, enroller.Running())

	spec := types.Spec{}
	require.NoError(t, types.AnyYAMLMust([]byte(`
kind: enrollment
metadata:
  name: nfs
properties:
  List: group/workers
  Instance:
    Plugin: nfs/authorization
    Properties:
       host: \{\{.ID\}\}
       iops: 10
options:
  SourceKeySelector: \{\{.ID\}\}

`)).Decode(&spec))

	require.NoError(t, enroller.updateSpec(spec))

	st, err := enroller.getSourceKeySelectorTemplate()
	require.NoError(t, err)
	require.NotNil(t, st)

	et, err := enroller.getEnrollmentPropertiesTemplate()
	require.NoError(t, err)
	require.NotNil(t, et)

	require.NoError(t, err)

	s, err := enroller.getSourceInstances()
	require.NoError(t, err)
	require.Equal(t, source, s)

	found, err := enroller.getEnrolledInstances()
	require.NoError(t, err)
	require.Equal(t, enrolled, found)

	require.NoError(t, enroller.sync())

	// check the provision and destroy calls
	require.Equal(t, []interface{}{
		instance.Spec{
			Properties: types.AnyString(`{"host":"h3","iops":10}`),
			Tags: map[string]string{
				"infrakit.enrollment.sourceID": "h3",
				"infrakit.enrollment.name":     "nfs",
			},
		},
		"Provision",
	}, <-seen)
	require.Equal(t, []interface{}{
		instance.ID("nfs5"),
		instance.Termination,
		"Destroy",
	}, <-seen)
}
