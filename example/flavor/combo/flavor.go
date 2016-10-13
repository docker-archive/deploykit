package main

import (
	"encoding/json"
	"github.com/docker/infrakit/plugin/group"
	"github.com/docker/infrakit/plugin/group/types"
	"github.com/docker/infrakit/spi/flavor"
	"github.com/docker/infrakit/spi/instance"
)

// Spec is the model of the plugin Properties.
type Spec struct {
	Flavors []types.FlavorPlugin
}

// NewPlugin creates a Flavor Combo plugin that chains multiple flavors in a sequence.  Each flavor
func NewPlugin(flavorPlugins group.FlavorPluginLookup) flavor.Plugin {
	return flavorCombo{flavorPlugins: flavorPlugins}
}

type flavorCombo struct {
	flavorPlugins group.FlavorPluginLookup
}

func (f flavorCombo) Validate(flavorProperties json.RawMessage, allocation types.AllocationMethod) error {
	s := Spec{}
	return json.Unmarshal(flavorProperties, &s)
}

func (f flavorCombo) Healthy(inst instance.Description) (bool, error) {
	return true, nil
}

func (f flavorCombo) Prepare(
	flavor json.RawMessage,
	instance instance.Spec,
	allocation types.AllocationMethod) (instance.Spec, error) {

	s := Spec{}
	err := json.Unmarshal(flavor, &s)
	if err != nil {
		return instance, err
	}

	for _, pluginSpec := range s.Flavors {
		plugin, err := f.flavorPlugins(pluginSpec.Plugin)
		if err != nil {
			return instance, err
		}

		var props json.RawMessage
		if pluginSpec.Properties != nil {
			props = *pluginSpec.Properties
		}

		instance, err = plugin.Prepare(props, instance, allocation)
		if err != nil {
			return instance, err
		}
	}

	return instance, nil
}
