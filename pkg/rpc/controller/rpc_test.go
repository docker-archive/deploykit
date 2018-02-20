package controller

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/controller"
	testing_controller "github.com/docker/infrakit/pkg/testing/controller"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestControllerPlan(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	smallSpec := types.Spec{Metadata: types.Metadata{Name: "small"}}
	largeSpec := types.Spec{Metadata: types.Metadata{Name: "large"}}
	smallObject := types.Object{Spec: smallSpec}
	smallPlan := controller.Plan{Message: []string{"increase size"}}

	smallActual := make(chan []interface{}, 1)

	small := &testing_controller.Controller{
		DoPlan: func(operation controller.Operation, spec types.Spec) (types.Object, controller.Plan, error) {
			if reflect.DeepEqual(spec, largeSpec) {
				return smallObject, smallPlan, fmt.Errorf("bad")
			}

			smallActual <- []interface{}{operation, spec}

			return smallObject, smallPlan, nil
		},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, Server(small))
	require.NoError(t, err)

	a1, p1, err := must(NewClient(plugin.Name(name), socketPath)).Plan(controller.Enforce, smallSpec)
	require.NoError(t, err)

	_, _, err = must(NewClient(plugin.Name("unknown"), socketPath)).Plan(controller.Enforce, smallSpec)
	require.Error(t, err)

	smallArgs := <-smallActual

	require.EqualValues(t, controller.Enforce, smallArgs[0].(controller.Operation))
	require.Equal(t, smallSpec, smallArgs[1])
	require.Equal(t, smallObject, a1)
	require.Equal(t, smallPlan, p1)

	// now return error
	_, _, err = must(NewClient(plugin.Name(name), socketPath)).Plan(controller.Enforce, largeSpec)
	require.Error(t, err)

	server.Stop()

}

func TestControllerCommit(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	smallSpec := types.Spec{Metadata: types.Metadata{Name: "small"}}
	largeSpec := types.Spec{Metadata: types.Metadata{Name: "large"}}
	smallObject := types.Object{Spec: smallSpec}

	smallActual := make(chan []interface{}, 1)

	small := &testing_controller.Controller{
		DoCommit: func(operation controller.Operation, spec types.Spec) (types.Object, error) {
			if reflect.DeepEqual(spec, largeSpec) {
				return smallObject, fmt.Errorf("bad")
			}

			smallActual <- []interface{}{operation, spec}

			return smallObject, nil
		},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, Server(small))
	require.NoError(t, err)

	a1, err := must(NewClient(plugin.Name(name), socketPath)).Commit(controller.Enforce, smallSpec)
	require.NoError(t, err)

	_, err = must(NewClient(plugin.Name("unknown"), socketPath)).Commit(controller.Enforce, smallSpec)
	require.Error(t, err)

	smallArgs := <-smallActual

	require.EqualValues(t, controller.Enforce, smallArgs[0].(controller.Operation))
	require.Equal(t, smallSpec, smallArgs[1])
	require.Equal(t, smallObject, a1)

	_, err = must(NewClient(plugin.Name(name), socketPath)).Commit(controller.Enforce, largeSpec)
	require.Error(t, err)
	server.Stop()

}

func TestControllerDescribe(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	smallSpec := types.Spec{Metadata: types.Metadata{Name: "small"}}
	smallObject := types.Object{Spec: smallSpec}

	smallActual := make(chan []interface{}, 1)

	small := &testing_controller.Controller{
		DoDescribe: func(search *types.Metadata) ([]types.Object, error) {
			if search == nil {
				return nil, fmt.Errorf("boom")
			}

			smallActual <- []interface{}{*search}

			return []types.Object{smallObject}, nil
		},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, Server(small))
	require.NoError(t, err)

	smallSearch := (types.Metadata{Name: "small"}).AddTagsFromStringSlice([]string{"a=b", "c=d"})

	a1, err := must(NewClient(plugin.Name(name), socketPath)).Describe(&smallSearch)
	require.NoError(t, err)

	_, err = must(NewClient(plugin.Name("unknown"), socketPath)).Describe(&smallSearch)
	require.Error(t, err)

	smallArgs := <-smallActual

	require.EqualValues(t, smallSearch, smallArgs[0])
	require.Equal(t, []types.Object{smallObject}, a1)

	// now return error
	_, err = must(NewClient(plugin.Name(name), socketPath)).Describe(nil)
	require.Error(t, err)

	server.Stop()

}

func TestControllerFree(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	smallSpec := types.Spec{Metadata: types.Metadata{Name: "small"}}
	smallObject := types.Object{Spec: smallSpec}

	smallActual := make(chan []interface{}, 1)

	small := &testing_controller.Controller{
		DoFree: func(search *types.Metadata) ([]types.Object, error) {
			if search == nil {
				return nil, fmt.Errorf("boom")
			}
			smallActual <- []interface{}{*search}

			return []types.Object{smallObject}, nil
		},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, Server(small))
	require.NoError(t, err)

	smallSearch := (types.Metadata{Name: "small"}).AddTagsFromStringSlice([]string{"a=b", "c=d"})

	a1, err := must(NewClient(plugin.Name(name), socketPath)).Free(&smallSearch)
	require.NoError(t, err)

	_, err = must(NewClient(plugin.Name("unknown"), socketPath)).Free(&smallSearch)
	require.Error(t, err)

	smallArgs := <-smallActual

	require.EqualValues(t, smallSearch, smallArgs[0])
	require.Equal(t, []types.Object{smallObject}, a1)

	// now return error
	_, err = must(NewClient(plugin.Name(name), socketPath)).Free(nil)
	require.Error(t, err)

	server.Stop()

}
