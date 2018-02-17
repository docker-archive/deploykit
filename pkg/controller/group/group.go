package group

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	group_types "github.com/docker/infrakit/pkg/controller/group/types"
	logutil "github.com/docker/infrakit/pkg/log"
	plugin_base "github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	debugV = logutil.V(300)
)

var log = logutil.New("module", "plugin/group")

// InstancePluginLookup helps with looking up an instance plugin by name
type InstancePluginLookup func(plugin_base.Name) (instance.Plugin, error)

// FlavorPluginLookup helps with looking up a flavor plugin by name
type FlavorPluginLookup func(plugin_base.Name) (flavor.Plugin, error)

// NewGroupPlugin creates a new group plugin.
// The LogicalID is optional.  It is set when we want to make sure a self-managing cluster manager
// that is running this group plugin doesn't end up terminating itself during a rolling update.
func NewGroupPlugin(
	instancePlugins InstancePluginLookup,
	flavorPlugins FlavorPluginLookup,
	options group_types.Options) group.Plugin {

	return &gController{
		instancePlugins: instancePlugins,
		flavorPlugins:   flavorPlugins,
		options:         options,
		pollInterval:    options.PollInterval.Duration(),
		maxParallelNum:  options.MaxParallelNum,
		groups:          groups{byID: map[group.ID]*groupContext{}},
		self:            options.Self,
	}
}

type gController struct {
	options group_types.Options

	self            *instance.LogicalID
	instancePlugins InstancePluginLookup
	flavorPlugins   FlavorPluginLookup
	pollInterval    time.Duration
	maxParallelNum  uint
	lock            sync.RWMutex
	groups          groups
}

func (p *gController) CommitGroup(config group.Spec, pretend bool) (string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	settings, err := p.validate(config)
	if err != nil {
		return "", err
	}

	settings.self = p.self // need this logicalID of the running node to prevent destroying self.
	settings.options = p.options

	log.Info("Committing", "groupID", config.ID, "pretend", pretend)

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
				log.Info("Executing update plan",
					"groupID", config.ID,
					"updating", settings.config.Updating,
					"plan", updatePlan.Explain())
				if err := updatePlan.Run(p.pollInterval, settings.config.Updating); err != nil {
					log.Error("Update failed", "groupID", config.ID, "err", err)
				} else {
					log.Info("Convergence", "groupID", config.ID)
				}
				context.setUpdate(nil)
			}()
		}

		return updatePlan.Explain(), nil
	}

	scaled := &scaledGroup{
		settings:   settings,
		memberTags: map[string]string{group.GroupTag: string(config.ID)},
	}

	var supervisor Supervisor
	if settings.config.Allocation.Size != 0 {
		supervisor = NewScalingGroup(config.ID, scaled, settings.config.Allocation.Size, p.pollInterval, p.maxParallelNum)
	} else if len(settings.config.Allocation.LogicalIDs) > 0 {
		supervisor = NewQuorum(config.ID, scaled, settings.config.Allocation.LogicalIDs, p.pollInterval)
	} else {
		panic("Invalid empty allocation method")
	}

	scaled.supervisor = supervisor
	if !pretend {
		p.groups.put(config.ID, &groupContext{supervisor: supervisor, scaled: scaled, settings: settings})
		go supervisor.Run()
	}

	return fmt.Sprintf("Managing %d instances", supervisor.Size()), nil
}

func (p *gController) doFree(id group.ID) (*groupContext, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	grp, exists := p.groups.get(id)
	if !exists {
		return nil, fmt.Errorf("Group '%s' is not being watched", id)
	}

	grp.stopUpdating()
	grp.supervisor.Stop()
	p.groups.del(id)

	log.Info("Ignored", "groupID", id)
	return grp, nil
}

func (p *gController) FreeGroup(id group.ID) error {
	_, err := p.doFree(id)
	return err
}

