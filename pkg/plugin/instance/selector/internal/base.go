package internal

import (
	"sort"
	"strings"

	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	instance_rpc "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"

	. "github.com/docker/infrakit/pkg/plugin/instance/selector"
)

var log = logutil.New("module", "plugin/instance/selector")

type Base struct {
	Plugins    func() discovery.Plugins
	Choices    []Choice
	SelectFunc func(instance.Spec, []Choice, func(Choice) instance.Plugin) (Choice, error)
}

func instancePlugin(plugins map[string]*plugin.Endpoint, name plugin.Name) instance.Plugin {
	lookup, _ := name.GetLookupAndType()
	if endpoint, has := plugins[lookup]; has {
		if p, err := instance_rpc.NewClient(name, endpoint.Address); err == nil {
			return p
		} else {
			log.Warn("not an instance plugin", "name", name, "endpoint", endpoint)
		}
	}
	return nil
}

func (b *Base) selectOne(spec instance.Spec) (match Choice, p instance.Plugin, err error) {
	all := map[plugin.Name]instance.Plugin{}
	var matchByLogicalID *Choice
	b.visit(func(c Choice, p instance.Plugin) error {
		all[c.Name] = p
		if spec.LogicalID != nil && c.HasLogicalID(*spec.LogicalID) {
			found := c // allocate a copy
			matchByLogicalID = &found
		}
		return nil
	})

	if matchByLogicalID != nil {
		match = *matchByLogicalID
	} else {
		match, err = b.SelectFunc(spec, b.Choices, func(c Choice) instance.Plugin { return all[c.Name] })
		if err != nil {
			return
		}
	}

	p = all[match.Name]
	return
}

func (b *Base) visit(f func(Choice, instance.Plugin) error) error {
	plugins, err := b.Plugins().List()
	if err != nil {
		return err
	}

	for _, choice := range b.Choices {
		instancePlugin := instancePlugin(plugins, choice.Name)
		if instancePlugin == nil {
			continue
		}
		if err := f(choice, instancePlugin); err != nil {
			return err
		}
	}
	return nil
}

func (b *Base) doAll(work func(instance.Plugin) error) error {

	errs := make(chan error, len(b.Choices))
	done := make(chan struct{})

	err := b.visit(func(c Choice, p instance.Plugin) error {
		go func() {
			if localErr := work(p); localErr != nil {
				errs <- localErr
				return
			}
			close(done)
		}()
		return nil
	})

	if err != nil {
		return err
	}

	collect := errorGroup{}
	for i := 0; i < len(b.Choices); i++ {
		select {
		case pErr := <-errs:
			collect.Add(pErr)
		case <-done:
		}
	}
	if len(collect) == len(b.Choices) {
		return collect
	}
	return nil
}

// Validate performs local validation on a provision request.
func (b *Base) Validate(req *types.Any) error {
	return b.visit(func(c Choice, p instance.Plugin) error {
		return p.Validate(req)
	})
}

// Provision creates a new instance based on the spec.
func (b *Base) Provision(spec instance.Spec) (*instance.ID, error) {
	match, selected, err := b.selectOne(spec)
	log.Debug("provision", "match", match, "err", err)
	if err != nil {
		return nil, err
	}
	return selected.Provision(spec)
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (b *Base) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	// Loop through all the choices and aggregate all the instances

	keys := []string{}
	uniques := map[string]instance.Description{}

	err := b.visit(func(c Choice, p instance.Plugin) error {
		instances, err := p.DescribeInstances(tags, properties)
		if err != nil {
			return err
		}
		for _, instance := range instances {
			keys = append(keys, string(instance.ID))
			uniques[string(instance.ID)] = instance
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(keys)
	result := []instance.Description{}
	for _, k := range keys {
		result = append(result, uniques[k])
	}
	return result, nil
}

// Label labels the instance
func (b *Base) Label(inst instance.ID, labels map[string]string) error {
	return b.doAll(func(p instance.Plugin) error {
		return p.Label(inst, labels)
	})
}

// Destroy terminates an existing instance.
func (b *Base) Destroy(inst instance.ID, context instance.Context) error {
	return b.doAll(func(p instance.Plugin) error {
		return p.Destroy(inst, context)
	})
}

type errorGroup []error

func (g *errorGroup) Add(e error) {
	*g = append(*g, e)
}
func (g errorGroup) Error() string {
	m := []string{}
	for _, e := range g {
		m = append(m, e.Error())
	}
	return strings.Join(m, ",")
}
