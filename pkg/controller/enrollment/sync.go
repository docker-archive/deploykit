package enrollment

import (
	"fmt"

	enrollment "github.com/docker/infrakit/pkg/controller/enrollment/types"
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

		return desc.Instances, nil
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

func (l *enroller) getSourceKeySelectorTemplate() (*template.Template, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.options.SourceKeySelector != "" {
		if l.sourceKeySelectorTemplate == nil {
			t, err := enrollment.TemplateFrom([]byte(l.options.SourceKeySelector))
			if err != nil {
				return nil, err
			}
			l.sourceKeySelectorTemplate = t
		}
	}

	return l.sourceKeySelectorTemplate, nil
}

func (l *enroller) getEnrollmentKeySelectorTemplate() (*template.Template, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.options.EnrollmentKeySelector != "" {
		if l.enrollmentKeySelectorTemplate == nil {
			t, err := enrollment.TemplateFrom([]byte(l.options.EnrollmentKeySelector))
			if err != nil {
				return nil, err
			}
			l.enrollmentKeySelectorTemplate = t
		}
	}

	return l.enrollmentKeySelectorTemplate, nil
}

func (l *enroller) getEnrollmentPropertiesTemplate() (*template.Template, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.properties.Instance.Properties != nil {
		if l.enrollmentPropertiesTemplate == nil {
			t, err := enrollment.TemplateFrom(l.properties.Instance.Properties.Bytes())
			if err != nil {
				return nil, err
			}
			l.enrollmentPropertiesTemplate = t
		}
	}

	return l.enrollmentPropertiesTemplate, nil
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
	sourceKeyFunc := func(d instance.Description) (string, error) {

		t, err := l.getSourceKeySelectorTemplate()
		if err != nil {
			return "", err
		}
		if t != nil {
			view, err := t.Render(d)
			if err != nil {
				return "", err
			}
			return view, nil
		}

		return string(d.ID), nil
	}

	// If specified, use the given enrollment selectior to get the index key;
	// else check for the labels so that we can even support 'importing'
	// out-of-band created enrollment records
	enrolledKeyFunc := func(d instance.Description) (string, error) {

		t, err := l.getEnrollmentKeySelectorTemplate()
		if err != nil {
			return "", err
		}
		if t == nil {
			if v, has := d.Tags["infrakit.enrollment.sourceID"]; has {
				return v, nil
			}
			return "", fmt.Errorf("not-matched:%v", d.ID)
		}
		view, err := t.Render(d)
		if err != nil {
			return "", err
		}
		return view, nil

	}

	// compute the delta required to make enrolled look like source
	add, remove, _ := Delta(
		instance.Descriptions(source), sourceKeyFunc,
		instance.Descriptions(enrolled), enrolledKeyFunc,
	)

	log.Info("Computed delta", "add", add, "remove", remove)

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
		spec := instance.Spec{
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
			continue // get them next time...
		}
	}
	return nil
}

// buildProperties for calling enrollment / Provision
func (l *enroller) buildProperties(d instance.Description) (*types.Any, error) {
	t, err := l.getEnrollmentPropertiesTemplate()
	if err != nil {
		return nil, err
	}
	if t == nil {
		return types.AnyValue(d)
	}
	view, err := t.Render(d)
	if err != nil {
		return nil, err
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
