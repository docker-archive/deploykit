package resource

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery"
	plugin_base "github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/twmb/algoimpl/go/graph"
)

const (
	resourceGroupTag = "infrakit.resource-group"
	resourceNameTag  = "infrakit.resource-name"
)

// Spec is the configuration schema for this plugin, provided in resource.Spec.Properties.
type Spec struct {
	Resources map[string]resourceSpec
}

type resourceSpec struct {
	Plugin     plugin_base.Name
	Properties *types.Any

	plugin instance.Plugin
}

// InstancePluginLookup looks up a plugin by name.
type InstancePluginLookup func(plugin_base.Name) (instance.Plugin, error)

// NewResourcePlugin creates a new resource plugin.
func NewResourcePlugin(instancePluginLookup InstancePluginLookup) resource.Plugin {
	return &plugin{
		instancePluginLookup: instancePluginLookup,
	}
}

type plugin struct {
	instancePluginLookup InstancePluginLookup
}

func (p *plugin) Commit(config resource.Spec, pretend bool) (string, error) {
	spec, provisioningOrder, err := validate(config, p.instancePluginLookup)
	if err != nil {
		return "", err
	}

	ids, err := describe(config.ID, *spec)
	if err != nil {
		return "", err
	}

	foundIDs := map[string]instance.ID{}
	for name, id := range ids {
		foundIDs[name] = id
	}

	f := func(ref string) (string, error) {
		if val, ok := ids[ref]; ok {
			return string(val), nil
		}
		return "", fmt.Errorf("Undefined resource %s", ref)
	}

	for _, name := range provisioningOrder {
		if _, ok := ids[name]; ok {
			continue
		}

		resourceSpec := spec.Resources[name]

		template, err := template.NewTemplate("str://"+resourceSpec.Properties.String(), template.Options{SocketDir: discovery.Dir()})
		if err != nil {
			return "", fmt.Errorf("Template parse error for %s: %s", name, err)
		}

		properties, err := template.AddFunc("resource", f).Render(nil)
		if err != nil {
			return "", fmt.Errorf("Template execution error for %s: %s", name, err)
		}

		if pretend {
			ids[name] = instance.ID("unknown")
		} else {
			id, err := resourceSpec.plugin.Provision(instance.Spec{
				Properties: types.AnyString(properties),
				Tags:       map[string]string{resourceGroupTag: string(config.ID), resourceNameTag: name},
			})
			if err != nil {
				return "", fmt.Errorf("Failed to provision %s: %s", name, err)
			}
			log.Infof("Provisioned %s (%s)", name, *id)
			ids[name] = *id
		}
	}

	var desc string
	for _, name := range provisioningOrder {
		var idStr, verb string
		if id, ok := foundIDs[name]; ok {
			verb = "Found"
			idStr = string(id)
		} else {
			verb = "Provisioned"
			idStr = "N/A"
			if id, ok := ids[name]; ok {
				idStr = string(id)
			}
		}
		desc += fmt.Sprintf("\n%s %s (%s)", verb, name, idStr)
	}

	return desc, nil
}

func (p *plugin) Destroy(config resource.Spec, pretend bool) (string, error) {
	spec, provisioningOrder, err := validate(config, p.instancePluginLookup)
	if err != nil {
		return "", err
	}

	ids, err := describe(config.ID, *spec)
	if err != nil {
		return "", err
	}

	// Traverse provisioningOrder in reverse.
	for i := len(provisioningOrder) - 1; i >= 0; i-- {
		name := provisioningOrder[i]

		id, ok := ids[name]
		if !ok {
			continue
		}

		if !pretend {
			if err = spec.Resources[name].plugin.Destroy(id); err != nil {
				return "", fmt.Errorf("Failed to destroy %s (%s): %s", name, id, err)
			}

			log.Infof("Detroyed %s (%s)", name, id)
		}
	}

	var desc string
	for name, id := range ids {
		desc += fmt.Sprintf("\nDestroyed %s (%s)", name, id)
	}

	return desc, nil
}

func (p *plugin) DescribeResources(config resource.Spec) (string, error) {
	spec, _, err := validate(config, p.instancePluginLookup)
	if err != nil {
		return "", err
	}

	ids, err := describe(config.ID, *spec)
	if err != nil {
		return "", err
	}

	details := []string{}
	for name, id := range ids {
		details = append(details, fmt.Sprintf("Found %s (%s)", name, id))
	}
	sort.Strings(details)

	return strings.Join(details, "\n"), nil
}

