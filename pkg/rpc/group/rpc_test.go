package group

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	testing_group "github.com/docker/infrakit/pkg/testing/group"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path"
)

func must(p group.Plugin, err error) group.Plugin {
	if err != nil {
		panic(err)
	}
	return p
}

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return path.Join(dir, "group-impl-test")
}

func nameFromPath(p string) plugin.Name {
	return plugin.Name(filepath.Base(p))
}

func TestGroupPluginCommitGroup(t *testing.T) {
	socketPath := tempSocket()

	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: types.AnyString(`{"foo":"bar"}`),
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_group.Plugin{
		DoCommitGroup: func(req group.Spec, pretend bool) (string, error) {
			groupSpecActual <- req
			return "commit details", nil
		},
	}))

	// Make call
	details, err := must(NewClient(nameFromPath(socketPath), socketPath)).CommitGroup(groupSpec, false)
	require.NoError(t, err)
	require.Equal(t, "commit details", details)

	server.Stop()

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupNamedPluginCommitGroupDefault(t *testing.T) {
	socketPath := tempSocket()

	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group1"),
		Properties: types.AnyString(`{"foo":"bar"}`),
	}

	server, err := rpc_server.StartPluginAtPath(socketPath,
		PluginServerWithGroups(
			func() (map[group.ID]group.Plugin, error) {
				return map[group.ID]group.Plugin{
					group.ID("*"): &testing_group.Plugin{
						DoCommitGroup: func(req group.Spec, pretend bool) (string, error) {
							groupSpecActual <- req
							return "commit details", nil
						},
					},
				}, nil
			}))

	// Make call
	details, err := must(NewClient(nameFromPath(socketPath), socketPath)).CommitGroup(groupSpec, false)
	require.NoError(t, err)
	require.Equal(t, "commit details", details)

	server.Stop()

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupNamedPluginCommitGroup(t *testing.T) {
	socketPath := tempSocket()

	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group1"),
		Properties: types.AnyString(`{"foo":"bar"}`),
	}

	server, err := rpc_server.StartPluginAtPath(socketPath,
		PluginServerWithGroups(
			func() (map[group.ID]group.Plugin, error) {
				return map[group.ID]group.Plugin{
					group.ID(""): &testing_group.Plugin{
						DoCommitGroup: func(req group.Spec, pretend bool) (string, error) {
							panic("shouldn't be here")
						},
					},
					group.ID("group1"): &testing_group.Plugin{
						DoCommitGroup: func(req group.Spec, pretend bool) (string, error) {
							groupSpecActual <- req
							return "commit details", nil
						},
					},
				}, nil
			}))

	// Make call
	details, err := must(NewClient(nameFromPath(socketPath).Sub("group1"), socketPath)).CommitGroup(groupSpec, false)
	require.NoError(t, err)
	require.Equal(t, "commit details", details)

	server.Stop()

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginCommitGroupError(t *testing.T) {
	socketPath := tempSocket()

	groupSpecActual := make(chan group.Spec, 1)
	groupSpec := group.Spec{
		ID:         group.ID("group"),
		Properties: types.AnyString(`{"foo":"bar"}`),
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_group.Plugin{
		DoCommitGroup: func(req group.Spec, pretend bool) (string, error) {
			groupSpecActual <- req
			return "", errors.New("error")
		},
	}))
	require.NoError(t, err)

	_, err = must(NewClient(nameFromPath(socketPath), socketPath)).CommitGroup(groupSpec, false)
	require.Error(t, err)
	require.Equal(t, "error", err.Error())

	server.Stop()

	require.Equal(t, groupSpec, <-groupSpecActual)
}

