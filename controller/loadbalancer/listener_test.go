package loadbalancer

import (
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/libmachete/spi/loadbalancer"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestListener(t *testing.T) {
	l, err := newListener("foo", 30000, "http://:80")
	require.NoError(t, err)

	require.Equal(t, loadbalancer.HTTP, l.protocol())
	require.Equal(t, uint32(80), l.extPort())
	require.Equal(t, uint32(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, "default", l.host())

	l, err = newListener("foo", 30000, "http://")
	require.NoError(t, err)

	require.Equal(t, loadbalancer.HTTP, l.protocol())
	require.Equal(t, uint32(80), l.extPort())
	require.Equal(t, uint32(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, "default", l.host())

	l, err = newListener("foo", 30000, "http://localswarm:8080")
	require.NoError(t, err)

	require.Equal(t, loadbalancer.HTTP, l.protocol())
	require.Equal(t, uint32(8080), l.extPort())
	require.Equal(t, uint32(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, "localswarm", l.host())
}

func TestImpliedSwarmPortToUrl(t *testing.T) {
	l, err := impliedSwarmPortToURL("foo", "30000=http://:8080")
	require.Error(t, err) // Because this is the explicit form

	l, err = impliedSwarmPortToURL("foo", "http://:8080")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(8080), l.extPort())
	require.Equal(t, "default", l.host())
	require.Equal(t, uint32(0), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = impliedSwarmPortToURL("foo", "https://")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(443), l.extPort())
	require.Equal(t, "default", l.host())
	require.Equal(t, uint32(0), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTPS, l.protocol())

	l, err = impliedSwarmPortToURL("foo", "http://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(80), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, uint32(0), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = impliedSwarmPortToURL("foo", "HTTP://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(80), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, uint32(0), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = impliedSwarmPortToURL("foo", "tcp://myapp.domain.com:2333")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(2333), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, uint32(0), l.SwarmPort)
	require.Equal(t, loadbalancer.TCP, l.protocol())

	l, err = impliedSwarmPortToURL("foo", "ssl://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(0), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, uint32(0), l.SwarmPort)
	require.Equal(t, loadbalancer.SSL, l.protocol())

	l, err = impliedSwarmPortToURL("foo", "https://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(443), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, uint32(0), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTPS, l.protocol())
}

func TestExplicitSwarmPortToUrl(t *testing.T) {
	l, err := explicitSwarmPortToURL("foo", "http://:8080")
	require.Error(t, err) // Because this is the implicit form

	l, err = explicitSwarmPortToURL("foo", "7000=http://:8080")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(8080), l.extPort())
	require.Equal(t, "default", l.host())
	require.Equal(t, uint32(7000), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = explicitSwarmPortToURL("foo", "8999=https://")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(443), l.extPort())
	require.Equal(t, "default", l.host())
	require.Equal(t, uint32(8999), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTPS, l.protocol())

	l, err = explicitSwarmPortToURL("foo", "80=http://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(80), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, uint32(80), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = explicitSwarmPortToURL("foo", "8088=HTTP://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(80), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, uint32(8088), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = explicitSwarmPortToURL("foo", "7543=tcp://myapp.domain.com:2333")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, uint32(2333), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, uint32(7543), l.SwarmPort)
	require.Equal(t, loadbalancer.TCP, l.protocol())
}

func TestAddListenerToHostMap(t *testing.T) {
	l, err := explicitSwarmPortToURL("foo", "7543=tcp://myapp.domain.com:2333")
	require.NoError(t, err)

	hm := map[string][]*listener{}
	addListenerToHostMap(hm, l)

	require.Equal(t, 1, len(hm))
	require.Equal(t, []*listener{l}, hm["myapp.domain.com"])
}

func TestListenersToPublishImplicitMapping(t *testing.T) {
	s := swarm.Service{}
	s.Spec.Name = "web1"

	l := listenersFromLabel(s, LabelExternalLoadBalancerSpec)
	require.Equal(t, 0, len(l))
	require.NotNil(t, l)

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "http://:8080",
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec)
	require.NotNil(t, l)
	require.Equal(t, 1, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, "default", l[0].host())
	require.Equal(t, loadbalancer.HTTP, l[0].protocol())
	require.Equal(t, uint32(8080), l[0].extPort())

	require.Equal(t, uint32(0), l[0].SwarmPort)  // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid()) // not known yet.

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "http://",
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec)
	require.NotNil(t, l)
	require.Equal(t, 1, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, "default", l[0].host())
	require.Equal(t, loadbalancer.HTTP, l[0].protocol())
	require.Equal(t, uint32(80), l[0].extPort())

	require.Equal(t, uint32(0), l[0].SwarmPort)  // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid()) // not known yet.

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "https://app1.domain.com",
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec)
	require.NotNil(t, l)
	require.Equal(t, 1, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, "app1.domain.com", l[0].host())
	require.Equal(t, loadbalancer.HTTPS, l[0].protocol())
	require.Equal(t, uint32(443), l[0].extPort())

	require.Equal(t, uint32(0), l[0].SwarmPort)  // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid()) // not known yet.

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "tcp://app1.domain.com:2375",
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec)
	require.NotNil(t, l)
	require.Equal(t, 1, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, "app1.domain.com", l[0].host())
	require.Equal(t, loadbalancer.TCP, l[0].protocol())
	require.Equal(t, uint32(2375), l[0].extPort())

	require.Equal(t, uint32(0), l[0].SwarmPort)  // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid()) // not known yet.

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "tcp://app1.domain.com:2375, https://",
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec)
	require.NotNil(t, l)
	require.Equal(t, 2, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, "app1.domain.com", l[0].host())
	require.Equal(t, loadbalancer.TCP, l[0].protocol())
	require.Equal(t, uint32(2375), l[0].extPort())
	require.Equal(t, uint32(0), l[0].SwarmPort)  // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid()) // not known yet.
	require.Equal(t, "web1", l[1].Service)
	require.Equal(t, "default", l[1].host())
	require.Equal(t, loadbalancer.HTTPS, l[1].protocol())
	require.Equal(t, uint32(443), l[1].extPort())
	require.Equal(t, uint32(0), l[1].SwarmPort)  // implied, no explicit port=url mapping
	require.False(t, l[1].SwarmProtocol.Valid()) // not known yet.
}

