package resource

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/docker/infrakit/pkg/discovery"
	plugin_base "github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/group/util"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/template"
	testing_instance "github.com/docker/infrakit/pkg/testing/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func newTestInstancePlugin() *testing_instance.Plugin {
	idPrefix := util.RandomAlphaNumericString(4)
	nextID := 0
	instances := map[instance.ID]instance.Spec{}

	return &testing_instance.Plugin{
		DoValidate: func(req *types.Any) error {
			return nil
		},
		DoProvision: func(spec instance.Spec) (*instance.ID, error) {
			id := instance.ID(fmt.Sprintf("%s-%d", idPrefix, nextID))
			nextID++
			instances[id] = spec
			return &id, nil
		},
		DoDestroy: func(id instance.ID) error {
			if _, ok := instances[id]; !ok {
				return errors.New("Instance does not exist")
			}
			delete(instances, id)
			return nil
		},
		DoDescribeInstances: func(tags map[string]string) ([]instance.Description, error) {
			descriptions := []instance.Description{}
		Loop:
			for id, inst := range instances {
				for k, v := range tags {
					if v2, ok := inst.Tags[k]; !ok || v2 != v {
						continue Loop
					}
				}
				descriptions = append(descriptions, instance.Description{
					ID:        id,
					LogicalID: inst.LogicalID,
					Tags:      inst.Tags,
				})
			}
			return descriptions, nil
		},
	}
}

const configID = "config"
const resourcesJSON = `
{
  "Resources": [
    {"ID": "a", "Plugin": "pluginA", "Properties": "{{ resource ` + "`b`" + ` }}"},
    {"ID": "b", "Plugin": "pluginB", "Properties": ""}
  ]
}`

func TestCommitAndDestroy(t *testing.T) {
	instancePluginA := newTestInstancePlugin()
	instancePluginB := newTestInstancePlugin()

	p := NewResourcePlugin(func(name plugin_base.Name) (instance.Plugin, error) {
		switch name {
		case "pluginA":
			return instancePluginA, nil
		case "pluginB":
			return instancePluginB, nil
		}
		return nil, errors.New("not found")
	})

	spec := resource.Spec{
		ID:         configID,
		Properties: types.AnyString(resourcesJSON),
	}

	// Commit when pretend is true should create no resources.
	_, err := p.Commit(spec, true)
	require.NoError(t, err)

	descriptions, err := instancePluginA.DescribeInstances(nil)
	require.NoError(t, err)
	require.Len(t, descriptions, 0)

	descriptions, err = instancePluginB.DescribeInstances(nil)
	require.NoError(t, err)
	require.Len(t, descriptions, 0)

	// Commit when pretend is false should create resources.
	_, err = p.Commit(spec, false)
	require.NoError(t, err)

	descriptions, err = instancePluginA.DescribeInstances(nil)
	require.NoError(t, err)
	require.Len(t, descriptions, 1)

	require.NotEqual(t, "", descriptions[0].ID)
	require.Nil(t, descriptions[0].LogicalID)
	require.Equal(t, map[string]string{resourcesTag: configID, resourcesIDTag: "a"}, descriptions[0].Tags)

	descriptions, err = instancePluginB.DescribeInstances(nil)
	require.NoError(t, err)
	require.Len(t, descriptions, 1)

	require.NotEqual(t, "", descriptions[0].ID)
	require.Nil(t, descriptions[0].LogicalID)
	require.Equal(t, map[string]string{resourcesTag: configID, resourcesIDTag: "b"}, descriptions[0].Tags)

	// Commit with the same specification should create no additional resources.
	_, err = p.Commit(spec, false)
	require.NoError(t, err)

	descriptions, err = instancePluginA.DescribeInstances(nil)
	require.NoError(t, err)
	require.Len(t, descriptions, 1)

	descriptions, err = instancePluginB.DescribeInstances(nil)
	require.NoError(t, err)
	require.Len(t, descriptions, 1)

	// Destroy when pretend is true should detroy no resources.
	_, err = p.Destroy(spec, true)
	require.NoError(t, err)

	descriptions, err = instancePluginA.DescribeInstances(nil)
	require.NoError(t, err)
	require.Len(t, descriptions, 1)

	descriptions, err = instancePluginB.DescribeInstances(nil)
	require.NoError(t, err)
	require.Len(t, descriptions, 1)

	// Destroy when pretend is false should destroy resources.
	_, err = p.Destroy(spec, false)
	require.NoError(t, err)

	descriptions, err = instancePluginA.DescribeInstances(nil)
	require.NoError(t, err)
	require.Len(t, descriptions, 0)

	descriptions, err = instancePluginB.DescribeInstances(nil)
	require.NoError(t, err)
	require.Len(t, descriptions, 0)

	// Destroy with the same specification should succeed.
	_, err = p.Destroy(spec, false)
	require.NoError(t, err)
}