func TestGroupPluginFreeGroup(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_group.Plugin{
		DoFreeGroup: func(req group.ID) error {
			idActual <- req
			return nil
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(nameFromPath(socketPath), socketPath)).FreeGroup(id)
	require.NoError(t, err)

	server.Stop()
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginFreeGroupError(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_group.Plugin{
		DoFreeGroup: func(req group.ID) error {
			idActual <- req
			return errors.New("no")
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(nameFromPath(socketPath), socketPath)).FreeGroup(id)
	require.Error(t, err)
	require.Equal(t, "no", err.Error())

	server.Stop()
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginDestroyGroup(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_group.Plugin{
		DoDestroyGroup: func(req group.ID) error {
			idActual <- req
			return nil
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(nameFromPath(socketPath), socketPath)).DestroyGroup(id)
	require.NoError(t, err)

	server.Stop()
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginDestroyGroupError(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_group.Plugin{
		DoDestroyGroup: func(req group.ID) error {
			idActual <- req
			return errors.New("no")
		},
	}))
	require.NoError(t, err)

	err = must(NewClient(nameFromPath(socketPath), socketPath)).DestroyGroup(id)
	require.Error(t, err)
	require.Equal(t, "no", err.Error())

	server.Stop()
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginInspectGroup(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)

	desc := group.Description{
		Instances: []instance.Description{
			{ID: instance.ID("hey")},
		},
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_group.Plugin{
		DoDescribeGroup: func(req group.ID) (group.Description, error) {
			idActual <- req
			return desc, nil
		},
	}))

	res, err := must(NewClient(nameFromPath(socketPath), socketPath)).DescribeGroup(id)
	require.NoError(t, err)
	require.Equal(t, desc, res)

	server.Stop()
	require.Equal(t, id, <-idActual)
}

func TestGroupPluginInspectGroupError(t *testing.T) {
	socketPath := tempSocket()

	id := group.ID("group")
	idActual := make(chan group.ID, 1)
	desc := group.Description{
		Instances: []instance.Description{
			{ID: instance.ID("hey")},
		},
	}

	server, err := rpc_server.StartPluginAtPath(socketPath, PluginServer(&testing_group.Plugin{
		DoDescribeGroup: func(req group.ID) (group.Description, error) {
			idActual <- req
			return desc, errors.New("no")
		},
	}))
	require.NoError(t, err)

	_, err = must(NewClient(nameFromPath(socketPath), socketPath)).DescribeGroup(id)
	require.Error(t, err)
	require.Equal(t, "no", err.Error())

	server.Stop()
	require.Equal(t, id, <-idActual)
}

func TestGroupNamedPluginDestroyInstances(t *testing.T) {
	socketPath := tempSocket()

	idsActual := make(chan []instance.ID, 1)
	gidActual := make(chan group.ID, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath,
		PluginServerWithGroups(
			func() (map[group.ID]group.Plugin, error) {
				return map[group.ID]group.Plugin{
					group.ID(""): &testing_group.Plugin{
						DoDestroyInstances: func(gid group.ID, ids []instance.ID) error {
							panic("shouldn't be here")
						},
					},
					group.ID("group1"): &testing_group.Plugin{
						DoDestroyInstances: func(gid group.ID, ids []instance.ID) error {
							gidActual <- gid
							idsActual <- ids
							return nil
						},
					},
				}, nil
			}))

	// Make call
	gid := group.ID("group1")
	ids := []instance.ID{instance.ID("foo"), instance.ID("bar")}
	err = must(NewClient(nameFromPath(socketPath).WithType(gid), socketPath)).DestroyInstances(gid, ids)
	require.NoError(t, err)

	server.Stop()

	require.Equal(t, ids, <-idsActual)
	require.Equal(t, gid, <-gidActual)
}

func TestGroupNamedPluginSizeSetSize(t *testing.T) {
	socketPath := tempSocket()

	gidActual := make(chan group.ID, 1)
	sizeActual := make(chan int, 1)

	server, err := rpc_server.StartPluginAtPath(socketPath,
		PluginServerWithGroups(
			func() (map[group.ID]group.Plugin, error) {
				return map[group.ID]group.Plugin{
					group.ID(""): &testing_group.Plugin{
						DoSize: func(gid group.ID) (int, error) {
							panic("shouldn't be here")
						},
						DoSetSize: func(gid group.ID, size int) error {
							panic("shouldn't be here")
						},
					},
					group.ID("group1"): &testing_group.Plugin{
						DoSize: func(gid group.ID) (int, error) {
							gidActual <- gid
							return 100, nil
						},
						DoSetSize: func(gid group.ID, size int) error {
							sizeActual <- size
							return nil
						},
					},
				}, nil
			}))

	// Make call
	gid := group.ID("group1")

	size, err := must(NewClient(nameFromPath(socketPath).Sub("group1"), socketPath)).Size(gid)
	require.NoError(t, err)
	require.Equal(t, 100, size)

	err = must(NewClient(nameFromPath(socketPath).WithType(gid), socketPath)).SetSize(gid, 1001)
	require.NoError(t, err)
	require.Equal(t, 100, size)

	server.Stop()

	require.Equal(t, 1001, <-sizeActual)
	require.Equal(t, gid, <-gidActual)
}
