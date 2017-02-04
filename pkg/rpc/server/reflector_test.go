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

func TestPrintFunc(t *testing.T) {

	s := printFunc("null", nil)
	require.Equal(t, "no-function", s)

	s = printFunc("string", "string")
	require.Equal(t, "not-a-function", s)

	s = printFunc("concat", func(a, b string) (string, error) { return "", nil })
	require.Equal(t, "func(string, string) (string, error)", s)

	s = printUsage("concat", func(a, b, c string) (string, error) { return "", nil })
	require.Equal(t, `{{ concat "string" "string" "string" }}`, s)

	s = printUsage("somefun", func(a, b, c string, d int) (string, error) { return "", nil })
	require.Equal(t, `{{ somefun "string" "string" "string" int }}`, s)

	s = printUsage("somefun", func(a, b, c string, d int, e ...bool) (string, error) { return "", nil })
	require.Equal(t, `{{ somefun "string" "string" "string" int [ bool ... ] }}`, s)

	s = printUsage("myfunc", func(a, b interface{}) (interface{}, error) { return "", nil })
	require.Equal(t, `{{ myfunc any any }}`, s)

	s = printUsage("myfunc", func(a []string, b map[string]interface{}) (interface{}, error) { return "", nil })
	require.Equal(t, `{{ myfunc []string map[string]interface{} }}`, s)
}

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
