package internal

import (
	"fmt"
	"os"
	"testing"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/plugin/instance/selector"
	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/run"
	"github.com/docker/infrakit/pkg/spi/instance"
	instance_test "github.com/docker/infrakit/pkg/testing/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func startInstancePlugin(t *testing.T, dir string, name plugin.Name,
	p instance.Plugin) (server.Stoppable, <-chan struct{}) {

	s, running, err := run.ServeRPC(plugin.Transport{Name: name, Dir: dir}, nil,
		map[run.PluginCode]interface{}{run.Instance: p})
	require.NoError(t, err)
	return s, running
}

func TestGetInstancePluginClientVisit(t *testing.T) {

	dir := os.TempDir()

	n1 := plugin.Name("us-west-2a")
	n2 := plugin.Name("us-west-2b")

	p1 := &instance_test.Plugin{}
	p2 := &instance_test.Plugin{}

	s1, _ := startInstancePlugin(t, dir, n1, p1)
	s2, _ := startInstancePlugin(t, dir, n2, p2)

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
		Plugins: discovery.Must(local.NewPluginDiscoveryWithDir(dir)),
		Choices: options,
	}

	m, err := b.Plugins().List()
	require.NoError(t, err)

	require.NotNil(t, instancePlugin(m, options[0].Name))
	require.NotNil(t, instancePlugin(m, options[1].Name))

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

	s1.Stop()
	s2.Stop()
}

func TestDoAll(t *testing.T) {

	dir := os.TempDir()

	n1 := plugin.Name("us-west-2a")
	n2 := plugin.Name("us-west-2b")

	called1 := make(chan struct{})
	called2 := make(chan struct{})
	p1 := &instance_test.Plugin{
		DoProvision: func(s instance.Spec) (*instance.ID, error) {
			close(called1)
			return nil, nil
		},
	}
	p2 := &instance_test.Plugin{
		DoProvision: func(s instance.Spec) (*instance.ID, error) {
			close(called2)
			return nil, nil
		},
	}

	s1, _ := startInstancePlugin(t, dir, n1, p1)
	s2, _ := startInstancePlugin(t, dir, n2, p2)

	options := selector.Options{
		selector.Choice{
			Name: plugin.Name("us-west-2a"),
		},
		selector.Choice{
			Name: plugin.Name("us-west-2b"),
		},
	}

	b := &Base{
		Plugins: discovery.Must(local.NewPluginDiscoveryWithDir(dir)),
		Choices: options,
	}

	require.NoError(t, b.doAll(len(options),
		func(i instance.Plugin) error {
			_, err := i.Provision(instance.Spec{})
			return err
		}))

	<-called1
	<-called2

	s1.Stop()
	s2.Stop()
}
