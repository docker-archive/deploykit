package types

import (
	"testing"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	fake "github.com/docker/infrakit/pkg/testing/loadbalancer"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

type fakePlugins map[string]*plugin.Endpoint

func (f fakePlugins) Find(name plugin.Name) (*plugin.Endpoint, error) {
	return f[string(name)], nil
}

func (f fakePlugins) List() (map[string]*plugin.Endpoint, error) {
	panic("fake shouldn't be called here")
}

func TestL4(t *testing.T) {
	vhost := Vhost("test.com")

	properties := Properties{
		{
			Vhost:    vhost,
			L4Plugin: plugin.Name("ingress/elb1"),
		},
	}

	fakeL4 := &fake.L4{}

	expect := map[Vhost]loadbalancer.L4{
		vhost: fakeL4,
	}

	calledFind := make(chan struct{})
	l4 := properties.L4Func(
		func(spec Spec) (loadbalancer.L4, error) {
			close(calledFind)
			require.Equal(t, vhost, spec.Vhost)
			require.Equal(t, "ingress/elb1", string(spec.L4Plugin))
			return fakeL4, nil
		},
	)

	m, err := l4()

	<-calledFind

	require.NoError(t, err)
	require.Equal(t, expect, m)
}

func TestHealthChecks(t *testing.T) {

	properties := Properties{
		{
			Vhost:    Vhost("test.com"),
			L4Plugin: plugin.Name("ingress/elb1"),
			HealthChecks: []HealthCheck{
				{
					Port:            8080,
					Healthy:         1,
					Unhealthy:       10,
					IntervalSeconds: 10,
					TimeoutSeconds:  60,
				},
			},
		},
		{
			Vhost:    Vhost("test2.com"),
			L4Plugin: plugin.Name("ingress/elb2"),
			HealthChecks: []HealthCheck{
				{
					Port:            80,
					Healthy:         1,
					Unhealthy:       10,
					IntervalSeconds: 10,
					TimeoutSeconds:  60,
				},
			},
		},
	}

	m, err := properties.HealthChecks()
	require.NoError(t, err)
	require.Equal(t, map[Vhost][]HealthCheck{
		Vhost("test.com"): {
			{
				Port:            8080,
				Healthy:         1,
				Unhealthy:       10,
				IntervalSeconds: 10,
				TimeoutSeconds:  60,
			},
		},
		Vhost("test2.com"): {
			{
				Port:            80,
				Healthy:         1,
				Unhealthy:       10,
				IntervalSeconds: 10,
				TimeoutSeconds:  60,
			},
		},
	}, m)
}

func TestGroupsInstanceIDs(t *testing.T) {

	properties := Properties{
		{
			Vhost:    Vhost("test.com"),
			L4Plugin: plugin.Name("ingress/elb1"),
			HealthChecks: []HealthCheck{
				{
					Port:            8080,
					Healthy:         1,
					Unhealthy:       10,
					IntervalSeconds: 10,
					TimeoutSeconds:  60,
				},
			},
			Backends: BackendSpec{
				Groups: []group.ID{
					group.ID("managers"),
					group.ID("workers"),
				},
			},
		},
		{
			Vhost:    Vhost("test2.com"),
			L4Plugin: plugin.Name("ingress/elb2"),
			HealthChecks: []HealthCheck{
				{
					Port:            80,
					Healthy:         1,
					Unhealthy:       10,
					IntervalSeconds: 10,
					TimeoutSeconds:  60,
				},
			},
			Backends: BackendSpec{
				Instances: []instance.ID{
					instance.ID("worker1"),
					instance.ID("worker2"),
					instance.ID("worker3"),
					instance.ID("worker4"),
					instance.ID("worker5"),
				},
			},
		},
	}

	m, err := properties.Groups()
	require.NoError(t, err)
	require.Equal(t, map[Vhost][]group.ID{
		Vhost("test.com"): {
			group.ID("managers"),
			group.ID("workers"),
		},
		Vhost("test2.com"): nil,
	}, m)

	mm, err := properties.InstanceIDs()
	require.NoError(t, err)
	require.Equal(t, map[Vhost][]instance.ID{
		Vhost("test.com"): nil,
		Vhost("test2.com"): {
			instance.ID("worker1"),
			instance.ID("worker2"),
			instance.ID("worker3"),
			instance.ID("worker4"),
			instance.ID("worker5"),
		},
	}, mm)
}

func TestRegisterRouteHandler(t *testing.T) {

	RegisterRouteHandler("nil", nil)
	require.Equal(t, 0, len(routeHandlers))

	calledChan := make(chan *types.Any, 1)
	routesChan := make(chan map[Vhost][]loadbalancer.Route, 1)
	f := func(customConfig *types.Any, options Options) (map[Vhost][]loadbalancer.Route, error) {
		calledChan <- customConfig
		return <-routesChan, nil
	}
	RegisterRouteHandler("test", f)
	require.Equal(t, 1, len(routeHandlers))

	vhost := Vhost("test")
	unknownRoutesConfig := types.AnyValueMust(map[string]interface{}{
		"swarmConfig":  "some-config",
		"dockerSocket": "dockerSocket",
	})
	properties := Properties{
		{
			Vhost: vhost,
			RouteSources: map[string]*types.Any{
				"test": unknownRoutesConfig,
			},
		},
	}

	routes := []loadbalancer.Route{
		{ // Port is the TCP port that the backend instance is listening on.
			Port: 8080,
			// Protocol is the network protocol that the load balancer is routing.
			Protocol: loadbalancer.Protocol("http"),
			// LoadBalancerPort is the TCP port that the load balancer is listening on.
			LoadBalancerPort: 8080,
		},
	}
	routesChan <- map[Vhost][]loadbalancer.Route{
		vhost: routes,
	}

	m, err := properties.Routes(Options{})
	require.NoError(t, err)
	require.Equal(t, map[Vhost][]loadbalancer.Route{
		vhost: routes,
	}, m)

	<-calledChan
	close(calledChan)
	close(routesChan)
}