func TestListenersToPublishExplicitMapping(t *testing.T) {
	s := swarm.Service{}
	s.Spec.Name = "web1"

	l := listenersFromLabel(s, LabelExternalLoadBalancerSpec)
	require.Equal(t, 0, len(l))
	require.NotNil(t, l)

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "30000=http://:8080",
	}
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec)
	require.NotNil(t, l)
	require.Equal(t, 1, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, "default", l[0].host())
	require.Equal(t, loadbalancer.HTTP, l[0].protocol())
	require.Equal(t, uint32(8080), l[0].extPort())
	require.Equal(t, uint32(30000), l[0].SwarmPort) // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid())    // not known yet.

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "30000=https://, 4040=tcp://foo.com:4040",
	}
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec)
	require.NotNil(t, l)
	require.Equal(t, 2, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, "default", l[0].host())
	require.Equal(t, loadbalancer.HTTPS, l[0].protocol())
	require.Equal(t, uint32(443), l[0].extPort())
	require.Equal(t, uint32(30000), l[0].SwarmPort)
	require.False(t, l[0].SwarmProtocol.Valid())

	require.Equal(t, "web1", l[1].Service)
	require.Equal(t, "foo.com", l[1].host())
	require.Equal(t, loadbalancer.TCP, l[1].protocol())
	require.Equal(t, uint32(4040), l[1].extPort())
	require.Equal(t, uint32(4040), l[1].SwarmPort)
	require.False(t, l[1].SwarmProtocol.Valid())
}

func TestListenersFromExposedPorts(t *testing.T) {
	s := swarm.Service{}
	s.Spec.Name = "web1"

	l := listenersFromExposedPorts(s)
	require.Equal(t, 0, len(l))
	require.NotNil(t, l)

	s.Spec.EndpointSpec = &swarm.EndpointSpec{
		Ports: []swarm.PortConfig{
			{
				Protocol:   swarm.PortConfigProtocol("tcp"),
				TargetPort: uint32(8080),
			},
			{
				Protocol:   swarm.PortConfigProtocol("tcp"),
				TargetPort: uint32(4343),
			},
		},
	}
	s.Endpoint.Ports = []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8080),
			PublishedPort: uint32(30000),
		},
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8081),
			PublishedPort: uint32(30001),
		},
	}

	l = listenersFromExposedPorts(s)
	require.Equal(t, 0, len(l))
	require.NotNil(t, l)

	// Now another case with user defined publish ports
	s.Spec.EndpointSpec = &swarm.EndpointSpec{
		Ports: []swarm.PortConfig{
			{
				Protocol:      swarm.PortConfigProtocol("tcp"),
				TargetPort:    uint32(8080),
				PublishedPort: uint32(8080),
			},
			{
				Protocol:   swarm.PortConfigProtocol("tcp"),
				TargetPort: uint32(4343),
			},
		},
	}
	s.Endpoint.Ports = []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8080),
			PublishedPort: uint32(8080),
		},
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8081),
			PublishedPort: uint32(30000), // assigned port -- not at what user requested
		},
	}

	l = listenersFromExposedPorts(s)
	require.Equal(t, 1, len(l))
	require.NotNil(t, l)

	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, "default", l[0].host())
	require.Equal(t, loadbalancer.TCP, l[0].protocol())
	require.Equal(t, uint32(8080), l[0].extPort())
	require.Equal(t, uint32(8080), l[0].SwarmPort)
	require.False(t, l[0].SwarmProtocol.Valid())
}
