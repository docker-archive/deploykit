package server

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	plugin_mock "github.com/docker/infrakit/pkg/mock/spi/instance"
	plugin_rpc "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestReflect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := plugin_mock.NewMockPlugin(ctrl)
	service := plugin_rpc.PluginServer(mock)
	r := &reflector{target: service}

	tt := r.targetType()
	require.Equal(t, reflect.TypeOf(plugin_rpc.Instance{}), tt)

	tver2 := r.Interface()
	require.Equal(t, instance.InterfaceSpec, tver2)

	methods := r.pluginMethods()
	require.Equal(t, 5, len(methods))

	// get method names
	names := []string{}
	for _, m := range methods {
		names = append(names, m.Name)
	}

	expect := []string{
		"Validate",
		"Provision",
		"Label",
		"Destroy",
		"DescribeInstances",
	}
	sort.Strings(expect)
	sort.Strings(names)
	require.Equal(t, expect, names)

	// find the Provision method to test
	f := func() reflect.Method {
		for _, m := range methods {
			if m.Name == "Validate" {
				return m
			}
		}
		return reflect.Method{}
	}()

	desc := r.toDescription(f)

	_, err := json.MarshalIndent(desc, "  ", "  ")
	require.NoError(t, err)
}

func toRaw(t *testing.T, v interface{}) *types.Any {
	any, err := types.AnyValue(v)
	require.NoError(t, err)
	return any
}
