package resource

import (
	"testing"

	resource "github.com/docker/infrakit/pkg/controller/resource/types"
	"github.com/docker/infrakit/pkg/fsm"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func testOptions(t *testing.T) resource.Options {
	buff := []byte(`
kind: resource
metadata:
  name: resources
options:
  WaitBeforeProvision: 100
properties:
`)
	var spec types.Spec
	err := types.Decode(buff, &spec)
	require.NoError(t, err)

	options := DefaultOptions
	err = spec.Options.Decode(&options)
	require.NoError(t, err)
	return options

}

func testProperties(t *testing.T) resource.Properties {
	buff := []byte(`
kind: resource
metadata:
  name: resources
options:
  WaitBeforeProvision: 100
properties:
  az1-net1:
    plugin: az1/net
    select:
      az: az1
      type: network
    ObserveInterval: 1s
    KeySelector: \{\{.Tags.infrakit_resource_name\}\}
    Properties:
      cidr: 10.20.100.0/24
      gateway: 10.20.0.1
  az1-net2:
    plugin: az1/net
    select:
      az: az1
      type: network
    ObserveInterval: 1s
    KeySelector: \{\{.Tags.infrakit_resource_name\}\}
    Properties:
      wait: "@depend('az1-net1/ID')@"
      cidr: 10.20.200.0/24
      gateway: 10.20.0.1
  az2-net1:
    plugin: az2/net
    select:
      az: az2
      type: network
    ObserveInterval: 1s
    KeySelector: \{\{.Tags.infrakit_resource_name\}\}
    Properties:
      wait: "@depend('az1-net1/ID')@"
      cidr: 192.178.100.0/24
      gateway: 192.178.0.1
  az2-net2:
    plugin: az2/net
    select:
      az: az2
      type: network
    ObserveInterval: 1s
    KeySelector: \{\{.Tags.infrakit_resource_name\}\}
    Properties:
      wait: "@depend('az1-net1/ID')@"
      cidr: 192.178.200.0/24
      gateway: 192.178.0.1
  az1-disk1:
    plugin: az1/disk
    select:
      az: az1
      type: storage
    ObserveInterval: 0.5s
    KeySelector: \{\{.Tags.infrakit_resource_name\}\}
    Properties:
      net: "@depend('az1-net1/ID')@"
      gw: "@depend('az1-net1/Properties/gateway')@"
      fs: ext4
      size: 1TB
  az1-disk2:
    plugin: az1/disk
    select:
      az: az1
      type: storage
    ObserveInterval: 0.5s
    KeySelector: \{\{.Tags.infrakit_resource_name\}\}
    Properties:
      net: "@depend('az1-net2/ID')@"
      gw: "@depend('az1-net2/Properties/gateway')@"
      fs: ext4
      size: 1TB
  az2-disk1:
    plugin: az2/disk
    select:
      az: az2
      type: storage
    ObserveInterval: 0.5s
    KeySelector: \{\{.Tags.infrakit_resource_name\}\}
    Properties:
      net: "@depend('az2-net1/ID')@"
      gw: "@depend('az2-net1/Properties/gateway')@"
      fs: ext4
      size: 1TB
  az2-disk1:
    plugin: az2/disk
    select:
      az: az2
      type: storage
    ObserveInterval: 0.5s
    KeySelector: \{\{.Tags.infrakit_resource_name\}\}
    Properties:
      net: "@depend('az2-net2/ID')@"
      gw: "@depend('az2-net2/Properties/gateway')@"
      fs: ext4
      size: 1TB
`)

	var spec types.Spec
	err := types.Decode(buff, &spec)
	require.NoError(t, err)

	properties := DefaultProperties
	err = spec.Properties.Decode(&properties)
	require.NoError(t, err)
	return properties
}

func TestModel(t *testing.T) {

	model, err := BuildModel(testProperties(t), testOptions(t))
	require.NoError(t, err)

	model.Start()

	total := 100
	list := []fsm.FSM{}
	for i := 0; i < total; i++ {
		list = append(list, model.Requested())
	}

	model.Stop()
}