func TestDescribeResources(t *testing.T) {
	instancePlugin := newTestInstancePlugin()
	p := NewResourcePlugin(func(name plugin_base.Name) (instance.Plugin, error) {
		return instancePlugin, nil
	})

	aID, err := instancePlugin.Provision(instance.Spec{Tags: map[string]string{resourcesTag: configID, resourcesIDTag: "a"}})
	require.NoError(t, err)
	bID, err := instancePlugin.Provision(instance.Spec{Tags: map[string]string{resourcesTag: configID, resourcesIDTag: "b"}})
	require.NoError(t, err)

	spec := resource.Spec{
		ID:         configID,
		Properties: types.AnyString(resourcesJSON),
	}
	details, err := p.DescribeResources(spec)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("Found a (%s)\nFound b (%s)", string(*aID), string(*bID)), details)
}

func TestDescribe(t *testing.T) {
	instancePlugin := newTestInstancePlugin()

	aID, err := instancePlugin.Provision(instance.Spec{Tags: map[string]string{resourcesTag: configID, resourcesIDTag: "a"}})
	require.NoError(t, err)
	bID, err := instancePlugin.Provision(instance.Spec{Tags: map[string]string{resourcesTag: configID, resourcesIDTag: "b"}})
	require.NoError(t, err)
	cID, err := instancePlugin.Provision(instance.Spec{Tags: map[string]string{resourcesTag: configID, resourcesIDTag: "c"}})
	require.NoError(t, err)

	configs := map[string]resourceConfig{
		"a": {plugin: instancePlugin},
		"b": {plugin: instancePlugin},
		"c": {plugin: instancePlugin},
	}
	ids, err := describe(configID, configs)
	require.NoError(t, err)
	require.Equal(t, map[string]instance.ID{"a": *aID, "b": *bID, "c": *cID}, ids)

	configs = map[string]resourceConfig{
		"b": {plugin: instancePlugin},
		"c": {plugin: instancePlugin},
		"d": {plugin: instancePlugin},
	}
	ids, err = describe(configID, configs)
	require.NoError(t, err)
	require.Equal(t, map[string]instance.ID{"b": *bID, "c": *cID}, ids)

	ids, err = describe("returns no IDs given a different"+configID, configs)
	require.NoError(t, err)
	require.Equal(t, map[string]instance.ID{}, ids)

	// Error from instance.Plugin.DescribeInstances.
	errorPlugin := &testing_instance.Plugin{
		DoDescribeInstances: func(tags map[string]string) ([]instance.Description, error) {
			return nil, errors.New("kaboom")
		},
	}
	configs = map[string]resourceConfig{
		"a": {plugin: errorPlugin},
	}
	_, err = describe(configID, configs)
	require.Error(t, err)

	// Multiple resources with the same name.
	instanceSpec := instance.Spec{Tags: map[string]string{resourcesTag: configID, resourcesIDTag: "x"}}
	_, err = instancePlugin.Provision(instanceSpec)
	require.NoError(t, err)
	_, err = instancePlugin.Provision(instanceSpec)
	require.NoError(t, err)
	configs = map[string]resourceConfig{
		"x": {plugin: instancePlugin},
	}
	_, err = describe(configID, configs)
	require.Error(t, err)

}