func (p *gController) DescribeGroup(id group.ID) (group.Description, error) {
	// TODO(wfarner): Include details about any in-flight updates.

	// The groups.get will do a read lock on the list of groups.
	// We don't want to lock the entire gController for a describe group
	// when the describe may take a long time.
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

func (p *gController) DestroyGroup(gid group.ID) error {
	context, err := p.doFree(gid)

	if context != nil {
		descriptions, err := context.scaled.List()
		if err != nil {
			return err
		}
		// Ensure that the current node is last
		sort.Sort(sortByID{list: descriptions, settings: &context.settings})
		for _, desc := range descriptions {
			context.scaled.Destroy(desc, instance.Termination)
		}
	}

	return err
}

func (p *gController) Size(gid group.ID) (size int, err error) {
	var all []group.Spec
	all, err = p.InspectGroups()
	if err != nil {
		return
	}
	for _, gg := range all {
		if gg.ID == gid {
			g, err := group_types.ParseProperties(gg)
			if err != nil {
				return 0, err
			}

			if s := len(g.Allocation.LogicalIDs); s > 0 {
				return s, nil
			}
			return int(g.Allocation.Size), nil
		}
	}
	err = fmt.Errorf("group %v not found", gid)
	return
}

func (p *gController) SetSize(gid group.ID, size int) (err error) {
	if size < 0 {
		return fmt.Errorf("size cannot be negative")
	}
	var all []group.Spec
	all, err = p.InspectGroups()
	if err != nil {
		return
	}
	for _, gg := range all {
		if gg.ID == gid {
			g, err := group_types.ParseProperties(gg)
			if err != nil {
				return err
			}

			if s := len(g.Allocation.LogicalIDs); s > 0 {
				return fmt.Errorf("cannot set size if logic ids are explicitly set")
			}
			g.Allocation.Size = uint(size)
			gg.Properties = types.AnyValueMust(g)
			_, err = p.CommitGroup(gg, false)
			return err
		}
	}
	err = fmt.Errorf("group not found %v", gid)
	return
}

type instancesErr []string

func (e instancesErr) Error() string {
	return strings.Join(e, ",")
}

func (p *gController) DestroyInstances(gid group.ID, toDestroy []instance.ID) error {
	log.Debug("Destorying instances", "gid", gid, "targets", toDestroy)

	context, exists := p.groups.get(gid)
	if !exists {
		return fmt.Errorf("Group '%s' is not being watched", gid)
	}

	instances, err := context.scaled.List()
	if err != nil {
		return err
	}

	// build index by instance id
	index := map[instance.ID]instance.Description{}
	for _, inst := range instances {
		index[inst.ID] = inst
	}

	missing := []string{}
	targets := []instance.Description{}
	for _, inst := range toDestroy {
		if desc, has := index[inst]; !has {
			missing = append(missing, string(inst))
		} else {
			targets = append(targets, desc)
		}
	}

	// tell the group to pause before we start killing the instances
	log.Debug("pausing before destroy instances")
	context.stopUpdating()
	log.Debug("paused")

	// kill the instances
	for _, target := range targets {
		if err := context.scaled.Destroy(target, instance.Termination); err != nil {
			return err
		}
	}
	// update the spec to lower count
	sizeSpec, err := p.Size(gid)
	if err != nil {
		return err
	}

	return p.SetSize(gid, sizeSpec-len(toDestroy)) // this will commit the change and watch again
}

func (p *gController) InspectGroups() ([]group.Spec, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
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
	Run(pollInterval time.Duration, updating group_types.Updating) error
	Stop()
}

type noopUpdate struct {
}

func (n noopUpdate) Explain() string {
	return "Noop"
}

func (n noopUpdate) Run(_ time.Duration, _ group_types.Updating) error {
	return nil
}

func (n noopUpdate) Stop() {
}

func (p *gController) validate(config group.Spec) (groupSettings, error) {

	noSettings := groupSettings{}

	if config.ID == "" {
		return noSettings, errors.New("Group ID must not be blank")
	}

	parsed, err := group_types.ParseProperties(config)
	if err != nil {
		return noSettings, err
	}

	// Validate Allocation
	if parsed.Allocation.Size == 0 &&
		(parsed.Allocation.LogicalIDs == nil || len(parsed.Allocation.LogicalIDs) == 0) {

		return noSettings, errors.New("Allocation must not be blank")
	}
	if parsed.Allocation.Size > 0 && parsed.Allocation.LogicalIDs != nil && len(parsed.Allocation.LogicalIDs) > 0 {
		return noSettings, errors.New("Only one Allocation method may be used")
	}

	// Validate Updating
	if parsed.Updating.Count > 0 && parsed.Updating.Duration.Duration() > time.Duration(0) {
		return noSettings, errors.New("Only one Updating method may be used")
	}

	// Validate Flavor plugin
	flavorPlugin, err := p.flavorPlugins(parsed.Flavor.Plugin)
	if err != nil {
		return noSettings, fmt.Errorf("Failed to find Flavor plugin '%s':%v", parsed.Flavor.Plugin, err)
	}
	if err := flavorPlugin.Validate(parsed.Flavor.Properties, parsed.Allocation); err != nil {
		return noSettings, err
	}

	// Validate instance plugin
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

// isSelf returns true if the configured "self" LogicalID Option matches the
// either the given instance's LogicalID or the associated logical ID tag
func isSelf(inst instance.Description, settings groupSettings) bool {
	if settings.self != nil {
		if inst.LogicalID != nil && *inst.LogicalID == *settings.self {
			return true
		}
		if v, has := inst.Tags[instance.LogicalIDTag]; has {
			return string(*settings.self) == v
		}
	}
	return false
}
