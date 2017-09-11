package internal

import (
	"fmt"
	"testing"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/instance/selector"
	"github.com/docker/infrakit/pkg/spi/instance"
	instance_testing "github.com/docker/infrakit/pkg/testing/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func mustPlugin(p instance.Plugin, err error) instance.Plugin {
	if err != nil {
		panic(err)
	}
	return p
}

type testDiscovery map[string]*plugin.Endpoint

func (td testDiscovery) List() (map[string]*plugin.Endpoint, error) {
	return map[string]*plugin.Endpoint(td), nil
}
func (td testDiscovery) Find(n plugin.Name) (*plugin.Endpoint, error) {
	return td[string(n)], nil
}

func TestVisit(t *testing.T) {

	n1 := plugin.Name("us-west-2a")
	n2 := plugin.Name("us-west-2b")

	p1 := &instance_testing.Plugin{}
	p2 := &instance_testing.Plugin{}

	options := selector.Options{
		selector.Choice{
			Name: plugin.Name("us-west-2a"),
			Instances: []instance.LogicalID{
				instance.LogicalID("10.20.100.101"),
			},
			Affinity: types.AnyValueMust(map[string]interface{}{
				"weight": 20,
			}),
		},
		selector.Choice{
			Name: plugin.Name("us-west-2b"),
			Instances: []instance.LogicalID{
				instance.LogicalID("10.20.100.102"),
			},
			Affinity: types.AnyValueMust(map[string]interface{}{
				"weight": 10,
			}),
		},
	}

	b := &Base{
		Plugins: func() discovery.Plugins {
			return testDiscovery{
				string(n1): &plugin.Endpoint{Name: string(n1), Address: string(n1)},
				string(n2): &plugin.Endpoint{Name: string(n2), Address: string(n2)},
			}
		},
		Choices: options,
		PluginClientFunc: func(n plugin.Name) (instance.Plugin, error) {
			switch n {
			case n1:
				return p1, nil
			case n2:
				return p2, nil
			}
			return nil, nil
		},
	}

	_, err := b.Plugins().List()
	require.NoError(t, err)
	require.Equal(t, p1, mustPlugin(b.PluginClientFunc(options[0].Name)))
	require.Equal(t, p2, mustPlugin(b.PluginClientFunc(options[1].Name)))

	// Check error handling
	require.Error(t, b.visit(
		func(c selector.Choice, i instance.Plugin) error {
			return fmt.Errorf("err")
		}))

	visited := []plugin.Name{}
	require.NoError(t, b.visit(
		func(c selector.Choice, i instance.Plugin) error {
			require.NotNil(t, i)
			visited = append(visited, c.Name)
			return nil
		}))
	require.Equal(t, []plugin.Name{options[0].Name, options[1].Name}, visited)
}

func TestDoAll(t *testing.T) {

	n1 := plugin.Name("us-west-2a")
	n2 := plugin.Name("us-west-2b")

	called1 := make(chan struct{})
	called2 := make(chan struct{})
	p1 := &instance_testing.Plugin{
		DoProvision: func(s instance.Spec) (*instance.ID, error) {
			close(called1)
			return nil, nil
		},
	}
	p2 := &instance_testing.Plugin{
		DoProvision: func(s instance.Spec) (*instance.ID, error) {
			close(called2)
			return nil, nil
		},
	}

	options := selector.Options{
		selector.Choice{
			Name: plugin.Name("us-west-2a"),
		},
		selector.Choice{
			Name: plugin.Name("us-west-2b"),
		},
	}

	b := &Base{
		Plugins: func() discovery.Plugins {
			return testDiscovery{
				string(n1): &plugin.Endpoint{Name: string(n1), Address: string(n1)},
				string(n2): &plugin.Endpoint{Name: string(n2), Address: string(n2)},
			}
		},
		Choices: options,
		PluginClientFunc: func(n plugin.Name) (instance.Plugin, error) {
			switch n {
			case n1:
				return p1, nil
			case n2:
				return p2, nil
			}
			return nil, nil
		},
	}

	require.NoError(t, b.doAll(len(options),
		func(i instance.Plugin) error {
			_, err := i.Provision(instance.Spec{})
			return err
		}))

	<-called1
	<-called2
}

