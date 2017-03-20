package resource

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"

	plugin_base "github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/group/util"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/resource"
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

	properties := types.AnyString(
		`{"Resources": {"a": {"Plugin": "pluginA", "Properties": "{{ resource ` + "`b`" + ` }}"}, "b": {"Plugin": "pluginB", "Properties": ""}}}`)

	const configID = "config"
	spec := resource.Spec{
		ID:         configID,
		Properties: properties,
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
	require.Equal(t, map[string]string{resourceGroupTag: configID, resourceNameTag: "a"}, descriptions[0].Tags)

	descriptions, err = instancePluginB.DescribeInstances(nil)
	require.NoError(t, err)
	require.Len(t, descriptions, 1)

	require.NotEqual(t, "", descriptions[0].ID)
	require.Nil(t, descriptions[0].LogicalID)
	require.Equal(t, map[string]string{resourceGroupTag: configID, resourceNameTag: "b"}, descriptions[0].Tags)

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
	const configID = "config"
	instancePlugin := newTestInstancePlugin()
	p := NewResourcePlugin(func(name plugin_base.Name) (instance.Plugin, error) {
		return instancePlugin, nil
	})

	aID, err := instancePlugin.Provision(instance.Spec{Tags: map[string]string{resourceGroupTag: configID, resourceNameTag: "a"}})
	require.NoError(t, err)
	bID, err := instancePlugin.Provision(instance.Spec{Tags: map[string]string{resourceGroupTag: configID, resourceNameTag: "b"}})
	require.NoError(t, err)

	properties := types.AnyString(`{"Resources": {"a": {"Plugin": "p", "Properties": ""}, "b": {"Plugin": "p", "Properties": ""}}}`)
	spec := resource.Spec{
		ID:         configID,
		Properties: properties,
	}
	details, err := p.DescribeResources(spec)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("Found a (%s)\nFound b (%s)", string(*aID), string(*bID)), details)
}

func TestDescribe(t *testing.T) {
	const configID = "config"
	instancePlugin := newTestInstancePlugin()

	aID, err := instancePlugin.Provision(instance.Spec{Tags: map[string]string{resourceGroupTag: configID, resourceNameTag: "a"}})
	require.NoError(t, err)
	bID, err := instancePlugin.Provision(instance.Spec{Tags: map[string]string{resourceGroupTag: configID, resourceNameTag: "b"}})
	require.NoError(t, err)
	cID, err := instancePlugin.Provision(instance.Spec{Tags: map[string]string{resourceGroupTag: configID, resourceNameTag: "c"}})
	require.NoError(t, err)

	abcSpec := Spec{Resources: map[string]resourceSpec{
		"a": {plugin: instancePlugin},
		"b": {plugin: instancePlugin},
		"c": {plugin: instancePlugin},
	}}
	ids, err := describe(configID, abcSpec)
	require.NoError(t, err)
	require.Equal(t, map[string]instance.ID{"a": *aID, "b": *bID, "c": *cID}, ids)

	bcdSpec := Spec{Resources: map[string]resourceSpec{
		"b": {plugin: instancePlugin},
		"c": {plugin: instancePlugin},
		"d": {plugin: instancePlugin},
	}}
	ids, err = describe(configID, bcdSpec)
	require.NoError(t, err)
	require.Equal(t, map[string]instance.ID{"b": *bID, "c": *cID}, ids)

	ids, err = describe("returns no IDs given a different"+configID, bcdSpec)
	require.NoError(t, err)
	require.Equal(t, map[string]instance.ID{}, ids)

	// Error from instance.Plugin.DescribeInstances.
	errorPlugin := &testing_instance.Plugin{
		DoDescribeInstances: func(tags map[string]string) ([]instance.Description, error) {
			return nil, errors.New("kaboom")
		},
	}
	errorSpec := Spec{Resources: map[string]resourceSpec{
		"a": {plugin: errorPlugin},
	}}
	_, err = describe(configID, errorSpec)
	require.Error(t, err)

	// Multiple resources with the same name.
	_, err = instancePlugin.Provision(instance.Spec{Tags: map[string]string{resourceGroupTag: configID, resourceNameTag: "a"}})
	require.NoError(t, err)
	_, err = describe(configID, abcSpec)
	require.Error(t, err)

}

