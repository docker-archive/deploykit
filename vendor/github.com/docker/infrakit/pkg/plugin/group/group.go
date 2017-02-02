package group

import (
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	plugin_base "github.com/docker/infrakit/pkg/plugin"
	group_types "github.com/docker/infrakit/pkg/plugin/group/types"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
)

const (
	groupTag  = "infrakit.group"
	configTag = "infrakit.config_sha"
)

// InstancePluginLookup helps with looking up an instance plugin by name
type InstancePluginLookup func(plugin_base.Name) (instance.Plugin, error)

// FlavorPluginLookup helps with looking up a flavor plugin by name
type FlavorPluginLookup func(plugin_base.Name) (flavor.Plugin, error)

// NewGroupPlugin creates a new group plugin.
func NewGroupPlugin(
	instancePlugins InstancePluginLookup,
	flavorPlugins FlavorPluginLookup,
	pollInterval time.Duration,
	maxParallelNum uint) group.Plugin {

	return &plugin{
		instancePlugins: instancePlugins,
		flavorPlugins:   flavorPlugins,
		pollInterval:    pollInterval,
		maxParallelNum:  maxParallelNum,
		groups:          groups{byID: map[group.ID]*groupContext{}},
	}
}

type plugin struct {
	instancePlugins InstancePluginLookup
	flavorPlugins   FlavorPluginLookup
	pollInterval    time.Duration
	maxParallelNum  uint
	lock            sync.Mutex
	groups          groups
}

func (p *plugin) CommitGroup(config group.Spec, pretend bool) (string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	settings, err := p.validate(config)
	if err != nil {
		return "", err
	}

	log.Infof("Committing group %s (pretend=%t)", config.ID, pretend)

	context, exists := p.groups.get(config.ID)
	if exists {
		if !pretend {
			// Halt the existing update to prevent interference.
			context.stopUpdating()
		}

		// TODO(wfarner): Change the updater behaviors to handle creating a group from scratch.  This should
		// not be much work, and will make this routine easier to follow.

		// TODO(wfarner): Don't hold the lock - this is a blocking operation.
		updatePlan, err := context.supervisor.PlanUpdate(context.scaled, context.settings, settings)
		if err != nil {
			return "unable to fulfill request", err
		}

		if !pretend {
			context.setUpdate(updatePlan)
			context.changeSettings(settings)
			go func() {
				log.Infof("Executing update plan for '%s': %s", config.ID, updatePlan.Explain())
				if err := updatePlan.Run(p.pollInterval); err != nil {
					log.Errorf("Update to %s failed: %s", config.ID, err)
				} else {
					log.Infof("Group %s has converged", config.ID)
				}
				context.setUpdate(nil)
			}()
		}

		return updatePlan.Explain(), nil
	}

	scaled := &scaledGroup{
		settings:   settings,
		memberTags: map[string]string{groupTag: string(config.ID)},
	}

	var supervisor Supervisor
	if settings.config.Allocation.Size != 0 {
		supervisor = NewScalingGroup(scaled, settings.config.Allocation.Size, p.pollInterval, p.maxParallelNum)
	} else if len(settings.config.Allocation.LogicalIDs) > 0 {
		supervisor = NewQuorum(scaled, settings.config.Allocation.LogicalIDs, p.pollInterval)
	} else {
		panic("Invalid empty allocation method")
	}

	if !pretend {
		p.groups.put(config.ID, &groupContext{supervisor: supervisor, scaled: scaled, settings: settings})
		go supervisor.Run()
	}

	return fmt.Sprintf("Managing %d instances", supervisor.Size()), nil
}

func (p *plugin) doFree(id group.ID) (*groupContext, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	grp, exists := p.groups.get(id)
	if !exists {
		return nil, fmt.Errorf("Group '%s' is not being watched", id)
	}

	grp.stopUpdating()
	grp.supervisor.Stop()
	p.groups.del(id)

	log.Infof("Ignored group '%s'", id)
	return grp, nil
}

func (p *plugin) FreeGroup(id group.ID) error {
	_, err := p.doFree(id)
	return err
}

func (p *plugin) DescribeGroup(id group.ID) (group.Description, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// TODO(wfarner): Include details about any in-flight updates.

	context, exists := p.groups.get(id)
	if !exists {
		return group.Description{}, fmt.Errorf("Group '%s' is not being watched", id)
	}

	instances, err := context.scaled.List()
	if err != nil {
		return group.Description{}, err
	}

	return group.Description{Instances: instances, Converged: !context.updating()}, nil
}

func (p *plugin) DestroyGroup(gid group.ID) error {
	context, err := p.doFree(gid)

	if context != nil {
		descriptions, err := context.scaled.List()
		if err != nil {
			return err
		}

		for _, desc := range descriptions {
			context.scaled.Destroy(desc)
		}
	}

	return err
}

func (p *plugin) InspectGroups() ([]group.Spec, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	var specs []group.Spec
	err := p.groups.forEach(func(id group.ID, ctx *groupContext) error {
		if ctx != nil {
			spec, err := group_types.UnparseProperties(string(id), ctx.settings.config)
			if err != nil {
				return err
			}
			specs = append(specs, spec)
		}
		return nil
	})
	return specs, err
}

type updatePlan interface {
	Explain() string
	Run(pollInterval time.Duration) error
	Stop()
}

type noopUpdate struct {
}

func (n noopUpdate) Explain() string {
	return "Noop"
}

func (n noopUpdate) Run(_ time.Duration) error {
	return nil
}

func (n noopUpdate) Stop() {
}

func (p *plugin) validate(config group.Spec) (groupSettings, error) {

	noSettings := groupSettings{}

	if config.ID == "" {
		return noSettings, errors.New("Group ID must not be blank")
	}

	parsed, err := group_types.ParseProperties(config)
	if err != nil {
		return noSettings, err
	}

	if parsed.Allocation.Size == 0 &&
		(parsed.Allocation.LogicalIDs == nil || len(parsed.Allocation.LogicalIDs) == 0) {

		return noSettings, errors.New("Allocation must not be blank")
	}

	if parsed.Allocation.Size > 0 && parsed.Allocation.LogicalIDs != nil && len(parsed.Allocation.LogicalIDs) > 0 {

		return noSettings, errors.New("Only one Allocation method may be used")
	}

	flavorPlugin, err := p.flavorPlugins(parsed.Flavor.Plugin)
	if err != nil {
		return noSettings, fmt.Errorf("Failed to find Flavor plugin '%s':%v", parsed.Flavor.Plugin, err)
	}

	if err := flavorPlugin.Validate(parsed.Flavor.Properties, parsed.Allocation); err != nil {
		return noSettings, err
	}

	instancePlugin, err := p.instancePlugins(parsed.Instance.Plugin)
	if err != nil {
		return noSettings, fmt.Errorf("Failed to find Instance plugin '%s':%v", parsed.Instance.Plugin, err)
	}

	if err := instancePlugin.Validate(parsed.Instance.Properties); err != nil {
		return noSettings, err
	}

	return groupSettings{
		instancePlugin: instancePlugin,
		flavorPlugin:   flavorPlugin,
		config:         parsed,
	}, nil
}
