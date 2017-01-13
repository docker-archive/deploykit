package instance

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/stretchr/testify/require"
)

func TestInstanceTypedPluginValidate(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	raw1 := json.RawMessage([]byte(`{"name":"instance","type":"xlarge1"}`))
	raw2 := json.RawMessage([]byte(`{"name":"instance","type":"xlarge2"}`))

	rawActual1 := make(chan json.RawMessage, 1)
	rawActual2 := make(chan json.RawMessage, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]instance.Plugin{
			"type1": &testPlugin{
				DoValidate: func(req json.RawMessage) error {

					rawActual1 <- req

					return nil
				},
			},
			"type2": &testPlugin{
				DoValidate: func(req json.RawMessage) error {

					rawActual2 <- req

					return nil
				},
			},
		}))
	require.NoError(t, err)

	err = NewClient(name+"/type1", socketPath).Validate(raw1)
	require.NoError(t, err)

	err = NewClient(name+"/type2", socketPath).Validate(raw2)
	require.NoError(t, err)

	err = NewClient(name+"/typeUnknown", socketPath).Validate(raw2)
	require.Error(t, err)
	require.Equal(t, "no-plugin:typeUnknown", err.Error())

	server.Stop()

	require.Equal(t, raw1, <-rawActual1)
	require.Equal(t, raw2, <-rawActual2)
}

func TestInstanceTypedPluginValidateError(t *testing.T) {

	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	raw1 := json.RawMessage([]byte(`{"name":"instance","type":"xlarge1"}`))
	rawActual1 := make(chan json.RawMessage, 1)
	raw2 := json.RawMessage([]byte(`{"name":"instance","type":"xlarge2"}`))
	rawActual2 := make(chan json.RawMessage, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(map[string]instance.Plugin{
		"type1": &testPlugin{
			DoValidate: func(req json.RawMessage) error {

				rawActual1 <- req

				return errors.New("whoops")
			},
		},
		"type2": &testPlugin{
			DoValidate: func(req json.RawMessage) error {

				rawActual2 <- req

				return errors.New("whoops2")
			},
		},
	}))
	require.NoError(t, err)

	err = NewClient(name+"/type1", socketPath).Validate(raw1)
	require.Error(t, err)
	require.Equal(t, "whoops", err.Error())

	err = NewClient(name+"/type2", socketPath).Validate(raw2)
	require.Error(t, err)
	require.Equal(t, "whoops2", err.Error())

	server.Stop()

	require.Equal(t, raw1, <-rawActual1)
	require.Equal(t, raw2, <-rawActual2)
}

func TestInstanceTypedPluginProvision(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	raw1 := json.RawMessage([]byte(`{"test":"foo"}`))
	specActual1 := make(chan instance.Spec, 1)
	raw2 := json.RawMessage([]byte(`{"test":"foo2"}`))
	specActual2 := make(chan instance.Spec, 1)
	raw3 := json.RawMessage([]byte(`{"test":"foo3"}`))
	specActual3 := make(chan instance.Spec, 1)
	spec1 := instance.Spec{
		Properties: &raw1,
	}
	spec2 := instance.Spec{
		Properties: &raw2,
	}
	spec3 := instance.Spec{
		Properties: &raw3,
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]instance.Plugin{
			"type1": &testPlugin{
				DoProvision: func(req instance.Spec) (*instance.ID, error) {
					specActual1 <- req
					return nil, nil
				},
			},
			"type2": &testPlugin{
				DoProvision: func(req instance.Spec) (*instance.ID, error) {
					specActual2 <- req
					v := instance.ID("test")
					return &v, nil
				},
			},
			"error": &testPlugin{
				DoProvision: func(req instance.Spec) (*instance.ID, error) {
					specActual3 <- req
					return nil, errors.New("nope")
				},
			},
		}))
	require.NoError(t, err)

	var id *instance.ID
	id, err = NewClient(name+"/type1", socketPath).Provision(spec1)
	require.NoError(t, err)
	require.Nil(t, id)

	id, err = NewClient(name+"/type2", socketPath).Provision(spec2)
	require.NoError(t, err)
	require.Equal(t, "test", string(*id))

	_, err = NewClient(name+"/error", socketPath).Provision(spec3)
	require.Error(t, err)
	require.Equal(t, "nope", err.Error())

	server.Stop()

	require.Equal(t, spec1, <-specActual1)
	require.Equal(t, spec2, <-specActual2)
	require.Equal(t, spec3, <-specActual3)
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
			"type1": &testPlugin{
				DoDestroy: func(req instance.ID) error {
					instActual1 <- req
					return nil
				},
			},
			"type2": &testPlugin{
				DoDestroy: func(req instance.ID) error {
					instActual2 <- req
					return errors.New("can't do")
				},
			},
		}))
	require.NoError(t, err)

	err = NewClient(name+"/type1", socketPath).Destroy(inst1)
	require.NoError(t, err)

	err = NewClient(name+"/type2", socketPath).Destroy(inst2)
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
	list1 := []instance.Description{
		{ID: instance.ID("boo1")}, {ID: instance.ID("boop")},
	}
	tags2 := map[string]string{
		"foo": "bar",
	}
	tagsActual2 := make(chan map[string]string, 1)
	list2 := []instance.Description{
		{ID: instance.ID("boo")}, {ID: instance.ID("boop2")},
	}
	tags3 := map[string]string{
		"foo": "bar",
	}
	tagsActual3 := make(chan map[string]string, 1)
	list3 := []instance.Description{
		{ID: instance.ID("boo3")}, {ID: instance.ID("boop")},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServerWithTypes(
		map[string]instance.Plugin{
			"type1": &testPlugin{
				DoDescribeInstances: func(req map[string]string) ([]instance.Description, error) {
					tagsActual1 <- req
					return list1, nil
				},
			},
			"type2": &testPlugin{
				DoDescribeInstances: func(req map[string]string) ([]instance.Description, error) {
					tagsActual2 <- req
					return list2, nil
				},
			},
			"type3": &testPlugin{
				DoDescribeInstances: func(req map[string]string) ([]instance.Description, error) {
					tagsActual3 <- req
					return list3, errors.New("bad")
				},
			},
		}))

	l, err := NewClient(name+"/type1", socketPath).DescribeInstances(tags1)
	require.NoError(t, err)
	require.Equal(t, list1, l)

	l, err = NewClient(name+"/type2", socketPath).DescribeInstances(tags2)
	require.NoError(t, err)
	require.Equal(t, list2, l)

	_, err = NewClient(name+"/type3", socketPath).DescribeInstances(tags3)
	require.Error(t, err)
	require.Equal(t, "bad", err.Error())

	server.Stop()
	require.Equal(t, tags1, <-tagsActual1)
	require.Equal(t, tags2, <-tagsActual2)
	require.Equal(t, tags3, <-tagsActual3)
}
