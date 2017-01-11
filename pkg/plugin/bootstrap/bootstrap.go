package bootstrap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"text/template"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/group"
	spi_resource "github.com/docker/infrakit/pkg/spi/resource"
	"github.com/twmb/algoimpl/go/graph"
)

const (
	bootstrapTag = "infrakit.bootstrap"
	resourceTag  = "infrakit.resource"
)

// ResourcePluginLookup looks up a resource plugin by name.
type ResourcePluginLookup func(string) (spi_resource.Plugin, error)

// NewBootstrapPlugin creates a new bootstrap plugin.
func NewBootstrapPlugin(resourcePlugins ResourcePluginLookup) group.Plugin {
	return &plugin{
		resourcePlugins: resourcePlugins,
	}
}

// ResourceConfig is the structure that describes a resource plugin.
type ResourceConfig struct {
	Plugin     string
	Type       string
	Properties *json.RawMessage // this will be the Spec of the plugin
}

// Spec is the configuration schema for this plugin, provided in group.Spec.Properties.
type Spec struct {
	Resources map[string]ResourceConfig
}

type resource struct {
	plugin spi_resource.Plugin
	config ResourceConfig
}

type plugin struct {
	resourcePlugins ResourcePluginLookup
}

func (p *plugin) CommitGroup(config group.Spec, pretend bool) (string, error) {
	spec := Spec{}
	if err := json.Unmarshal([]byte(types.RawMessage(config.Properties)), &spec); err != nil {
		return "", fmt.Errorf("Invalid properties %q: %s", config.Properties, err)
	}

	resources := map[string]*resource{}
	for name, resourceConfig := range spec.Resources {
		resourcePlugin, err := p.resourcePlugins(resourceConfig.Plugin)
		if err != nil {
			return "", fmt.Errorf("Failed to find resource plugin %s: %s", resourceConfig.Plugin, err)
		}
		if err := resourcePlugin.Validate(resourceConfig.Type, types.RawMessage(resourceConfig.Properties)); err != nil {
			return "", err
		}
		resources[name] = &resource{plugin: resourcePlugin, config: resourceConfig}
	}

	orderedResourceNames, err := getProvisioningOrder(resources)
	if err != nil {
		return "", err
	}

	resourceIDs := map[string]struct{ spi_resource.ID }{}
	for name, resource := range resources {
		tags := map[string]string{bootstrapTag: string(config.ID), resourceTag: name}
		descriptions, err := resource.plugin.DescribeResources(resource.config.Type, tags)
		if err != nil {
			return "", fmt.Errorf("Describe failed for %s: %s", name, err)
		}

		switch len(descriptions) {
		case 0:
			break
		case 1:
			log.Infof("Found %s with ID %s", name, descriptions[0].ID)
			resourceIDs[name] = struct{ spi_resource.ID }{descriptions[0].ID}
		default:
			var ids []spi_resource.ID
			for _, d := range descriptions {
				ids = append(ids, d.ID)
			}
			return "", fmt.Errorf("Found multiple resources named %s: %v", name, ids)
		}
	}
	numFound := len(resourceIDs)

	for _, name := range orderedResourceNames {
		if _, ok := resourceIDs[name]; ok {
			continue
		}

		resource := resources[name]
		properties, err := executeAsTemplate(*resource.config.Properties, struct{ Resources interface{} }{resourceIDs})
		if err != nil {
			return "", fmt.Errorf("Failed to get properties for %s: %s", name, err)
		}

		id, err := resource.plugin.Provision(spi_resource.Spec{
			Type:       resource.config.Type,
			Properties: &properties,
			Tags:       map[string]string{bootstrapTag: string(config.ID), resourceTag: name},
		})
		if err != nil {
			return "", fmt.Errorf("Failed to provision %s: %s", name, err)
		}
		log.Infof("Provisioned %s with ID %s", name, *id)
		resourceIDs[name] = struct{ spi_resource.ID }{*id}
	}

	return fmt.Sprintf("Found %d resources and created %d ", numFound, len(resourceIDs)-numFound), nil
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

func getResourceReferences(properties *json.RawMessage) []string {

	var references []string
	// TODO: Use text/template.Template.Execute instead.
	for _, submatches := range resourceReferenceRegexp.FindAllSubmatch(*properties, -1) {
		references = append(references, string(submatches[1]))
	}
	return references
}

func getProvisioningOrder(resources map[string]*resource) ([]string, error) {
	g := graph.New(graph.Directed)

	nodes := map[string]graph.Node{}
	for name := range resources {
		nodes[name] = g.MakeNode()
		*nodes[name].Value = name
	}

	for name, resource := range resources {
		to := nodes[name]
		references := getResourceReferences(resource.config.Properties)
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

func (p *plugin) FreeGroup(id group.ID) error {
	return errors.New("unimplemented")
}

func (p *plugin) DescribeGroup(id group.ID) (group.Description, error) {
	return group.Description{}, errors.New("unimplemented")
}

func (p *plugin) DestroyGroup(gid group.ID) error {
	return errors.New("unimplemented")
}

func (p *plugin) InspectGroups() ([]group.Spec, error) {
	return nil, errors.New("unimplemented")
}
