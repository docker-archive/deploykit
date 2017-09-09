package enrollment

import (
	"fmt"

	"github.com/docker/infrakit/pkg/plugin"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	instance_rpc "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

func (l *enroller) getSourceInstances() ([]instance.Description, error) {
	list, err := l.properties.List.InstanceDescriptions()
	if err != nil {

		pn, err := l.properties.List.GroupPlugin()
		if err != nil {
			return nil, fmt.Errorf("no list source specified")
		}

		// we have a plugin name. -- eg. us-east/workers
		lookup, gid := pn.GetLookupAndType()

		gp, err := l.getGroupPlugin(plugin.Name(lookup))
		if err != nil {
			return nil, fmt.Errorf("cannot connect to group %v", pn)
		}

		desc, err := gp.DescribeGroup(group.ID(gid))
		if err != nil {
			return nil, err
		}

		list = desc.Instances
	}
	return list, err
}

func (l *enroller) getEnrolledInstances() ([]instance.Description, error) {
	instancePlugin, err := l.getInstancePlugin(l.properties.Instance.Plugin)
	if err != nil {
		return nil, err
	}

	return instancePlugin.DescribeInstances(l.properties.Instance.Labels, true)
}

// run one synchronization round
func (l *enroller) sync() error {

	source, err := l.getSourceInstances()
	if err != nil {
		log.Error("Error getting sources. No action", "err", err)
		return nil
	}

	enrolled, err := l.getEnrolledInstances()
	if err != nil {
		log.Error("Error getting enrollment. No action", "err", err)
		return nil
	}

	// We need to compute a projection for each one of the vectors and compare
	// them.  This is because instance IDs from the respective lists are likely
	// to be different.  Instead there's a join key / common attribute somewhere
	// embedded in the Description.Properties.
	sourceKeyFunc := func(d instance.Description) string {
		// TODO render a template
		return string(d.ID)
	}
	enrolledKeyFunc := func(d instance.Description) string {
		// TODO render a template
		if d.LogicalID != nil {
			return string(*d.LogicalID)
		}
		return string(d.ID)
	}

	add, remove, _ := Delta(instance.Descriptions(enrolled), enrolledKeyFunc, instance.Descriptions(source), sourceKeyFunc)

	log.Debug("Computed delta", "add", add, "remove", remove)

	instancePlugin, err := l.getInstancePlugin(l.properties.Instance.Plugin)
	if err != nil {
		log.Error("cannot get instance plugin", "err", err)
		return err
	}

	for _, n := range add {

		props, err := l.buildProperties(n)
		if err != nil {
			log.Error("Cannot bulid properties to enroll", "err", err, "description", n)
			continue
		}

		logicalID := instance.LogicalID(string(n.ID))
		spec := instance.Spec{
			LogicalID: &logicalID,
			// TODO - render a template using the value n as context?
			Properties: props,
			Tags:       l.labels(n),
		}
		_, err = instancePlugin.Provision(spec)
		if err != nil {
			log.Error("Failed to create enrollment", "err", err, "spec", spec)
		}
	}

	for _, n := range remove {
		err = instancePlugin.Destroy(n.ID, instance.Termination)
		if err != nil {
			log.Error("Failed to remove enrollment", "err", err, "id", n.ID)
		}
	}
	return nil
}

// buildProperties for calling enrollment / Provision
func (l *enroller) buildProperties(d instance.Description) (props *types.Any, err error) {
	props = l.properties.Instance.Properties

	if props == nil {
		return
	}

	t, e := template.NewTemplate(props.String(), template.Options{MultiPass: false})
	if e != nil {
		err = e
		return
	}

	view, e := t.Render(d)
	if e != nil {
		err = e
		return
	}
	return types.AnyString(view), nil
}

func (l *enroller) labels(n instance.Description) map[string]string {
	labels := l.properties.Instance.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels["infrakit.enrollment.sourceID"] = string(n.ID)
	labels["infrakit.enrollment.name"] = l.spec.Metadata.Name
	return labels
}

// destroy all the instances in the enrolled instance plugin
func (l *enroller) destroy() error {

	instancePlugin, err := l.getInstancePlugin(l.properties.Instance.Plugin)
	if err != nil {
		return err
	}

	// TODO -- add retry loop here to let Terminate block until everything is cleaned up.
	{
		l.lock.Lock()

		enrolled, err := l.getEnrolledInstances()
		if err != nil {
			return err
		}

		for _, n := range enrolled {
			err = instancePlugin.Destroy(n.ID, instance.Termination)
			if err != nil {
				log.Error("failed to destroy instance. retry next cycle.", "id", n.ID)
			}
		}

		defer l.lock.Unlock()
	}

	return nil
}

func (l *enroller) getGroupPlugin(name plugin.Name) (group.Plugin, error) {
	if l.groupPlugin != nil {
		return l.groupPlugin, nil
	}
	return l.connectGroupPlugin(name)
}

func (l *enroller) connectGroupPlugin(name plugin.Name) (group.Plugin, error) {
	l.lock.Lock()
	defer l.lock.Unlock()
	endpoint, err := l.plugins().Find(name)
	if err != nil {
		return nil, err
	}
	return group_rpc.NewClient(endpoint.Address)
}

func (l *enroller) getInstancePlugin(name plugin.Name) (instance.Plugin, error) {
	if l.instancePlugin != nil {
		return l.instancePlugin, nil
	}
	return l.connectInstancePlugin(name)
}

func (l *enroller) connectInstancePlugin(name plugin.Name) (instance.Plugin, error) {
	l.lock.Lock()
	defer l.lock.Unlock()
	endpoint, err := l.plugins().Find(name)
	if err != nil {
		return nil, err
	}
	return instance_rpc.NewClient(name, endpoint.Address)
}
