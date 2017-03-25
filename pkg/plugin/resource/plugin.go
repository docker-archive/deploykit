package resource

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/discovery/local"
	plugin_base "github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/twmb/algoimpl/go/graph"
)

const (
	resourcesTag   = "infrakit.resources"
	resourcesIDTag = "infrakit.resources.id"
)

// Spec is the configuration schema for this plugin, provided in resource.Spec.Properties.
type Spec struct {
	Resources []struct {
		ID         string
		Plugin     plugin_base.Name
		Properties *types.Any
	}
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

type resourceConfig struct {
	plugin     instance.Plugin
	properties string
}

func (p *plugin) Commit(config resource.Spec, pretend bool) (string, error) {
	resourceConfigs, provisioningOrder, err := validate(config, p.instancePluginLookup)
	if err != nil {
		return "", err
	}

	log.Infof("Committing %s (pretend=%t)", config.ID, pretend)

	instanceIDs, err := describe(config.ID, resourceConfigs)
	if err != nil {
		return "", err
	}

	resourceTemplateFunc := func(name string) (string, error) {
		if id, ok := instanceIDs[name]; ok {
			return string(id), nil
		}
		return "", fmt.Errorf("Undefined resource '%s'", name)
	}

	details := []string{}
	for _, name := range provisioningOrder {
		if id, ok := instanceIDs[name]; ok {
			details = append(details, fmt.Sprintf("Found %s (%s)", name, id))
			continue
		}

		template, err := template.NewTemplate("str://"+resourceConfigs[name].properties, template.Options{SocketDir: local.Dir()})
		if err != nil {
			return "", fmt.Errorf("Failed to parse template '%s' for resource '%s': %s", resourceConfigs[name].properties, name, err)
		}

		properties, err := template.AddFunc("resource", resourceTemplateFunc).Render(nil)
		if err != nil {
			return "", fmt.Errorf("Failed to execute template '%s' for resource '%s': %s", resourceConfigs[name].properties, name, err)
		}

		detail := ""
		if pretend {
			instanceIDs[name] = instance.ID("unknown")

			detail = fmt.Sprintf("Would provision %s", name)
		} else {
			id, err := resourceConfigs[name].plugin.Provision(instance.Spec{
				Properties: types.AnyString(properties),
				Tags: map[string]string{
					resourcesTag:   string(config.ID),
					resourcesIDTag: name,
				},
			})
			if err != nil {
				return "", fmt.Errorf("Failed to provision resource '%s': %s", name, err)
			}
			instanceIDs[name] = *id

			detail = fmt.Sprintf("Provisioned %s (%s)", name, *id)
			log.Info(detail)
		}

		details = append(details, detail)
	}

	sort.Strings(details)
	return strings.Join(details, "\n"), nil
}

func (p *plugin) Destroy(config resource.Spec, pretend bool) (string, error) {
	resourceConfigs, provisioningOrder, err := validate(config, p.instancePluginLookup)
	if err != nil {
		return "", err
	}

	log.Infof("Destroying %s (pretend=%t)", config.ID, pretend)

	instanceIDs, err := describe(config.ID, resourceConfigs)
	if err != nil {
		return "", err
	}

	details := []string{}

	// Traverse provisioningOrder in reverse.
	for i := len(provisioningOrder) - 1; i >= 0; i-- {
		name := provisioningOrder[i]

		id, ok := instanceIDs[name]
		if !ok {
			continue
		}

		detail := ""
		if pretend {
			detail = fmt.Sprintf("Would destroy %s (%s)", name, id)
		} else {
			if err = resourceConfigs[name].plugin.Destroy(id); err != nil {
				return "", fmt.Errorf("Failed to destroy resource '%s' (%s): %s", name, id, err)
			}

			detail = fmt.Sprintf("Destroyed %s (%s)", name, id)
			log.Infof(detail)
		}

		details = append(details, detail)
	}

	sort.Strings(details)
	return strings.Join(details, "\n"), nil
}

func (p *plugin) DescribeResources(config resource.Spec) (string, error) {
	resourceConfigs, _, err := validate(config, p.instancePluginLookup)
	if err != nil {
		return "", err
	}

	log.Infof("Describing %s", config.ID)

	instanceIDs, err := describe(config.ID, resourceConfigs)
	if err != nil {
		return "", err
	}

	details := []string{}
	for name, id := range instanceIDs {
		details = append(details, fmt.Sprintf("Found %s (%s)", name, id))
	}

	sort.Strings(details)
	return strings.Join(details, "\n"), nil
}

func describe(id resource.ID, resourceConfigs map[string]resourceConfig) (map[string]instance.ID, error) {
	instanceIDs := map[string]instance.ID{}

	for name, resourceConfig := range resourceConfigs {
		descriptions, err := resourceConfig.plugin.DescribeInstances(map[string]string{
			resourcesTag:   string(id),
			resourcesIDTag: name,
		})
		if err != nil {
			return nil, fmt.Errorf("Failed to describe resource '%s': %s", name, err)
		}

		switch len(descriptions) {
		case 0:
			break
		case 1:
			instanceIDs[name] = descriptions[0].ID
			log.Infof("Found %s (%s)'", name, descriptions[0].ID)
		default:
			ids := []instance.ID{}
			for _, d := range descriptions {
				ids = append(ids, d.ID)
			}
			return nil, fmt.Errorf("Found multiple instance IDs for resource '%s': %v", name, ids)
		}
	}

	return instanceIDs, nil
}

func validate(config resource.Spec, instancePluginLookup InstancePluginLookup) (map[string]resourceConfig, []string, error) {
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

	resourceConfigs := map[string]resourceConfig{}
	for _, resourceSpec := range spec.Resources {
		if resourceSpec.ID == "" {
			return nil, nil, errors.New("Resource ID must be set")
		}

		if _, ok := resourceConfigs[resourceSpec.ID]; ok {
			return nil, nil, fmt.Errorf("Duplicate resource ID '%s'", resourceSpec.ID)
		}

		instancePlugin, err := instancePluginLookup(resourceSpec.Plugin)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to find plugin '%s' for resource '%s': %s", resourceSpec.Plugin, resourceSpec.ID, err)
		}

		if resourceSpec.Properties == nil {
			return nil, nil, fmt.Errorf("Properties must be set for resource '%s'", resourceSpec.ID)
		}

		if err := instancePlugin.Validate(resourceSpec.Properties); err != nil {
			return nil, nil, fmt.Errorf("Failed to validate spec '%s' for resource '%s': %s", resourceSpec.Properties, resourceSpec.ID, err)
		}

		resourceConfigs[resourceSpec.ID] = resourceConfig{
			plugin:     instancePlugin,
			properties: resourceSpec.Properties.String(),
		}
	}

	provisioningOrder, err := getProvisioningOrder(resourceConfigs)
	if err != nil {
		return nil, nil, err
	}
	log.Infof("Provisioning order: %s", provisioningOrder)

	return resourceConfigs, provisioningOrder, nil
}

