package resource

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"text/template"

	log "github.com/Sirupsen/logrus"
	plugin_base "github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/types"
	"github.com/twmb/algoimpl/go/graph"
)

const (
	resourceGroupTag = "infrakit.resource-group"
	resourceNameTag  = "infrakit.resource-name"
)

// Spec is the configuration schema for this plugin, provided in resource.Spec.Properties.
type Spec struct {
	Resources map[string]*struct {
		Plugin     plugin_base.Name
		plugin     instance.Plugin
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

func (p *plugin) validate(config resource.Spec) (*Spec, []string, error) {
	spec := Spec{}
	if err := config.Properties.Decode(&spec); err != nil {
		return nil, nil, fmt.Errorf("Invalid properties %q: %s", config.Properties, err)
	}

	for name, resourceSpec := range spec.Resources {
		instancePlugin, err := p.instancePluginLookup(resourceSpec.Plugin)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to find plugin %s for %s: %s", resourceSpec.Plugin, name, err)
		}
		if err := instancePlugin.Validate(json.RawMessage(*resourceSpec.Properties)); err != nil {
			return nil, nil, fmt.Errorf("Failed to validate spec for %s: %s", name, err)
		}
		resourceSpec.plugin = instancePlugin
	}

	provisioningOrder, err := getProvisioningOrder(spec)
	if err != nil {
		return nil, nil, err
	}

	return &spec, provisioningOrder, nil
}

func (p *plugin) describe(spec Spec, configID resource.ID) (map[string]instance.ID, error) {
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
			return nil, fmt.Errorf("Found multiple resources named %s: %v", name, idList)
		}
	}

	return ids, nil
}

func (p *plugin) Commit(config resource.Spec, pretend bool) (string, error) {
	spec, provisioningOrder, err := p.validate(config)
	if err != nil {
		return "", err
	}

	ids, err := p.describe(*spec, config.ID)
	if err != nil {
		return "", err
	}

	idStructs := map[string]struct{ instance.ID }{}
	for name, id := range ids {
		idStructs[name] = struct{ instance.ID }{id}
	}

	for _, name := range provisioningOrder {
		if _, ok := idStructs[name]; ok {
			continue
		}

		resourceSpec := spec.Resources[name]
		properties, err := executeAsTemplate(json.RawMessage(*resourceSpec.Properties), struct{ Resources interface{} }{idStructs})
		if err != nil {
			return "", fmt.Errorf("Failed to get properties for %s: %s", name, err)
		}

		if pretend {
			idStructs[name] = struct{ instance.ID }{instance.ID("unknown")}
		} else {
			id, err := resourceSpec.plugin.Provision(instance.Spec{
				Properties: &properties,
				Tags:       map[string]string{resourceGroupTag: string(config.ID), resourceNameTag: name},
			})
			if err != nil {
				return "", fmt.Errorf("Failed to provision %s: %s", name, err)
			}

			log.Infof("Provisioned %s (ID %s)", name, *id)
			idStructs[name] = struct{ instance.ID }{*id}
		}
	}

	var desc string
	for _, name := range provisioningOrder {
		var idStr, verb string
		if id, ok := ids[name]; ok {
			verb = "Found"
			idStr = string(id)
		} else {
			verb = "Created"
			idStr = "N/A"
			if idStruct, ok := idStructs[name]; ok {
				idStr = string(idStruct.ID)
			}
		}
		desc += fmt.Sprintf("\n%s %s (%s)", verb, name, idStr)
	}

	return desc, nil
}

func (p *plugin) Destroy(config resource.Spec, pretend bool) (string, error) {
	spec, provisioningOrder, err := p.validate(config)
	if err != nil {
		return "", err
	}

	ids, err := p.describe(*spec, config.ID)
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
				return "", fmt.Errorf("Failed to destroy %s (ID %s): %s", name, id, err)
			}

			log.Infof("Detroyed %s (ID %s)", name, id)
		}
	}

	var desc string
	for name, id := range ids {
		desc += fmt.Sprintf("\nDestroyed %s (ID %s)", name, id)
	}

	return desc, nil
}

func (p *plugin) DescribeResources() ([]instance.Description, error) {
	return nil, errors.New("unimplemented")
}

func executeAsTemplate(text json.RawMessage, data interface{}) (json.RawMessage, error) {
	tmpl, err := template.New("").Option("missingkey=error").Parse(string(text))
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	if err = tmpl.Execute(&b, data); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

var resourceReferenceRegexp = regexp.MustCompile(`{{\s*\.Resources\.(\w+)`)

func getResourceReferences(properties json.RawMessage) []string {
	var references []string
	// TODO: Use text/template.Template.Execute instead.
	for _, submatches := range resourceReferenceRegexp.FindAllSubmatch(properties, -1) {
		references = append(references, string(submatches[1]))
	}
	return references
}

func getProvisioningOrder(spec Spec) ([]string, error) {
	g := graph.New(graph.Directed)

	nodes := map[string]graph.Node{}
	for name := range spec.Resources {
		nodes[name] = g.MakeNode()
		*nodes[name].Value = name
	}

	for name, resourceSpec := range spec.Resources {
		to := nodes[name]
		references := getResourceReferences(json.RawMessage(*resourceSpec.Properties))
		for _, reference := range references {
			from, ok := nodes[reference]
			if !ok {
				return nil, fmt.Errorf("Reference to undefined resource %s in %s", reference, name)
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

	var sorted []string
	for _, node := range g.TopologicalSort() {
		sorted = append(sorted, (*node.Value).(string))
	}

	return sorted, nil
}