func TestDescribeInstances(t *testing.T) {

	n1 := plugin.Name("us-west-2a")
	n2 := plugin.Name("us-west-2b")

	instances1 := []instance.Description{
		{ID: instance.ID("us-west-2a-1")},
		{ID: instance.ID("us-west-2a-2")},
		{ID: instance.ID("us-west-2a-3")},
		{ID: instance.ID("us-west-2a-4")},
	}
	instances2 := []instance.Description{
		{ID: instance.ID("us-west-2b-1")},
		{ID: instance.ID("us-west-2b-2")},
		{ID: instance.ID("us-west-2b-3")},
	}

	p1 := &instance_testing.Plugin{}
	p2 := &instance_testing.Plugin{}

	options := selector.Options{
		selector.Choice{
			Name: plugin.Name("us-west-2a"),
		},
		selector.Choice{
			Name: plugin.Name("us-west-2b"),
		},
	}

	b := &Base{
		Plugins: func() discovery.Plugins {
			return testDiscovery{
				string(n1): &plugin.Endpoint{Name: string(n1), Address: string(n1)},
				string(n2): &plugin.Endpoint{Name: string(n2), Address: string(n2)},
			}
		},
		Choices: options,
		PluginClientFunc: func(n plugin.Name) (instance.Plugin, error) {
			switch n {
			case n1:
				return p1, nil
			case n2:
				return p2, nil
			}
			return nil, nil
		},
	}

	qtags := map[string]string{
		"group": "foo",
		"sha":   "xyz",
	}
	p1.DoDescribeInstances = func(tags map[string]string, properties bool) ([]instance.Description, error) {
		require.Equal(t, qtags, tags)
		return instances1, nil
	}
	p2.DoDescribeInstances = func(tags map[string]string, properties bool) ([]instance.Description, error) {
		require.Equal(t, qtags, tags)
		return instances2, nil
	}

	all, err := b.DescribeInstances(qtags, false)
	require.NoError(t, err)
	require.Equal(t, []instance.Description{
		{ID: instance.ID("us-west-2a-1")},
		{ID: instance.ID("us-west-2a-2")},
		{ID: instance.ID("us-west-2a-3")},
		{ID: instance.ID("us-west-2a-4")},
		{ID: instance.ID("us-west-2b-1")},
		{ID: instance.ID("us-west-2b-2")},
		{ID: instance.ID("us-west-2b-3")},
	}, all)

	// Now one of them will throw an error
	p2.DoDescribeInstances = func(tags map[string]string, properties bool) ([]instance.Description, error) {
		return nil, fmt.Errorf("error")
	}
	_, err = b.DescribeInstances(qtags, false)
	require.Error(t, err)
}

func TestValidate(t *testing.T) {

	n1 := plugin.Name("us-west-2a")
	n2 := plugin.Name("us-west-2b")

	p1 := &instance_testing.Plugin{}
	p2 := &instance_testing.Plugin{}

	options := selector.Options{
		selector.Choice{
			Name: plugin.Name("us-west-2a"),
		},
		selector.Choice{
			Name: plugin.Name("us-west-2b"),
		},
	}

	b := &Base{
		Plugins: func() discovery.Plugins {
			return testDiscovery{
				string(n1): &plugin.Endpoint{Name: string(n1), Address: string(n1)},
				string(n2): &plugin.Endpoint{Name: string(n2), Address: string(n2)},
			}
		},
		Choices: options,
		PluginClientFunc: func(n plugin.Name) (instance.Plugin, error) {
			switch n {
			case n1:
				return p1, nil
			case n2:
				return p2, nil
			}
			return nil, nil
		},
	}

	p1.DoValidate = func(req *types.Any) error {
		return nil
	}
	p2.DoValidate = func(req *types.Any) error {
		return fmt.Errorf("error")
	}
	qproperty := map[string]*types.Any{
		"us-west-2a": types.AnyValueMust("foo"),
		"us-west-2b": types.AnyValueMust("baa"),
	}
	err := b.Validate(types.AnyValueMust(qproperty))
	require.NoError(t, err)

	// Now all of them will return error
	p1.DoValidate = func(req *types.Any) error {
		return fmt.Errorf("error")
	}
	p2.DoValidate = func(req *types.Any) error {
		return fmt.Errorf("error")
	}

	require.Error(t, b.Validate(types.AnyValueMust("foo")))
}

func TestLabel(t *testing.T) {

	n1 := plugin.Name("us-west-2a")
	n2 := plugin.Name("us-west-2b")

	p1 := &instance_testing.Plugin{}
	p2 := &instance_testing.Plugin{}

	options := selector.Options{
		selector.Choice{
			Name: plugin.Name("us-west-2a"),
		},
		selector.Choice{
			Name: plugin.Name("us-west-2b"),
		},
	}

	b := &Base{
		Plugins: func() discovery.Plugins {
			return testDiscovery{
				string(n1): &plugin.Endpoint{Name: string(n1), Address: string(n1)},
				string(n2): &plugin.Endpoint{Name: string(n2), Address: string(n2)},
			}
		},
		Choices: options,
		PluginClientFunc: func(n plugin.Name) (instance.Plugin, error) {
			switch n {
			case n1:
				return p1, nil
			case n2:
				return p2, nil
			}
			return nil, nil
		},
	}

	qid := instance.ID("fool")
	qlabels := map[string]string{
		"test": "test",
	}

	p1.DoLabel = func(inst instance.ID, labels map[string]string) error {
		require.Equal(t, qlabels, labels)
		return fmt.Errorf("not found")
	}
	p2.DoLabel = func(inst instance.ID, labels map[string]string) error {
		require.Equal(t, qlabels, labels)
		return nil // labeled ok
	}

	err := b.Label(qid, qlabels)
	require.NoError(t, err)

	// Now all of them will return error
	p1.DoLabel = func(inst instance.ID, labels map[string]string) error {
		require.Equal(t, qlabels, labels)
		return fmt.Errorf("not found")
	}
	p2.DoLabel = func(inst instance.ID, labels map[string]string) error {
		require.Equal(t, qlabels, labels)
		return fmt.Errorf("not found")
	}

	require.Error(t, b.Label(qid, qlabels))
}

