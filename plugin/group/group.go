package group

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/plugin/group/types"
	"github.com/docker/libmachete/spi/group"
	"github.com/docker/libmachete/spi/instance"
	"sync"
	"time"
)

const (
	groupTag  = "machete.group"
	configTag = "machete.config_sha"
)

// NewGroupPlugin creates a new group plugin.
func NewGroupPlugin(
	plugins func() map[string]instance.Plugin,
	provisionHelper types.ProvisionHelper,
	pollInterval time.Duration) group.Plugin {

	return &plugin{
		plugins:         plugins,
		provisionHelper: provisionHelper,
		pollInterval:    pollInterval,
		groups:          groups{byID: map[group.ID]*groupContext{}},
	}
}

type plugin struct {
	plugins         func() map[string]instance.Plugin
	provisionHelper types.ProvisionHelper
	pollInterval    time.Duration
	lock            sync.Mutex
	groups          groups
}

func (p *plugin) validate(config group.Configuration) (groupSettings, error) {

	noSettings := groupSettings{}

	if config.ID == "" {
		return noSettings, errors.New("Group ID must not be blank")
	}

	if config.Role == "" {
		return noSettings, errors.New("Group Role must not be blank")
	}

	groupKind := p.provisionHelper.GroupKind(config.Role)
	if groupKind == types.KindUnknown {
		return noSettings, errors.New("Unrecognized group Role")
	}

	parsed, err := types.ParseProperties(config)
	if err != nil {
		return noSettings, err
	}

	switch groupKind {
	case types.KindStaticIP:
		if parsed.Size != 0 {
			return noSettings, errors.New("Size is unsupported for static IP groups, use IPs instead")
		}
	case types.KindDynamicIP:
		if len(parsed.IPs) != 0 {
			return noSettings, errors.New("IPs is unsupported for dynamic IP groups, use Size instead")
		}
	default:
		return noSettings, errors.New("Unsupported Role type")
	}

	if err := p.provisionHelper.Validate(config, parsed); err != nil {
		return noSettings, err
	}

	pluginMap := p.plugins()
	if pluginMap == nil {
		return noSettings, fmt.Errorf("No instance plugins installed")
	}

	instancePlugin, exists := pluginMap[parsed.InstancePlugin]
	if !exists {
		return noSettings, fmt.Errorf("Instance plugin '%s' is not available", parsed.InstancePlugin)
	}

	if err := instancePlugin.Validate(parsed.InstancePluginProperties); err != nil {
		return noSettings, err
	}

	return groupSettings{
		role:   config.Role,
		plugin: instancePlugin,
		config: parsed,
	}, nil
}

func (p *plugin) WatchGroup(config group.Configuration) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	settings, err := p.validate(config)
	if err != nil {
		return err
	}

	// Two sets of instance tags are used - one for defining membership within the group, and another used to tag
	// newly-created instances.  This allows the scaler to collect and report members of a group which have
	// membership tags but different generation-specific tags.  In practice, we use this the additional tags to
	// attach a config SHA to instances for config change detection.
	scaled := &scaledGroup{
		instancePlugin:  settings.plugin,
		provisionHelper: p.provisionHelper,
		memberTags:      map[string]string{groupTag: string(config.ID)},
	}
	scaled.changeSettings(settings)

	var supervisor Supervisor
	switch groupKind := p.provisionHelper.GroupKind(config.Role); groupKind {
	case types.KindDynamicIP:
		supervisor = NewScalingGroup(scaled, settings.config.Size, p.pollInterval)
	case types.KindStaticIP:
		supervisor = NewQuorum(scaled, settings.config.IPs, p.pollInterval)
	default:
		panic("Unhandled Role type")
	}

	if _, exists := p.groups.get(config.ID); exists {
		return fmt.Errorf("Already watching group '%s'", config.ID)
	}

	p.groups.put(config.ID, &groupContext{supervisor: supervisor, scaled: scaled, settings: settings})

	// TODO(wfarner): Consider changing Run() to not block.
	go supervisor.Run()
	log.Infof("Watching group '%v'", config.ID)

	return nil
}

func (p *plugin) UnwatchGroup(id group.ID) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	grp, exists := p.groups.get(id)
	if !exists {
		return fmt.Errorf("Group '%s' is not being watched", id)
	}

	grp.supervisor.Stop()

	p.groups.del(id)
	log.Infof("Stopped watching group '%s'", id)
	return nil
}

func (p *plugin) InspectGroup(id group.ID) (group.Description, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	context, exists := p.groups.get(id)
	if !exists {
		return group.Description{}, fmt.Errorf("Group '%s' is not being watched", id)
	}

	instances, err := context.scaled.List()
	if err != nil {
		return group.Description{}, err
	}

	return group.Description{Instances: instances}, nil
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

func (p *plugin) planUpdate(id group.ID, updatedSettings groupSettings) (updatePlan, error) {

	context, exists := p.groups.get(id)
	if !exists {
		return nil, fmt.Errorf("Group '%s' is not being watched", id)
	}

	if context.settings.role != updatedSettings.role {
		return nil, errors.New("A group's role cannot be changed")
	}

	return context.supervisor.PlanUpdate(context.scaled, context.settings, updatedSettings)
}

func (p *plugin) DescribeUpdate(updated group.Configuration) (string, error) {
	updatedSettings, err := p.validate(updated)
	if err != nil {
		return "", err
	}

	plan, err := p.planUpdate(updated.ID, updatedSettings)
	if err != nil {
		return "", err
	}

	return plan.Explain(), nil
}

func (p *plugin) initiateUpdate(id group.ID, updatedSettings groupSettings) (updatePlan, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	plan, err := p.planUpdate(id, updatedSettings)
	if err != nil {
		return nil, err
	}

	grp, _ := p.groups.get(id)
	if grp.getUpdate() != nil {
		return nil, errors.New("Update already in progress for this group")
	}

	grp.setUpdate(plan)
	grp.changeSettings(updatedSettings)
	log.Infof("Executing update plan for '%s': %s", id, plan.Explain())
	return plan, nil
}

func (p *plugin) UpdateGroup(updated group.Configuration) error {
	updatedSettings, err := p.validate(updated)
	if err != nil {
		return err
	}

	plan, err := p.initiateUpdate(updated.ID, updatedSettings)
	if err != nil {
		return err
	}

	err = plan.Run(p.pollInterval)
	log.Infof("Finished updating group %s", updated.ID)
	return err
}

func (p *plugin) StopUpdate(gid group.ID) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	grp, exists := p.groups.get(gid)
	if !exists {
		return fmt.Errorf("Group '%s' is not being watched", gid)
	}
	update := grp.getUpdate()
	if update == nil {
		return fmt.Errorf("Group '%s' is not being updated", gid)
	}

	grp.setUpdate(nil)
	update.Stop()

	return nil
}

func (p *plugin) DestroyGroup(gid group.ID) error {
	p.lock.Lock()

	context, exists := p.groups.get(gid)
	if !exists {
		p.lock.Unlock()
		return fmt.Errorf("Group '%s' is not being watched", gid)
	}

	// The lock is released before performing blocking operations.
	p.groups.del(gid)
	p.lock.Unlock()

	context.supervisor.Stop()
	descriptions, err := context.scaled.List()
	if err != nil {
		return err
	}

	for _, desc := range descriptions {
		context.scaled.Destroy(desc.ID)
	}

	return nil
}
