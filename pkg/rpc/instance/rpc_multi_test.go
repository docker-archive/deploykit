package instance

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/instance"
	testing_instance "github.com/docker/infrakit/pkg/testing/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func must(i instance.Plugin, err error) instance.Plugin {
	if err != nil {
		panic(err)
	}
	return i
}

func TestInstanceTypedPluginValidate(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	raw1 := types.AnyString(`{"name":"instance","type":"xlarge1"}`)
	raw2 := types.AnyString(`{"name":"instance","type":"xlarge2"}`)

	rawActual1 := make(chan *types.Any, 1)
	rawActual2 := make(chan *types.Any, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]instance.Plugin{
			"type1": &testing_instance.Plugin{
				DoValidate: func(req *types.Any) error {

					rawActual1 <- req

					return nil
				},
			},
			"type2": &testing_instance.Plugin{
				DoValidate: func(req *types.Any) error {

					rawActual2 <- req

					return nil
				},
			},
		}))
	require.NoError(t, err)

	err = must(NewClient(plugin.Name(name+"/type1"), socketPath)).Validate(raw1)
	require.NoError(t, err)

	err = must(NewClient(plugin.Name(name+"/type2"), socketPath)).Validate(raw2)
	require.NoError(t, err)

	err = must(NewClient(plugin.Name(name+"/typeUnknown"), socketPath)).Validate(raw2)
	require.Error(t, err)
	require.Equal(t, "no-plugin:typeUnknown", err.Error())

	server.Stop()

	require.Equal(t, raw1, <-rawActual1)
	require.Equal(t, raw2, <-rawActual2)
}

func TestInstanceTypedPluginValidateError(t *testing.T) {

	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	raw1 := types.AnyString(`{"name":"instance","type":"xlarge1"}`)
	rawActual1 := make(chan *types.Any, 1)
	raw2 := types.AnyString(`{"name":"instance","type":"xlarge2"}`)
	rawActual2 := make(chan *types.Any, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(map[string]instance.Plugin{
		"type1": &testing_instance.Plugin{
			DoValidate: func(req *types.Any) error {

				rawActual1 <- req

				return errors.New("whoops")
			},
		},
		"type2": &testing_instance.Plugin{
			DoValidate: func(req *types.Any) error {

				rawActual2 <- req

				return errors.New("whoops2")
			},
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(plugin.Name(name+"/type1"), socketPath)).Validate(raw1)
	require.Error(t, err)
	require.Equal(t, "whoops", err.Error())

	err = must(NewClient(plugin.Name(name+"/type2"), socketPath)).Validate(raw2)
	require.Error(t, err)
	require.Equal(t, "whoops2", err.Error())

	server.Stop()

	require.Equal(t, raw1, <-rawActual1)
	require.Equal(t, raw2, <-rawActual2)
}

func TestInstanceTypedPluginProvision(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	raw1 := types.AnyString(`{"test":"foo"}`)
	specActual1 := make(chan instance.Spec, 1)
	raw2 := types.AnyString(`{"test":"foo2"}`)
	specActual2 := make(chan instance.Spec, 1)
	raw3 := types.AnyString(`{"test":"foo3"}`)
	specActual3 := make(chan instance.Spec, 1)
	spec1 := instance.Spec{
		Properties: raw1,
	}
	spec2 := instance.Spec{
		Properties: raw2,
	}
	spec3 := instance.Spec{
		Properties: raw3,
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]instance.Plugin{
			"type1": &testing_instance.Plugin{
				DoProvision: func(req instance.Spec) (*instance.ID, error) {
					specActual1 <- req
					return nil, nil
				},
			},
			"type2": &testing_instance.Plugin{
				DoProvision: func(req instance.Spec) (*instance.ID, error) {
					specActual2 <- req
					v := instance.ID("test")
					return &v, nil
				},
			},
			"error": &testing_instance.Plugin{
				DoProvision: func(req instance.Spec) (*instance.ID, error) {
					specActual3 <- req
					return nil, errors.New("nope")
				},
			},
		}))
	require.NoError(t, err)

	var id *instance.ID
	id, err = must(NewClient(plugin.Name(name+"/type1"), socketPath)).Provision(spec1)
	require.NoError(t, err)
	require.Nil(t, id)

	id, err = must(NewClient(plugin.Name(name+"/type2"), socketPath)).Provision(spec2)
	require.NoError(t, err)
	require.Equal(t, "test", string(*id))

	_, err = must(NewClient(plugin.Name(name+"/error"), socketPath)).Provision(spec3)
	require.Error(t, err)
	require.Equal(t, "nope", err.Error())

	server.Stop()

	require.Equal(t, spec1, <-specActual1)
	require.Equal(t, spec2, <-specActual2)
	require.Equal(t, spec3, <-specActual3)
}

func TestInstanceTypedPluginLabel(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inst1 := instance.ID("hello1")
	labels1 := map[string]string{"l1": "v1"}
	instActual1 := make(chan instance.ID, 1)
	labelActual1 := make(chan map[string]string, 1)

	inst2 := instance.ID("hello2")
	labels2 := map[string]string{"l1": "v2"}
	instActual2 := make(chan instance.ID, 2)
	labelActual2 := make(chan map[string]string, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]instance.Plugin{
			"type1": &testing_instance.Plugin{
				DoLabel: func(req instance.ID, labels map[string]string) error {
					instActual1 <- req
					labelActual1 <- labels
					return nil
				},
			},
			"type2": &testing_instance.Plugin{
				DoLabel: func(req instance.ID, labels map[string]string) error {
					instActual2 <- req
					labelActual2 <- labels
					return errors.New("can't do")
				},
			},
		}))
	require.NoError(t, err)

	err = must(NewClient(plugin.Name(name+"/type1"), socketPath)).Label(inst1, labels1)
	require.NoError(t, err)

	err = must(NewClient(plugin.Name(name+"/type2"), socketPath)).Label(inst2, labels2)
	require.Error(t, err)
	require.Equal(t, "can't do", err.Error())

	server.Stop()

	require.Equal(t, inst1, <-instActual1)
	require.Equal(t, inst2, <-instActual2)
	require.Equal(t, labels1, <-labelActual1)
	require.Equal(t, labels2, <-labelActual2)
}