func TestDestroy(t *testing.T) {

	n1 := plugin.Name("us-west-2a")
	n2 := plugin.Name("us-west-2b")

	p1 := &instance_testing.Plugin{}
	p2 := &instance_testing.Plugin{}

	options := selector.Options{
		selector.Choice{
			Name: plugin.Name("us-west-2a"),
		},
		selector.Choice{
			Name: plugin.Name("us-west-2b"),
		},
	}

	b := &Base{
		Plugins: func() discovery.Plugins {
			return testDiscovery{
				string(n1): &plugin.Endpoint{Name: string(n1), Address: string(n1)},
				string(n2): &plugin.Endpoint{Name: string(n2), Address: string(n2)},
			}
		},
		Choices: options,
		PluginClientFunc: func(n plugin.Name) (instance.Plugin, error) {
			switch n {
			case n1:
				return p1, nil
			case n2:
				return p2, nil
			}
			return nil, nil
		},
	}

	qid := instance.ID("fool")

	p1.DoDestroy = func(inst instance.ID, context instance.Context) error {
		require.Equal(t, qid, inst)
		return fmt.Errorf("not found")
	}
	p2.DoDestroy = func(inst instance.ID, context instance.Context) error {
		require.Equal(t, qid, inst)
		return nil // labeled ok
	}

	err := b.Destroy(qid, instance.Context{})
	require.NoError(t, err)

	// Now all of them will return error
	p1.DoDestroy = func(inst instance.ID, context instance.Context) error {
		require.Equal(t, qid, inst)
		return fmt.Errorf("not found")
	}
	p2.DoDestroy = func(inst instance.ID, context instance.Context) error {
		require.Equal(t, qid, inst)
		return fmt.Errorf("not found")
	}

	require.Error(t, b.Destroy(qid, instance.Context{}))
}

func TestProvision(t *testing.T) {

	n1 := plugin.Name("us-west-2a")
	n2 := plugin.Name("us-west-2b")

	p1 := &instance_testing.Plugin{}
	p2 := &instance_testing.Plugin{}

	options := selector.Options{
		selector.Choice{
			Name: plugin.Name("us-west-2a"),
		},
		selector.Choice{
			Name: plugin.Name("us-west-2b"),
		},
	}

	b := &Base{
		Plugins: func() discovery.Plugins {
			return testDiscovery{
				string(n1): &plugin.Endpoint{Name: string(n1), Address: string(n1)},
				string(n2): &plugin.Endpoint{Name: string(n2), Address: string(n2)},
			}
		},
		Choices: options,
		PluginClientFunc: func(n plugin.Name) (instance.Plugin, error) {
			switch n {
			case n1:
				return p1, nil
			case n2:
				return p2, nil
			}
			return nil, nil
		},
	}

	qid := instance.ID("foo-instance")
	qlid := instance.LogicalID("logical-foo")
	qproperty := map[string]*types.Any{
		"us-west-2a": types.AnyValueMust("foo"),
		"us-west-2b": types.AnyValueMust("baa"),
	}
	qspec := instance.Spec{LogicalID: &qlid, Properties: types.AnyValueMust(qproperty)}

	b.SelectFunc = func(s instance.Spec, c []selector.Choice,
		f func(selector.Choice) instance.Plugin) (selector.Choice, error) {
		require.Equal(t, qspec, s)
		require.NotNil(t, f)
		return options[1], nil
	}

	p1.DoProvision = func(spec instance.Spec) (*instance.ID, error) {
		panic("shouldn't be here")
	}
	p2.DoProvision = func(spec instance.Spec) (*instance.ID, error) {
		expectSpec := instance.Spec{LogicalID: &qlid, Properties: types.AnyValueMust(qproperty["us-west-2b"])}
		require.Equal(t, expectSpec, spec)
		return &qid, nil // provisioned ok
	}

	id, err := b.Provision(qspec)
	require.NoError(t, err)
	require.Equal(t, qid, *id)

	p2.DoProvision = func(spec instance.Spec) (*instance.ID, error) {
		expectSpec := instance.Spec{LogicalID: &qlid, Properties: types.AnyValueMust(qproperty["us-west-2b"])}
		require.Equal(t, expectSpec, spec)
		return nil, fmt.Errorf("error")
	}

	_, err = b.Provision(qspec)
	require.Error(t, err)
}

func TestErrorGroup(t *testing.T) {
	g := errorGroup{}
	g.Add(fmt.Errorf("a"))
	g.Add(fmt.Errorf("b"))
	g.Add(fmt.Errorf("c"))
	g.Add(fmt.Errorf("d"))
	require.Equal(t, "a,b,c,d", g.Error())
}
