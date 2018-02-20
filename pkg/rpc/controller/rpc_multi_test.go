package controller

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/controller"
	testing_controller "github.com/docker/infrakit/pkg/testing/controller"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return filepath.Join(dir, "controller-impl-test")
}

func must(i controller.Controller, err error) controller.Controller {
	if err != nil {
		panic(err)
	}
	return i
}

func TestMultiControllerPlan(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	smallSpec := types.Spec{Metadata: types.Metadata{Name: "small"}}
	smallObject := types.Object{Spec: smallSpec}
	smallPlan := controller.Plan{Message: []string{"increase size"}}
	largeSpec := types.Spec{Metadata: types.Metadata{Name: "large"}}
	largeObject := types.Object{Spec: largeSpec}
	largePlan := controller.Plan{Message: []string{"decrease size"}}

	smallActual := make(chan []interface{}, 1)
	largeActual := make(chan []interface{}, 1)

	small := &testing_controller.Controller{
		DoPlan: func(operation controller.Operation, spec types.Spec) (types.Object, controller.Plan, error) {

			smallActual <- []interface{}{operation, spec}

			return smallObject, smallPlan, nil
		},
	}
	large := &testing_controller.Controller{
		DoPlan: func(operation controller.Operation, spec types.Spec) (types.Object, controller.Plan, error) {

			largeActual <- []interface{}{operation, spec}

			return largeObject, largePlan, nil
		},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, ServerWithNames(
		func() (map[string]controller.Controller, error) {
			return map[string]controller.Controller{
				"small": small,
				"large": large,
			}, nil
		},
	))
	require.NoError(t, err)

	a1, p1, err := must(NewClient(plugin.Name(name+"/small"), socketPath)).Plan(controller.Enforce, smallSpec)
	require.NoError(t, err)

	a2, p2, err := must(NewClient(plugin.Name(name+"/large"), socketPath)).Plan(controller.Destroy, largeSpec)
	require.NoError(t, err)

	_, _, err = must(NewClient(plugin.Name(name+"/typeUnknown"), socketPath)).Plan(controller.Enforce, largeSpec)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "controller-impl-test/typeUnknown"))

	smallArgs := <-smallActual
	largeArgs := <-largeActual

	require.EqualValues(t, controller.Enforce, smallArgs[0].(controller.Operation))
	require.Equal(t, smallSpec, smallArgs[1])
	require.Equal(t, smallObject, a1)
	require.Equal(t, smallPlan, p1)

	require.EqualValues(t, controller.Destroy, largeArgs[0].(controller.Operation))
	require.Equal(t, largeSpec, largeArgs[1])
	require.Equal(t, largeObject, a2)
	require.Equal(t, largePlan, p2)

	// now return error
	small.DoPlan = func(operation controller.Operation, spec types.Spec) (types.Object, controller.Plan, error) {
		return largeObject, largePlan, fmt.Errorf("boom")
	}

	_, _, err = must(NewClient(plugin.Name(name+"/small"), socketPath)).Plan(controller.Enforce, smallSpec)
	require.Error(t, err)

	server.Stop()

}

func TestMultiControllerCommit(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	smallSpec := types.Spec{Metadata: types.Metadata{Name: "small"}}
	smallObject := types.Object{Spec: smallSpec}

	largeSpec := types.Spec{Metadata: types.Metadata{Name: "large"}}
	largeObject := types.Object{Spec: largeSpec}

	smallActual := make(chan []interface{}, 1)
	largeActual := make(chan []interface{}, 1)

	small := &testing_controller.Controller{
		DoCommit: func(operation controller.Operation, spec types.Spec) (types.Object, error) {

			smallActual <- []interface{}{operation, spec}

			return smallObject, nil
		},
	}
	large := &testing_controller.Controller{
		DoCommit: func(operation controller.Operation, spec types.Spec) (types.Object, error) {

			largeActual <- []interface{}{operation, spec}

			return largeObject, nil
		},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, ServerWithNames(
		func() (map[string]controller.Controller, error) {
			return map[string]controller.Controller{
				"small": small,
				"large": large,
			}, nil
		},
	))
	require.NoError(t, err)

	a1, err := must(NewClient(plugin.Name(name+"/small"), socketPath)).Commit(controller.Enforce, smallSpec)
	require.NoError(t, err)

	a2, err := must(NewClient(plugin.Name(name+"/large"), socketPath)).Commit(controller.Destroy, largeSpec)
	require.NoError(t, err)

	_, err = must(NewClient(plugin.Name(name+"/typeUnknown"), socketPath)).Commit(controller.Enforce, largeSpec)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "controller-impl-test/typeUnknown"))

	smallArgs := <-smallActual
	largeArgs := <-largeActual

	require.EqualValues(t, controller.Enforce, smallArgs[0].(controller.Operation))
	require.Equal(t, smallSpec, smallArgs[1])
	require.Equal(t, smallObject, a1)

	require.EqualValues(t, controller.Destroy, largeArgs[0].(controller.Operation))
	require.Equal(t, largeSpec, largeArgs[1])
	require.Equal(t, largeObject, a2)

	// now return error
	small.DoCommit = func(operation controller.Operation, spec types.Spec) (types.Object, error) {
		return largeObject, fmt.Errorf("boom")
	}

	_, err = must(NewClient(plugin.Name(name+"/small"), socketPath)).Commit(controller.Enforce, smallSpec)
	require.Error(t, err)

	server.Stop()

}