func getProvisioningOrder(resourceConfigs map[string]resourceConfig) ([]string, error) {
	g := graph.New(graph.Directed)

	nodes := map[string]graph.Node{}
	for name := range resourceConfigs {
		nodes[name] = g.MakeNode()
		*nodes[name].Value = name
	}

	for name, resourceConfig := range resourceConfigs {
		template, err := template.NewTemplate("str://"+resourceConfig.properties, template.Options{SocketDir: local.Dir()})
		if err != nil {
			return nil, fmt.Errorf("Failed to parse template for resource '%s': %s", name, err)
		}

		dependencies, err := getResourceDependencies(template)
		if err != nil {
			return nil, fmt.Errorf("Failed to get dependencies for resource '%s': %s", name, err)
		}

		to := nodes[name]
		for dependency := range dependencies {
			from, ok := nodes[dependency]
			if !ok {
				return nil, fmt.Errorf("Resource '%s' depends on undefined resource '%s'", name, dependency)
			}
			if from == to {
				return nil, fmt.Errorf("Resource '%s' depends on itself", name)
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

func getResourceDependencies(template *template.Template) (map[string]bool, error) {
	dependencies := map[string]bool{}
	resourceTemplateFunc := func(name string) string {
		dependencies[name] = true
		return ""
	}
	if _, err := template.AddFunc("resource", resourceTemplateFunc).Render(nil); err != nil {
		return nil, err
	}

	return dependencies, nil
}