func TestInstanceTypedPluginDestroy(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	inst1 := instance.ID("hello1")
	instActual1 := make(chan instance.ID, 1)
	inst2 := instance.ID("hello2")
	instActual2 := make(chan instance.ID, 2)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]instance.Plugin{
			"type1": &testing_instance.Plugin{
				DoDestroy: func(req instance.ID, ctx instance.Context) error {
					instActual1 <- req
					return nil
				},
			},
			"type2": &testing_instance.Plugin{
				DoDestroy: func(req instance.ID, ctx instance.Context) error {
					instActual2 <- req
					return errors.New("can't do")
				},
			},
		}))
	require.NoError(t, err)

	err = must(NewClient(plugin.Name(name+"/type1"), socketPath)).Destroy(inst1, instance.Termination)
	require.NoError(t, err)

	err = must(NewClient(plugin.Name(name+"/type2"), socketPath)).Destroy(inst2, instance.Termination)
	require.Error(t, err)
	require.Equal(t, "can't do", err.Error())

	server.Stop()

	require.Equal(t, inst1, <-instActual1)
	require.Equal(t, inst2, <-instActual2)
}

func TestInstanceTypedPluginDescribeInstances(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	tags1 := map[string]string{}
	tagsActual1 := make(chan map[string]string, 1)
	propertiesActual1 := make(chan bool, 1)
	list1 := []instance.Description{
		{ID: instance.ID("boo1")}, {ID: instance.ID("boop")},
	}
	tags2 := map[string]string{
		"foo": "bar",
	}
	tagsActual2 := make(chan map[string]string, 1)
	propertiesActual2 := make(chan bool, 1)
	list2 := []instance.Description{
		{ID: instance.ID("boo")}, {ID: instance.ID("boop2")},
	}
	tags3 := map[string]string{
		"foo": "bar",
	}
	tagsActual3 := make(chan map[string]string, 1)
	propertiesActual3 := make(chan bool, 1)
	list3 := []instance.Description{
		{ID: instance.ID("boo3")}, {ID: instance.ID("boop")},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]instance.Plugin{
			"type1": &testing_instance.Plugin{
				DoDescribeInstances: func(req map[string]string, properties bool) ([]instance.Description, error) {
					tagsActual1 <- req
					propertiesActual1 <- properties
					return list1, nil
				},
			},
			"type2": &testing_instance.Plugin{
				DoDescribeInstances: func(req map[string]string, properties bool) ([]instance.Description, error) {
					tagsActual2 <- req
					propertiesActual2 <- properties
					return list2, nil
				},
			},
			"type3": &testing_instance.Plugin{
				DoDescribeInstances: func(req map[string]string, properties bool) ([]instance.Description, error) {
					tagsActual3 <- req
					propertiesActual3 <- properties
					return list3, errors.New("bad")
				},
			},
		}))

	l, err := must(NewClient(plugin.Name(name+"/type1"), socketPath)).DescribeInstances(tags1, false)
	require.NoError(t, err)
	require.Equal(t, list1, l)

	l, err = must(NewClient(plugin.Name(name+"/type2"), socketPath)).DescribeInstances(tags2, true)
	require.NoError(t, err)
	require.Equal(t, list2, l)

	_, err = must(NewClient(plugin.Name(name+"/type3"), socketPath)).DescribeInstances(tags3, false)
	require.Error(t, err)
	require.Equal(t, "bad", err.Error())

	server.Stop()
	require.Equal(t, tags1, <-tagsActual1)
	require.Equal(t, tags2, <-tagsActual2)
	require.Equal(t, tags3, <-tagsActual3)
	require.Equal(t, false, <-propertiesActual1)
	require.Equal(t, true, <-propertiesActual2)
	require.Equal(t, false, <-propertiesActual3)
}
