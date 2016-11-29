package server

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	plugin_mock "github.com/docker/infrakit/pkg/mock/spi/instance"
	"github.com/docker/infrakit/pkg/plugin"
	plugin_rpc "github.com/docker/infrakit/pkg/rpc/instance"
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

	tver2, err := r.TypeVersion()
	require.NoError(t, err)
	require.Equal(t, plugin.TypeVersion("Instance/"+plugin.CurrentVersion), tver2)

	methods := r.pluginMethods()
	require.Equal(t, 4, len(methods))

	// get method names
	names := []string{}
	for _, m := range methods {
		names = append(names, m.Name)
	}

	expect := []string{
		"Validate",
		"Provision",
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

	_, err = json.MarshalIndent(desc, "  ", "  ")
	require.NoError(t, err)
}

func toRaw(t *testing.T, v interface{}) *json.RawMessage {
	buff, err := json.MarshalIndent(v, "  ", "  ")
	require.NoError(t, err)
	raw := json.RawMessage(buff)
	return &raw
}

func TestSetProperties(t *testing.T) {
	type foo struct {
		Properties *json.RawMessage
	}
	v := struct {
		Properties *json.RawMessage
		Foo        *foo
		Nested     struct {
			Properties *json.RawMessage
			Nested     *foo
		}
	}{}

	custom := map[string]interface{}{
		"test": "value",
		"bool": true,
	}

	raw := toRaw(t, custom)
	setFieldValue("Properties", reflect.ValueOf(&v), raw, true)

	// All the Properties field of type *json.RawMessage should be set.
	require.Equal(t, *raw, *v.Properties)
	require.Equal(t, *raw, *v.Foo.Properties)
	require.Equal(t, *raw, *v.Nested.Properties)
	require.Equal(t, *raw, *v.Nested.Nested.Properties)

	_, err := json.MarshalIndent(v, "  ", "  ")
	require.NoError(t, err)
}