func TestValidate(t *testing.T) {
	instancePlugin := &testing_instance.Plugin{
		DoValidate: func(req *types.Any) error { return nil },
	}
	pluginLookup := func(name plugin_base.Name) (instance.Plugin, error) {
		if name == "p" {
			return instancePlugin, nil
		}
		return nil, errors.New("not found")
	}

	newResourceDotSpec := func(id, properties string) resource.Spec {
		return resource.Spec{
			ID:         resource.ID(id),
			Properties: types.AnyString(properties),
		}
	}

	// Missing resource.Spec.ID.
	_, _, err := validate(newResourceDotSpec("", ``), pluginLookup)
	require.Error(t, err)

	// Missing resource.Spec.Properties.
	_, _, err = validate(resource.Spec{ID: "id", Properties: nil}, pluginLookup)
	require.Error(t, err)

	// Malformed resource.Spec.Properties.
	_, _, err = validate(newResourceDotSpec("id", `malformed JSON`), pluginLookup)
	require.Error(t, err)

	// Missing resource plugin.
	_, _, err = validate(newResourceDotSpec("id", `{"Resources": {"a": {"Properties": ""}}}`), pluginLookup)
	require.Error(t, err)

	// Nonexistent resource plugin.
	_, _, err = validate(newResourceDotSpec("id", `{"Resources": {"a": {"Plugin": "nonexistent plugin", "Properties": ""}}}`), pluginLookup)
	require.Error(t, err)

	// Missing resource properties.
	_, _, err = validate(newResourceDotSpec("id", `{"Resources": {"a": {"Plugin": "p"}}}`), pluginLookup)
	require.Error(t, err)

	// Malformed resource properties.
	properties := `{"Resources": {"a": {"Plugin": "p", "Properties": "{{/* malformed template"}}}`
	_, _, err = validate(newResourceDotSpec("id", properties), pluginLookup)
	require.Error(t, err)

	// Empty resource.Spec.Properties.
	spec, provisioningOrder, err := validate(newResourceDotSpec("id", ``), pluginLookup)
	require.NoError(t, err)
	require.Equal(t, &Spec{}, spec)
	require.Len(t, provisioningOrder, 0)

	// Empty resource properties.
	spec, provisioningOrder, err = validate(newResourceDotSpec("id", `{"Resources": {"a": {"Plugin": "p", "Properties": ""}}}`), pluginLookup)
	require.NoError(t, err)
	expectedSpec := &Spec{
		Resources: map[string]resourceSpec{"a": {Plugin: "p", Properties: types.AnyString(`""`), plugin: instancePlugin}}}
	require.Equal(t, expectedSpec, spec)
	require.Equal(t, []string{"a"}, provisioningOrder)

	properties = `{"Resources": {"a": {"Plugin": "p", "Properties": "{{ resource ` + "`b`" + ` }}"}, "b": {"Plugin": "p", "Properties": ""}}}`
	spec, provisioningOrder, err = validate(newResourceDotSpec("id", properties), pluginLookup)
	require.NoError(t, err)
	expectedSpec = &Spec{Resources: map[string]resourceSpec{
		"a": {Plugin: "p", Properties: types.AnyString(`"{{ resource ` + "`b`" + ` }}"`), plugin: instancePlugin},
		"b": {Plugin: "p", Properties: types.AnyString(`""`), plugin: instancePlugin},
	}}
	require.Equal(t, expectedSpec, spec)
	require.Equal(t, []string{"b", "a"}, provisioningOrder)
}

func TestGetProvisioningOrder(t *testing.T) {
	newSpec := func(resources map[string]string) Spec {
		spec := Spec{Resources: map[string]resourceSpec{}}
		for name, properties := range resources {
			spec.Resources[name] = resourceSpec{Properties: types.AnyString(properties)}
		}
		return spec
	}

	_, err := getProvisioningOrder(newSpec(map[string]string{
		"a": `{{ resource "nonexistent" }}`,
	}))
	require.Error(t, err)

	_, err = getProvisioningOrder(newSpec(map[string]string{
		"a": `{{ resource "a" }}`,
	}))
	require.Error(t, err)

	_, err = getProvisioningOrder(newSpec(map[string]string{
		"a": `{{ resource "b" }}`,
		"b": `{{ resource "a" }}`,
	}))
	require.Error(t, err)

	order, err := getProvisioningOrder(newSpec(map[string]string{
		"a": `{{ resource "b" }}`,
		"b": ``,
	}))
	require.NoError(t, err)
	require.Equal(t, []string{"b", "a"}, order)

	order, err = getProvisioningOrder(newSpec(map[string]string{
		"a": `{{ resource "b" }}`,
		"b": `{{ resource "c" }} {{ resource "d" }}`,
		"c": `{{ resource "e" }}`,
		"d": `{{ resource "e" }}`,
		"e": `{{ resource "f" }}`,
		"f": ``,
	}))
	require.NoError(t, err)
	require.Condition(t, func() bool {
		return (reflect.DeepEqual(order, []string{"f", "e", "d", "c", "b", "a"}) ||
			reflect.DeepEqual(order, []string{"f", "e", "c", "d", "b", "a"}))

	})
}

func TestGetResourceReferences(t *testing.T) {
	refs, err := getResourceReferences(types.AnyString(``))
	require.NoError(t, err)
	require.Len(t, refs, 0)

	_, err = getResourceReferences(types.AnyString(`{{/* malformed template`))
	require.Error(t, err)

	_, err = getResourceReferences(types.AnyString(`{{ wellFormedTemplateWithExecutionError }}`))
	require.Error(t, err)

	refs, err = getResourceReferences(types.AnyString(`{{ resource "a" }}`))
	require.NoError(t, err)
	require.Equal(t, []string{"a"}, refs)

	refs, err = getResourceReferences(types.AnyString(`{{ resource "a" }} {{ resource "a" }}`))
	require.NoError(t, err)
	require.Equal(t, []string{"a"}, refs)

	refs, err = getResourceReferences(types.AnyString(`{{ resource "a" }} {{ resource "b" }} {{ resource "c" }}`))
	require.NoError(t, err)
	sort.Strings(refs)
	require.Equal(t, []string{"a", "b", "c"}, refs)
}