func TestMultiControllerDescribe(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	smallSpec := types.Spec{Metadata: types.Metadata{Name: "small"}}
	smallObject := types.Object{Spec: smallSpec}

	largeSpec := types.Spec{Metadata: types.Metadata{Name: "large"}}
	largeObject := types.Object{Spec: largeSpec}

	smallActual := make(chan []interface{}, 1)
	largeActual := make(chan []interface{}, 1)

	small := &testing_controller.Controller{
		DoDescribe: func(search *types.Metadata) ([]types.Object, error) {

			smallActual <- []interface{}{*search}

			return []types.Object{smallObject}, nil
		},
	}
	large := &testing_controller.Controller{
		DoDescribe: func(search *types.Metadata) ([]types.Object, error) {

			largeActual <- []interface{}{*search}

			return []types.Object{largeObject}, nil
		},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, ServerWithNames(
		func() (map[string]controller.Controller, error) {
			return map[string]controller.Controller{
				"small": small,
				"large": large,
			}, nil
		},
	))
	require.NoError(t, err)

	smallSearch := (types.Metadata{Name: "small"}).AddTagsFromStringSlice([]string{"a=b", "c=d"})
	largeSearch := types.Metadata{Name: "large"}

	a1, err := must(NewClient(plugin.Name(name+"/small"), socketPath)).Describe(&smallSearch)
	require.NoError(t, err)

	a2, err := must(NewClient(plugin.Name(name+"/large"), socketPath)).Describe(&largeSearch)
	require.NoError(t, err)

	_, err = must(NewClient(plugin.Name(name+"/typeUnknown"), socketPath)).Describe(nil)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "controller-impl-test/typeUnknown"))

	smallArgs := <-smallActual
	largeArgs := <-largeActual

	require.EqualValues(t, smallSearch, smallArgs[0])
	require.Equal(t, []types.Object{smallObject}, a1)

	require.EqualValues(t, largeSearch, largeArgs[0])
	require.Equal(t, []types.Object{largeObject}, a2)

	// now return error
	small.DoDescribe = func(search *types.Metadata) ([]types.Object, error) {
		require.Nil(t, search)
		return nil, fmt.Errorf("boom")
	}
	_, err = must(NewClient(plugin.Name(name+"/small"), socketPath)).Describe(nil)
	require.Error(t, err)

	server.Stop()

}

func TestMultiControllerFree(t *testing.T) {
	socketPath := tempSocket()
	name := filepath.Base(socketPath)

	smallSpec := types.Spec{Metadata: types.Metadata{Name: "small"}}
	smallObject := types.Object{Spec: smallSpec}

	largeSpec := types.Spec{Metadata: types.Metadata{Name: "large"}}
	largeObject := types.Object{Spec: largeSpec}

	smallActual := make(chan []interface{}, 1)
	largeActual := make(chan []interface{}, 1)

	small := &testing_controller.Controller{
		DoFree: func(search *types.Metadata) ([]types.Object, error) {

			smallActual <- []interface{}{*search}

			return []types.Object{smallObject}, nil
		},
	}
	large := &testing_controller.Controller{
		DoFree: func(search *types.Metadata) ([]types.Object, error) {

			largeActual <- []interface{}{*search}

			return []types.Object{largeObject}, nil
		},
	}
	server, err := rpc_server.StartPluginAtPath(socketPath, ServerWithNames(
		func() (map[string]controller.Controller, error) {
			return map[string]controller.Controller{
				"small": small,
				"large": large,
			}, nil
		},
	))
	require.NoError(t, err)

	smallSearch := (types.Metadata{Name: "small"}).AddTagsFromStringSlice([]string{"a=b", "c=d"})
	largeSearch := types.Metadata{Name: "large"}

	a1, err := must(NewClient(plugin.Name(name+"/small"), socketPath)).Free(&smallSearch)
	require.NoError(t, err)

	a2, err := must(NewClient(plugin.Name(name+"/large"), socketPath)).Free(&largeSearch)
	require.NoError(t, err)

	_, err = must(NewClient(plugin.Name(name+"/typeUnknown"), socketPath)).Free(nil)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "controller-impl-test/typeUnknown"))

	smallArgs := <-smallActual
	largeArgs := <-largeActual

	require.EqualValues(t, smallSearch, smallArgs[0])
	require.Equal(t, []types.Object{smallObject}, a1)

	require.EqualValues(t, largeSearch, largeArgs[0])
	require.Equal(t, []types.Object{largeObject}, a2)

	// now return error
	small.DoFree = func(search *types.Metadata) ([]types.Object, error) {
		require.Nil(t, search)
		return nil, fmt.Errorf("boom")
	}
	_, err = must(NewClient(plugin.Name(name+"/small"), socketPath)).Free(nil)
	require.Error(t, err)

	server.Stop()

}
