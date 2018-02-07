package group

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	group_controller "github.com/docker/infrakit/pkg/controller/group"
	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	"github.com/docker/infrakit/pkg/provider/google/plugin/gcloud"
	instance_types "github.com/docker/infrakit/pkg/provider/google/plugin/instance/types"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

type settings struct {
	spec               group_types.Spec
	groupSpec          group.Spec
	instanceSpec       instance.Spec
	instanceProperties instance_types.Properties
	currentTemplate    int
	createdTemplates   []string
}

type plugin struct {
	API           gcloud.API
	flavorPlugins group_controller.FlavorPluginLookup
	groups        map[group.ID]settings
	lock          sync.Mutex
}

// NewGCEGroupPlugin creates a new GCE group plugin for a given project
// and zone.
func NewGCEGroupPlugin(project, zone string, flavorPlugins group_controller.FlavorPluginLookup) group.Plugin {
	api, err := gcloud.NewAPI(project, zone)
	if err != nil {
		log.Fatal(err)
	}

	return &plugin{
		API:           api,
		flavorPlugins: flavorPlugins,
		groups:        map[group.ID]settings{},
	}
}

func (p *plugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-group-gcp",
			Version: "0.3.0",
		},
		URL: "https://github.com/docker/infrakit/pkg/provider/google",
	}
}

func (p *plugin) validate(groupSpec group.Spec) (settings, error) {
	noSettings := settings{}

	if groupSpec.ID == "" {
		return noSettings, errors.New("Group ID must not be blank")
	}

	spec, err := group_types.ParseProperties(groupSpec)
	if err != nil {
		return noSettings, err
	}

	if spec.Allocation.LogicalIDs != nil {
		return noSettings, errors.New("Allocation.LogicalIDs is not supported")
	}

	if spec.Allocation.Size <= 0 {
		return noSettings, errors.New("Allocation must be > 0")
	}

	flavorPlugin, err := p.flavorPlugins(spec.Flavor.Plugin)
	if err != nil {
		return noSettings, fmt.Errorf("Failed to find Flavor plugin '%s':%v", spec.Flavor.Plugin, err)
	}

	err = flavorPlugin.Validate(spec.Flavor.Properties, spec.Allocation)
	if err != nil {
		return noSettings, err
	}

	instanceSpec := instance.Spec{
		Tags:       map[string]string{},
		Properties: spec.Instance.Properties,
	}

	instanceGroupInstances, err := p.API.ListInstanceGroupInstances(string(groupSpec.ID))
	if err != nil {
		return noSettings, err
	}

	index := group.Index{
		Group:    groupSpec.ID,
		Sequence: uint(len(instanceGroupInstances)),
	}
	instanceSpec, err = flavorPlugin.Prepare(spec.Flavor.Properties, instanceSpec, spec.Allocation, index)
	if err != nil {
		return noSettings, err
	}

	instanceProperties, err := instance_types.ParseProperties(instanceSpec.Properties)
	if err != nil {
		return noSettings, err
	}

	return settings{
		spec:               spec,
		groupSpec:          groupSpec,
		instanceSpec:       instanceSpec,
		instanceProperties: instanceProperties,
		currentTemplate:    1,
	}, nil
}