func describe(configID resource.ID, spec Spec) (map[string]instance.ID, error) {
	ids := map[string]instance.ID{}
	for name, resourceSpec := range spec.Resources {
		tags := map[string]string{resourceGroupTag: string(configID), resourceNameTag: name}
		descriptions, err := resourceSpec.plugin.DescribeInstances(tags)
		if err != nil {
			return nil, fmt.Errorf("Describe failed for %s: %s", name, err)
		}

		switch len(descriptions) {
		case 0:
			break
		case 1:
			log.Infof("Found %s with ID %s", name, descriptions[0].ID)
			ids[name] = descriptions[0].ID
		default:
			var idList []instance.ID
			for _, d := range descriptions {
				idList = append(idList, d.ID)
			}
			return nil, fmt.Errorf("Found multiple resources for %s: %v", name, idList)
		}
	}

	return ids, nil
}

func validate(config resource.Spec, instancePluginLookup InstancePluginLookup) (*Spec, []string, error) {
	if config.ID == "" {
		return nil, nil, errors.New("ID must be set")
	}
	if config.Properties == nil {
		return nil, nil, errors.New("Properties must be set")
	}

	spec := Spec{}
	if err := config.Properties.Decode(&spec); err != nil {
		return nil, nil, fmt.Errorf("Invalid properties '%s': %s", config.Properties, err)
	}

	for name, resourceSpec := range spec.Resources {
		instancePlugin, err := instancePluginLookup(resourceSpec.Plugin)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to find plugin %s for %s: %s", resourceSpec.Plugin, name, err)
		}

		if resourceSpec.Properties == nil {
			return nil, nil, fmt.Errorf("Properties must be set for %s", name)
		}

		if err := instancePlugin.Validate(resourceSpec.Properties); err != nil {
			return nil, nil, fmt.Errorf("Failed to validate spec for %s: %s", name, err)
		}

		if _, err = template.NewTemplate("str://"+resourceSpec.Properties.String(), template.Options{SocketDir: discovery.Dir()}); err != nil {
			return nil, nil, fmt.Errorf("Template parse error for %s: %s", name, err)
		}

		resourceSpec.plugin = instancePlugin
		spec.Resources[name] = resourceSpec
	}

	provisioningOrder, err := getProvisioningOrder(spec)
	if err != nil {
		return nil, nil, err
	}

	log.Infof("Provisioning order: %s", provisioningOrder)
	return &spec, provisioningOrder, nil
}

func getProvisioningOrder(spec Spec) ([]string, error) {
	g := graph.New(graph.Directed)

	nodes := map[string]graph.Node{}
	for name := range spec.Resources {
		nodes[name] = g.MakeNode()
		*nodes[name].Value = name
	}

	for name, resourceSpec := range spec.Resources {
		references, err := getResourceReferences(resourceSpec.Properties)
		if err != nil {
			return nil, fmt.Errorf("%s for resource %s with properties %s ", err, name, resourceSpec.Properties.String())
		}

		to := nodes[name]
		for _, reference := range references {
			from, ok := nodes[reference]
			if !ok {
				return nil, fmt.Errorf("Resource %s depends on undefined resource %s", name, reference)
			}
			if from == to {
				return nil, fmt.Errorf("Resource %s depends on itself", name)
			}
			if err := g.MakeEdge(from, to); err != nil {
				return nil, err
			}
		}
	}

	for _, connectedComponent := range g.StronglyConnectedComponents() {
		if len(connectedComponent) > 1 {
			var cycle []string
			for _, node := range connectedComponent {
				cycle = append(cycle, (*node.Value).(string))
			}
			sort.Strings(cycle)
			return nil, fmt.Errorf("Cycle detected: %v", cycle)
		}
	}

	order := []string{}
	for _, node := range g.TopologicalSort() {
		order = append(order, (*node.Value).(string))
	}

	return order, nil
}

func getResourceReferences(properties *types.Any) ([]string, error) {
	template, err := template.NewTemplate("str://"+properties.String(), template.Options{SocketDir: discovery.Dir()})
	if err != nil {
		return nil, fmt.Errorf("Template parse error: %s", err)
	}

	refMap := map[string]bool{}
	f := func(ref string) string { refMap[ref] = true; return "" }

	if _, err := template.AddFunc("resource", f).Render(nil); err != nil {
		return nil, fmt.Errorf("Template execution error: %s", err)
	}

	refList := []string{}
	for ref := range refMap {
		refList = append(refList, ref)
	}
	return refList, nil
}