func TestValidate(t *testing.T) {
	instancePlugin := &testing_instance.Plugin{
		DoValidate: func(req *types.Any) error { return nil },
	}
	lookup := func(name plugin_base.Name) (instance.Plugin, error) {
		if name == "p" {
			return instancePlugin, nil
		}
		return nil, errors.New("not found")
	}

	newResourceDotSpec := func(id, properties string) resource.Spec {
		var i interface{}
		if err := json.Unmarshal([]byte(properties), &i); err != nil {
			panic(err)
		}

		return resource.Spec{
			ID:         resource.ID(id),
			Properties: types.AnyString(properties),
		}
	}

	// Missing resource.Spec.ID.
	_, _, err := validate(newResourceDotSpec("", `{}`), lookup)
	require.Error(t, err)

	// Missing resource.Spec.Properties.
	_, _, err = validate(resource.Spec{ID: "id", Properties: nil}, lookup)
	require.Error(t, err)

	// Malformed resource.Spec.Properties.
	_, _, err = validate(resource.Spec{ID: "id", Properties: types.AnyString(`malformed JSON`)}, lookup)
	require.Error(t, err)

	// Missing resource ID.
	_, _, err = validate(newResourceDotSpec("id", `{"Resources": [{"Plugin": "p", "Properties": ""}]}`), lookup)
	require.Error(t, err)

	// Missing resource plugin.
	_, _, err = validate(newResourceDotSpec("id", `{"Resources": [{"ID": "a", "Properties": ""}]}`), lookup)
	require.Error(t, err)

	// Nonexistent resource plugin.
	_, _, err = validate(newResourceDotSpec("id", `{"Resources": [{"ID": "a", "Plugin": "nonexistent", "Properties": ""}]}`), lookup)
	require.Error(t, err)

	// Missing resource properties.
	_, _, err = validate(newResourceDotSpec("id", `{"Resources": [{"ID": "a", "Plugin": "p"}]}`), lookup)
	require.Error(t, err)

	// Malformed resource properties.
	properties := `{"Resources": [{"ID": "a", "Plugin": "p", "Properties": "{{/* malformed template"}]}`
	_, _, err = validate(newResourceDotSpec("id", properties), lookup)
	require.Error(t, err)

	configs, provisioningOrder, err := validate(newResourceDotSpec("id", `{}`), lookup)
	require.NoError(t, err)
	require.Equal(t, map[string]resourceConfig{}, configs)
	require.Len(t, provisioningOrder, 0)

	// Empty resource properties.
	properties = `{"Resources": [{"ID": "a", "Plugin": "p", "Properties": ""}]}`
	configs, provisioningOrder, err = validate(newResourceDotSpec("id", properties), lookup)
	require.NoError(t, err)
	expectedConfigs := map[string]resourceConfig{
		"a": {plugin: instancePlugin, properties: `""`},
	}
	require.Equal(t, expectedConfigs, configs)
	require.Equal(t, []string{"a"}, provisioningOrder)

	properties = `{"Resources": [{"ID": "a", "Plugin": "p", "Properties": "{{ resource ` + "`b`" + ` }}"}, {"ID": "b", "Plugin": "p", "Properties": ""}]}`
	configs, provisioningOrder, err = validate(newResourceDotSpec("id", properties), lookup)
	require.NoError(t, err)
	expectedConfigs = map[string]resourceConfig{
		"a": {plugin: instancePlugin, properties: `"{{ resource ` + "`b`" + ` }}"`},
		"b": {plugin: instancePlugin, properties: `""`},
	}
	require.Equal(t, expectedConfigs, configs)
	require.Equal(t, []string{"b", "a"}, provisioningOrder)
}

func TestGetProvisioningOrder(t *testing.T) {
	_, err := getProvisioningOrder(map[string]resourceConfig{
		"a": {properties: `{{/* malformed template }}`},
	})
	require.Error(t, err)

	_, err = getProvisioningOrder(map[string]resourceConfig{
		"a": {properties: `{{ wellFormedTemplateWithExecutionError }}`},
	})
	require.Error(t, err)

	_, err = getProvisioningOrder(map[string]resourceConfig{
		"a": {properties: `{{ resource "nonexistent" }}`},
	})
	require.Error(t, err)

	_, err = getProvisioningOrder(map[string]resourceConfig{
		"a": {properties: `{{ resource "a" }}`},
	})
	require.Error(t, err)

	_, err = getProvisioningOrder(map[string]resourceConfig{
		"a": {properties: `{{ resource "b" }}`},
		"b": {properties: `{{ resource "a" }}`},
	})
	require.Error(t, err)

	order, err := getProvisioningOrder(map[string]resourceConfig{})
	require.NoError(t, err)
	require.Equal(t, []string{}, order)

	order, err = getProvisioningOrder(map[string]resourceConfig{
		"a": {properties: `{{ resource "b" }}`},
		"b": {properties: ``},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"b", "a"}, order)

	order, err = getProvisioningOrder(map[string]resourceConfig{
		"a": {properties: `{{ resource "b" }}`},
		"b": {properties: `{{ resource "c" }} {{ resource "d" }}`},
		"c": {properties: `{{ resource "e" }}`},
		"d": {properties: `{{ resource "e" }}`},
		"e": {properties: `{{ resource "f" }}`},
		"f": {properties: ``},
	})
	require.NoError(t, err)
	require.Condition(t, func() bool {
		return (reflect.DeepEqual(order, []string{"f", "e", "d", "c", "b", "a"}) ||
			reflect.DeepEqual(order, []string{"f", "e", "c", "d", "b", "a"}))

	})
}

func TestGetResourceDependencies(t *testing.T) {
	newTemplate := func(s string) *template.Template {
		t, err := template.NewTemplate("str://"+s, template.Options{SocketDir: discovery.Dir()})
		if err != nil {
			panic(err)
		}
		return t
	}

	_, err := getResourceDependencies(newTemplate(`{{ wellFormedTemplateWithExecutionError }}`))
	require.Error(t, err)

	deps, err := getResourceDependencies(newTemplate(``))
	require.NoError(t, err)
	require.Len(t, deps, 0)

	deps, err = getResourceDependencies(newTemplate(`{{ resource "a" }}`))
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"a": true}, deps)

	deps, err = getResourceDependencies(newTemplate(`{{ resource "a" }} {{ resource "a" }}`))
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"a": true}, deps)

	deps, err = getResourceDependencies(newTemplate(`{{ resource "a" }} {{ resource "b" }} {{ resource "c" }}`))
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"a": true, "b": true, "c": true}, deps)
}