// TODO: handle reusing existing group
func (p *plugin) CommitGroup(config group.Spec, pretend bool) (string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	newSettings, err := p.validate(config)
	if err != nil {
		return "", err
	}

	log.Infof("Committing group %s (pretend=%t)", config.ID, pretend)

	name := string(config.ID)
	targetSize := int64(newSettings.spec.Allocation.Size)

	operations := []string{}
	createManager := false
	createTemplate := false
	updateManager := false
	resize := false

	settings, present := p.groups[config.ID]
	if !present {
		settings = newSettings

		operations = append(operations, fmt.Sprintf("Managing %d instances", targetSize))
		createManager = true
		createTemplate = true
	} else {
		if !reflect.DeepEqual(settings.instanceProperties, newSettings.instanceProperties) {
			operations = append(operations, "Updating instance template")
			createTemplate = true
			if !pretend {
				settings.currentTemplate++
			}
		}

		if settings.spec.Allocation.Size != newSettings.spec.Allocation.Size {
			operations = append(operations, fmt.Sprintf("Scaling group to %d instance.", targetSize))
			resize = true
		}
	}

	if !pretend {
		templateName := fmt.Sprintf("%s-%d", name, settings.currentTemplate)
		settings.createdTemplates = append(settings.createdTemplates, templateName)

		if createTemplate {
			spec := settings.instanceSpec
			settings := settings.instanceProperties.InstanceSettings

			// TODO - for now we overwrite, but support merging of MetaData field in the future, if the
			// user provided some.
			tags, err := instance_types.ParseTags(spec)
			if err != nil {
				return "", err
			}
			settings.MetaData = gcloud.TagsToMetaData(tags)

			if err = p.API.CreateInstanceTemplate(templateName, settings); err != nil {
				return "", err
			}
		}

		if createManager {
			if err = p.API.CreateInstanceGroupManager(name, &gcloud.InstanceManagerSettings{
				TemplateName:     fmt.Sprintf("%s-%d", name, settings.currentTemplate),
				TargetSize:       targetSize,
				Description:      settings.instanceProperties.Description,
				TargetPools:      settings.instanceProperties.TargetPools,
				BaseInstanceName: settings.instanceProperties.NamePrefix,
			}); err != nil {
				return "", err
			}
		}

		if updateManager {
			// TODO: should we trigger a recreation of the VMS
			// TODO: What about the instances already being updated
			if err = p.API.SetInstanceTemplate(name, templateName); err != nil {
				return "", err
			}
		}

		if resize {
			err := p.API.ResizeInstanceGroupManager(name, targetSize)
			if err != nil {
				return "", err
			}
		}
	}

	p.groups[config.ID] = settings

	return strings.Join(operations, "\n"), nil
}

func (p *plugin) FreeGroup(id group.ID) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	_, present := p.groups[id]
	if !present {
		return fmt.Errorf("This group is not being watched: '%s", id)
	}

	delete(p.groups, id)

	return nil
}

func (p *plugin) DescribeGroup(id group.ID) (group.Description, error) {
	noDescription := group.Description{}

	p.lock.Lock()
	defer p.lock.Unlock()

	currentSettings, present := p.groups[id]
	if !present {
		return noDescription, fmt.Errorf("This group is not being watched: '%s", id)
	}

	name := string(id)

	instanceGroupInstances, err := p.API.ListInstanceGroupInstances(name)
	if err != nil {
		return noDescription, err
	}

	instances := []instance.Description{}

	for _, grpInst := range instanceGroupInstances {
		name := last(grpInst.Instance)

		inst, err := p.API.GetInstance(name)
		if err != nil {
			return noDescription, err
		}

		instances = append(instances, instance.Description{
			ID:   instance.ID(inst.Name),
			Tags: gcloud.MetaDataToTags(inst.Metadata.Items),
		})
	}

	return group.Description{
		Converged: len(instanceGroupInstances) == int(currentSettings.spec.Allocation.Size),
		Instances: instances,
	}, nil
}

func (p *plugin) DestroyGroup(id group.ID) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	currentSettings, present := p.groups[id]
	if !present {
		return fmt.Errorf("This group is not being watched: '%s", id)
	}

	name := string(id)

	if err := p.API.DeleteInstanceGroupManager(name); err != nil {
		return err
	}

	for _, createdTemplate := range currentSettings.createdTemplates {
		if err := p.API.DeleteInstanceTemplate(createdTemplate); err != nil {
			return err
		}
	}

	delete(p.groups, id)

	return nil
}

func (p *plugin) InspectGroups() ([]group.Spec, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	specs := []group.Spec{}
	for _, spec := range p.groups {
		specs = append(specs, spec.groupSpec)
	}

	return specs, nil
}

// DestroyInstances TODO(chungers) - implement this
func (p *plugin) DestroyInstances(id group.ID, instances []instance.ID) error {
	return fmt.Errorf("not implemented")
}

// Size TODO(chungers) - implement this
func (p *plugin) Size(id group.ID) (int, error) {
	return 0, fmt.Errorf("not implemented")
}

func (p *plugin) SetSize(id group.ID, size int) error {
	return p.API.ResizeInstanceGroupManager(string(id), int64(size))
}

func last(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}
