package server

import (
	"encoding/json"
	"testing"

	flavor_plugin_mock "github.com/docker/infrakit/pkg/mock/spi/flavor"
	group_plugin_mock "github.com/docker/infrakit/pkg/mock/spi/group"
	instance_plugin_mock "github.com/docker/infrakit/pkg/mock/spi/instance"
	group_plugin_types "github.com/docker/infrakit/pkg/plugin/group/types"
	flavor_plugin_rpc "github.com/docker/infrakit/pkg/rpc/flavor"
	group_plugin_rpc "github.com/docker/infrakit/pkg/rpc/group"
	instance_plugin_rpc "github.com/docker/infrakit/pkg/rpc/instance"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type infoWrapper plugin.Info

func (f infoWrapper) VendorInfo() *spi.VendorInfo {
	return plugin.Info(f).Vendor
}

type inputExampleWrapper struct {
	value interface{}
}

func (i inputExampleWrapper) ExampleProperties() *json.RawMessage {
	buff, err := json.MarshalIndent(i.value, "  ", "  ")
	if err != nil {
		panic(err)
	}
	raw := json.RawMessage(buff)
	return &raw
}

func TestMetaForInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	vendorName := "instance-plugin-by-mocking-corp"
	vendorVersion := "1.0"
	example := struct {
		Type  string
		IP    string
		Zone  string
		MemGb int
	}{
		Type:  "large",
		IP:    "10.0.1.100",
		Zone:  "a",
		MemGb: 512,
	}
	mock := struct {
		instance.Plugin
		spi.Vendor
		spi.InputExample
	}{
		Plugin: instance_plugin_mock.NewMockPlugin(ctrl),
		Vendor: infoWrapper(plugin.Info{
			Vendor: &spi.VendorInfo{
				InterfaceSpec: spi.InterfaceSpec{
					Name:    vendorName,
					Version: vendorVersion,
				},
			},
		}),
		InputExample: inputExampleWrapper{value: example},
	}

	require.NotNil(t, mock.ExampleProperties())

	service := instance_plugin_rpc.PluginServer(mock)

	md, err := NewPluginInfo(service)
	require.NoError(t, err)

	meta := md.getInfo()
	require.Equal(t, []spi.InterfaceSpec{instance.InterfaceSpec}, meta.Implements)

	require.Equal(t, vendorName, meta.Vendor.Name)
	require.Equal(t, vendorVersion, meta.Vendor.Version)
	require.Equal(t, 1, len(meta.Interfaces))
	require.Equal(t, 4, len(meta.Interfaces[0].Methods))
	require.Equal(t, instance.InterfaceSpec, meta.Interfaces[0].InterfaceSpec)

	buff, err := json.MarshalIndent(meta, "  ", "  ")
	require.NoError(t, err)

	t.Log(string(buff))
}

func TestMetaForFlavor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	vendorName := "flavor-plugin-by-mocking-corp"
	vendorVersion := "1.0"
	example := struct {
		SwarmWorker bool
	}{
		SwarmWorker: true,
	}

	mock := struct {
		flavor.Plugin
		spi.Vendor
		spi.InputExample
	}{
		Plugin: flavor_plugin_mock.NewMockPlugin(ctrl),
		Vendor: infoWrapper(plugin.Info{
			Vendor: &spi.VendorInfo{
				InterfaceSpec: spi.InterfaceSpec{
					Name:    vendorName,
					Version: vendorVersion,
				},
			},
		}),
		InputExample: inputExampleWrapper{value: example},
	}
	service := flavor_plugin_rpc.PluginServer(mock)

	md, err := NewPluginInfo(service)
	require.NoError(t, err)

	meta := md.getInfo()
	require.Equal(t, []spi.InterfaceSpec{flavor.InterfaceSpec}, meta.Implements)

	require.Equal(t, vendorName, meta.Vendor.Name)
	require.Equal(t, vendorVersion, meta.Vendor.Version)
	require.Equal(t, 1, len(meta.Interfaces))
	require.Equal(t, 4, len(meta.Interfaces[0].Methods))
	require.Equal(t, flavor.InterfaceSpec, meta.Interfaces[0].InterfaceSpec)

	buff, err := json.MarshalIndent(meta, "  ", "  ")
	require.NoError(t, err)
	t.Log(string(buff))
}

func TestMetaForGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	vendorName := "group-plugin-by-mocking-corp"
	vendorVersion := "1.0"

	mock := struct {
		group.Plugin
		spi.Vendor
		spi.InputExample
	}{
		Plugin: group_plugin_mock.NewMockPlugin(ctrl),
		Vendor: infoWrapper(plugin.Info{
			Vendor: &spi.VendorInfo{
				InterfaceSpec: spi.InterfaceSpec{
					Name:    vendorName,
					Version: vendorVersion,
				},
			},
		}),
		InputExample: inputExampleWrapper{value: group_plugin_types.Spec{}},
	}
	service := group_plugin_rpc.PluginServer(mock)

	md, err := NewPluginInfo(service)
	require.NoError(t, err)

	meta := md.getInfo()
	require.Equal(t, []spi.InterfaceSpec{group.InterfaceSpec}, meta.Implements)

	require.Equal(t, vendorName, meta.Vendor.Name)
	require.Equal(t, vendorVersion, meta.Vendor.Version)
	require.Equal(t, 1, len(meta.Interfaces))
	require.Equal(t, 5, len(meta.Interfaces[0].Methods))
	require.Equal(t, group.InterfaceSpec, meta.Interfaces[0].InterfaceSpec)

	buff, err := json.MarshalIndent(meta, "  ", "  ")
	require.NoError(t, err)

	t.Log(string(buff))
}
