package group

import (
	"fmt"
	"testing"

	group_mock "github.com/docker/infrakit/pkg/mock/spi/group"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAsController(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	spec, err := types.SpecFromString(`
kind: group
version: Group/1.0
metadata:
  name: group/workers
properties:
  target: 100
  min: 20
  max: 200
`)
	require.NoError(t, err)

	gspecs := []group.Spec{
		{ID: "workers", Properties: types.AnyYAMLMust([]byte(`
  Flavor:
    Plugin: swarm
    Properties:
        docker: /var/run/docker.sock
  Instance:
    Plugin: aws/ec2-instance
    Properties:
        type: xlarge
`))}}

	gDescription := group.Description{
		Converged: true,
		Instances: []instance.Description{
			{ID: instance.ID("h1")},
			{ID: instance.ID("h2")},
		},
	}

	g := group_mock.NewMockPlugin(ctrl)
	g.EXPECT().InspectGroups().Do(
		func() ([]group.Spec, error) {
			return gspecs, nil
		},
	).AnyTimes().Return(gspecs, nil)

	g.EXPECT().DescribeGroup(gomock.Eq(group.ID("workers"))).Do(
		func(gid group.ID) (group.Description, error) {
			require.Equal(t, group.ID("workers"), gid)
			return gDescription, nil
		}).AnyTimes().Return(gDescription, nil)
	g.EXPECT().CommitGroup(gomock.Any(), true).Do(
		func(gspec group.Spec, pretend bool) (string, error) {
			require.Equal(t, group.ID("workers"), gspec.ID)
			require.EqualValues(t, spec.Properties, gspec.Properties)
			return "ok", nil
		}).Return("ok", nil)

	c := AsController(plugin.NewAddressable("group", plugin.Name("group-stateless/"), ""), g)

	c.Plan(controller.Enforce, spec)
}

func TestAsController2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	spec, err := types.SpecFromString(`
kind: group
version: Group/1.0
metadata:
  name: workers
properties:
  Flavor:
    Plugin: swarm
    Properties:
        docker: /var/run/docker.sock
  Instance:
    Plugin: aws/ec2-instance
    Properties:
        type: xlarge
`)
	require.NoError(t, err)

	gspecs := []group.Spec{
		{ID: "workers", Properties: types.AnyYAMLMust([]byte(`
  Flavor:
    Plugin: swarm
    Properties:
        docker: /var/run/docker.sock
  Instance:
    Plugin: aws/ec2-instance
    Properties:
        type: xlarge
`))}}

	gDescription := group.Description{
		Converged: true,
		Instances: []instance.Description{
			{ID: instance.ID("h1")},
			{ID: instance.ID("h2")},
		},
	}

	g := group_mock.NewMockPlugin(ctrl)
	g.EXPECT().InspectGroups().Do(
		func() ([]group.Spec, error) {
			return gspecs, nil
		},
	).AnyTimes().Return(gspecs, nil)

	g.EXPECT().DescribeGroup(gomock.Eq(group.ID("workers"))).Do(
		func(gid group.ID) (group.Description, error) {
			require.Equal(t, group.ID("workers"), gid)
			return gDescription, nil
		}).AnyTimes().Return(gDescription, nil)
	g.EXPECT().CommitGroup(gomock.Any(), false).Do(
		func(gspec group.Spec, pretend bool) (string, error) {
			require.Equal(t, group.ID("workers"), gspec.ID)
			require.EqualValues(t, spec.Properties, gspec.Properties)

			prop := map[string]interface{}{}
			require.NoError(t, gspec.Properties.Decode(&prop))
			require.Equal(t, "/var/run/docker.sock", types.Get(types.PathFromString("Flavor/Properties/docker"), prop))
			require.Equal(t, "xlarge", types.Get(types.PathFromString("Instance/Properties/type"), prop))
			return "ok", nil
		}).Return("ok", nil)

	c := AsController(plugin.NewAddressable("group", plugin.Name("group-stateless/"), ""), g)

	c.Commit(controller.Enforce, spec)
}

func TestAsControllerDescribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	spec, err := types.SpecFromString(`
kind: group
metadata:
  name: workers
properties:
  Flavor:
    Plugin: swarm
    Properties:
        docker: /var/run/docker.sock
  Instance:
    Plugin: aws/ec2-instance
    Properties:
        type: xlarge
`)
	require.NoError(t, err)

	gspecs := []group.Spec{
		{ID: "workers", Properties: types.AnyYAMLMust([]byte(`
  Flavor:
    Plugin: swarm
    Properties:
        docker: /var/run/docker.sock
  Instance:
    Plugin: aws/ec2-instance
    Properties:
        type: xlarge
`))}}

	gDescription := group.Description{
		Converged: true,
		Instances: []instance.Description{
			{ID: instance.ID("h1")},
			{ID: instance.ID("h2")},
		},
	}

	g := group_mock.NewMockPlugin(ctrl)
	g.EXPECT().InspectGroups().Do(
		func() ([]group.Spec, error) {
			return gspecs, nil
		},
	).AnyTimes().Return(gspecs, nil)

	g.EXPECT().DescribeGroup(gomock.Eq(group.ID("workers"))).Do(
		func(gid group.ID) (group.Description, error) {
			require.Equal(t, group.ID("workers"), gid)
			return gDescription, nil
		}).AnyTimes().Return(gDescription, nil)

	c := AsController(plugin.NewAddressable("group", plugin.Name("group-stateless/"), ""), g)

	objects, err := c.Describe(nil)
	require.NoError(t, err)
	require.Equal(t, spec.Properties, objects[0].Spec.Properties)

	buff, err := types.AnyValueMust(objects).MarshalYAML()
	require.NoError(t, err)

	fmt.Println(string(buff))

	objects, err = c.Describe(&types.Metadata{Name: "workers"})
	require.NoError(t, err)
	require.Equal(t, spec.Properties, objects[0].Spec.Properties)

	_, err = types.AnyValueMust(objects).MarshalYAML()
	require.NoError(t, err)
}

func TestAsControllerFree(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	spec, err := types.SpecFromString(`
kind: group
metadata:
  name: workers
properties:
  Flavor:
    Plugin: swarm
    Properties:
        docker: /var/run/docker.sock
  Instance:
    Plugin: aws/ec2-instance
    Properties:
        type: xlarge
`)
	require.NoError(t, err)

	gspecs := []group.Spec{
		{ID: "workers", Properties: types.AnyYAMLMust([]byte(`
  Flavor:
    Plugin: swarm
    Properties:
        docker: /var/run/docker.sock
  Instance:
    Plugin: aws/ec2-instance
    Properties:
        type: xlarge
`))}}

	gDescription := group.Description{
		Converged: true,
		Instances: []instance.Description{
			{ID: instance.ID("h1")},
			{ID: instance.ID("h2")},
		},
	}

	g := group_mock.NewMockPlugin(ctrl)
	g.EXPECT().InspectGroups().Do(
		func() ([]group.Spec, error) {
			return gspecs, nil
		},
	).AnyTimes().Return(gspecs, nil)

	g.EXPECT().DescribeGroup(gomock.Eq(group.ID("workers"))).Do(
		func(gid group.ID) (group.Description, error) {
			require.Equal(t, group.ID("workers"), gid)
			return gDescription, nil
		}).AnyTimes().Return(gDescription, nil)

	g.EXPECT().FreeGroup(gomock.Eq(group.ID("workers"))).Do(
		func(gid group.ID) error {
			require.Equal(t, group.ID("workers"), gid)
			return nil
		}).AnyTimes().Return(nil)

	c := AsController(plugin.NewAddressable("group", plugin.Name("group-stateless/"), ""), g)

	objects, err := c.Free(nil)
	require.NoError(t, err)
	require.Equal(t, spec.Properties, objects[0].Spec.Properties)

	buff, err := types.AnyValueMust(objects).MarshalYAML()
	require.NoError(t, err)

	fmt.Println(string(buff))

	objects, err = c.Free(&types.Metadata{Name: "workers"})
	require.NoError(t, err)
	require.Equal(t, spec.Properties, objects[0].Spec.Properties)

	_, err = types.AnyValueMust(objects).MarshalYAML()
	require.NoError(t, err)
}
