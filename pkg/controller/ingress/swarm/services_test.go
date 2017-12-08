package swarm

import (
	"strings"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/stretchr/testify/require"
)

func TestExternalLoadBalancerListenersFromService1(t *testing.T) {

	certLabel := "cert.id"
	certLookupID := "cert-uuid@8080"
	healthLabel := "health.url"
	healthPath := "/othercheck@1234,/health@8080"

	s := swarm.Service{}
	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "http://:8080",
		certLabel:                     certLookupID,
		healthLabel:                   healthPath,
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports

	listenersByHost := externalLoadBalancerListenersFromServices([]swarm.Service{s}, false,
		LabelExternalLoadBalancerSpec, certLabel, healthLabel)
	require.NotNil(t, listenersByHost)
	require.Equal(t, 0, len(listenersByHost))

	// now we have exposed port
	s.Spec.Name = "web1"
	s.Endpoint.Ports = []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8080),
			PublishedPort: uint32(8080),
		},
	}

	listenersByHost = externalLoadBalancerListenersFromServices([]swarm.Service{s}, true,
		LabelExternalLoadBalancerSpec, certLabel, healthLabel)
	require.NotNil(t, listenersByHost)
	require.Equal(t, 1, len(listenersByHost))

	hostname := HostNotSpecified
	listeners, has := listenersByHost[hostname]
	require.True(t, has)
	require.Equal(t, 1, len(listeners))
	listener := listeners[0]
	require.Equal(t, "web1", listener.Service)
	require.Equal(t, 8080, listener.SwarmPort)
	require.Equal(t, loadbalancer.TCP, listener.SwarmProtocol)
	require.Equal(t, HostNotSpecified, listener.host())
	require.Equal(t, loadbalancer.SSL, listener.protocol())
	require.Equal(t, 8080, listener.extPort())
	require.Equal(t, strings.Split(certLookupID, "@")[0], *listener.CertASN())
	require.Equal(t, "/health", *listener.GetHealthMonitorPath())
}

func TestExternalLoadBalancerListenersFromService1a(t *testing.T) {

	certLabel := "cert.id"
	certLookupID := "cert-uuid"
	healthLabel := "health.label"
	healthPath := "/health"
	healthPathPort := healthPath + "@1234,8080"

	s := swarm.Service{}
	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "http://:8080",
		certLabel:                     certLookupID,
		healthLabel:                   healthPathPort,
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports

	listenersByHost := externalLoadBalancerListenersFromServices([]swarm.Service{s}, false,
		LabelExternalLoadBalancerSpec, certLabel, healthLabel)
	require.NotNil(t, listenersByHost)
	require.Equal(t, 0, len(listenersByHost))

	// now we have exposed port
	s.Spec.Name = "web1"
	s.Endpoint.Ports = []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8080),
			PublishedPort: uint32(8080),
		},
	}

	listenersByHost = externalLoadBalancerListenersFromServices([]swarm.Service{s}, true,
		LabelExternalLoadBalancerSpec, certLabel, healthLabel)
	require.NotNil(t, listenersByHost)
	require.Equal(t, 1, len(listenersByHost))

	hostname := HostNotSpecified
	listeners, has := listenersByHost[hostname]
	require.True(t, has)
	require.Equal(t, 1, len(listeners))
	listener := listeners[0]
	require.Equal(t, "web1", listener.Service)
	require.Equal(t, 8080, listener.SwarmPort)
	require.Equal(t, loadbalancer.TCP, listener.SwarmProtocol)
	require.Equal(t, HostNotSpecified, listener.host())
	require.Equal(t, loadbalancer.HTTP, listener.protocol())
	require.Equal(t, 8080, listener.extPort())
	require.Equal(t, (*string)(nil), listener.CertASN())
	require.Equal(t, &healthPath, listener.GetHealthMonitorPath())
}

func TestExternalLoadBalancerListenersFromService2(t *testing.T) {
	certLabel := "certLabel"
	certID := "certID@80"
	healthLabel := "health.label"
	healthPath := "/health@1234"

	s := swarm.Service{}
	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "http://",
		certLabel:                     certID,
		healthLabel:                   healthPath,
	}
	s.Spec.Name = "web1"
	s.Endpoint.Ports = []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8080),
			PublishedPort: uint32(30000),
		},
	}

	listenersByHost := externalLoadBalancerListenersFromServices([]swarm.Service{s}, true,
		LabelExternalLoadBalancerSpec, certLabel, healthLabel)
	require.NotNil(t, listenersByHost)
	require.Equal(t, 1, len(listenersByHost))

	hostname := HostNotSpecified
	listeners, has := listenersByHost[hostname]
	require.True(t, has)
	require.Equal(t, 1, len(listeners))
	listener := listeners[0]
	require.Equal(t, "web1", listener.Service)
	require.Equal(t, 30000, listener.SwarmPort)
	require.Equal(t, loadbalancer.TCP, listener.SwarmProtocol)
	require.Equal(t, HostNotSpecified, listener.host())
	require.Equal(t, loadbalancer.SSL, listener.protocol())
	require.Equal(t, 80, listener.extPort())
	require.Equal(t, strings.Split(certID, "@")[0], *listener.CertASN())
	// Health port does not match, return nothing
	require.Equal(t, (*string)(nil), listener.GetHealthMonitorPath())
}

func TestExternalLoadBalancerListenersFromService2a(t *testing.T) {
	certLabel := "certLabel"
	certID := "certID"
	healthLabel := "health.label"
	healthPath := "/health"

	s := swarm.Service{}
	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "http://",
		certLabel:                     certID,
		healthLabel:                   healthPath,
	}
	s.Spec.Name = "web1"
	s.Endpoint.Ports = []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8080),
			PublishedPort: uint32(30000),
		},
	}

	listenersByHost := externalLoadBalancerListenersFromServices([]swarm.Service{s}, true,
		LabelExternalLoadBalancerSpec, certLabel, healthLabel)
	require.NotNil(t, listenersByHost)
	require.Equal(t, 1, len(listenersByHost))

	hostname := HostNotSpecified
	listeners, has := listenersByHost[hostname]
	require.True(t, has)
	require.Equal(t, 1, len(listeners))
	listener := listeners[0]
	require.Equal(t, "web1", listener.Service)
	require.Equal(t, 30000, listener.SwarmPort)
	require.Equal(t, loadbalancer.TCP, listener.SwarmProtocol)
	require.Equal(t, HostNotSpecified, listener.host())
	require.Equal(t, loadbalancer.HTTP, listener.protocol())
	require.Equal(t, 80, listener.extPort())
	require.Equal(t, (*string)(nil), listener.CertASN())
	require.Equal(t, (*string)(nil), listener.GetHealthMonitorPath())
}

func TestExternalLoadBalancerListenersFromService3(t *testing.T) {
	certLabel := "emptylabel"
	healthLabel := "emptylabel"

	s := swarm.Service{}
	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "http://foo.bar.com",
	}
	s.Spec.Name = "web1"
	s.Endpoint.Ports = []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(5000),
			PublishedPort: uint32(5000),
		},
	}

	listenersByHost := externalLoadBalancerListenersFromServices([]swarm.Service{s}, true,
		LabelExternalLoadBalancerSpec, certLabel, healthLabel)
	require.NotNil(t, listenersByHost)
	require.Equal(t, 1, len(listenersByHost))

	hostname := "foo.bar.com"
	listeners, has := listenersByHost[hostname]
	require.True(t, has)
	require.Equal(t, 1, len(listeners))
	listener := listeners[0]
	require.Equal(t, "web1", listener.Service)
	require.Equal(t, 5000, listener.SwarmPort)
	require.Equal(t, loadbalancer.TCP, listener.SwarmProtocol)
	require.Equal(t, "foo.bar.com", listener.host())
	require.Equal(t, loadbalancer.HTTP, listener.protocol())
	require.Equal(t, 80, listener.extPort())
}

func TestExternalLoadBalancerListenersFromService4(t *testing.T) {
	certLabel := "emptylabel"
	healthLabel := "emptyLabel"

	s := swarm.Service{}
	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "4556=tcp://foo.bar.com:4556",
	}
	s.Spec.Name = "web1"
	s.Endpoint.Ports = []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(4556),
			PublishedPort: uint32(4556),
		},
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8080),
			PublishedPort: uint32(30000),
		},
	}

	listenersByHost := externalLoadBalancerListenersFromServices([]swarm.Service{s}, true,
		LabelExternalLoadBalancerSpec, certLabel, healthLabel)
	require.NotNil(t, listenersByHost)
	require.Equal(t, 1, len(listenersByHost))

	hostname := "foo.bar.com"
	listeners, has := listenersByHost[hostname]
	require.True(t, has)
	require.Equal(t, 1, len(listeners))
	listener := listeners[0]
	require.Equal(t, "web1", listener.Service)
	require.Equal(t, 4556, listener.SwarmPort)
	require.Equal(t, loadbalancer.TCP, listener.SwarmProtocol)
	require.Equal(t, "foo.bar.com", listener.host())
	require.Equal(t, loadbalancer.TCP, listener.protocol())
	require.Equal(t, 4556, listener.extPort())
}

func findListener(listeners []*listener, protocol loadbalancer.Protocol, host string, extPort, swarmPort int) bool {
	for _, l := range listeners {
		if l.extPort() == extPort && l.host() == host && l.protocol() == protocol && l.SwarmPort == swarmPort {
			return true
		}
	}
	return false
}

func TestExternalLoadBalancerListenersFromService5(t *testing.T) {
	certLabel := "emptylabel"
	healthLabel := "emptylabel"

	s := swarm.Service{}
	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "8080=http://foo.bar.com:8080, 4343=https://secret.com",
	}
	s.Spec.Name = "web1"
	s.Spec.EndpointSpec = &swarm.EndpointSpec{
		Ports: []swarm.PortConfig{
			{
				Protocol:      swarm.PortConfigProtocol("tcp"),
				TargetPort:    uint32(8080),
				PublishedPort: uint32(8080),
				PublishMode:   swarm.PortConfigPublishModeIngress,
			},
			{
				Protocol:      swarm.PortConfigProtocol("tcp"),
				TargetPort:    uint32(4343),
				PublishedPort: uint32(4343),
				PublishMode:   swarm.PortConfigPublishModeIngress,
			},
		},
	}
	s.Endpoint.Ports = []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8080),
			PublishedPort: uint32(8080),
			PublishMode:   swarm.PortConfigPublishModeIngress,
		},
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(4343),
			PublishedPort: uint32(4343),
			PublishMode:   swarm.PortConfigPublishModeIngress,
		},
	}

	listenersByHost := externalLoadBalancerListenersFromServices([]swarm.Service{s}, true, LabelExternalLoadBalancerSpec, certLabel, healthLabel)
	require.NotNil(t, listenersByHost)
	require.Equal(t, 3, len(listenersByHost))

	hostname := HostNotSpecified
	listeners, has := listenersByHost[hostname]
	require.True(t, has)
	require.Equal(t, 2, len(listeners))
	require.True(t, findListener(listeners, loadbalancer.TCP, HostNotSpecified, 8080, 8080))
	require.True(t, findListener(listeners, loadbalancer.TCP, HostNotSpecified, 4343, 4343))

	hostname = "foo.bar.com"
	listeners, has = listenersByHost[hostname]
	require.True(t, has)
	require.Equal(t, 1, len(listeners))
	listener := listeners[0]
	require.Equal(t, "web1", listener.Service)
	require.Equal(t, 8080, listener.SwarmPort)
	require.Equal(t, loadbalancer.TCP, listener.SwarmProtocol)
	require.Equal(t, "foo.bar.com", listener.host())
	require.Equal(t, loadbalancer.HTTP, listener.protocol())
	require.Equal(t, 8080, listener.extPort())

	hostname = "secret.com"
	listeners, has = listenersByHost[hostname]
	require.True(t, has)
	require.Equal(t, 1, len(listeners))
	listener = listeners[0]
	require.Equal(t, "web1", listener.Service)
	require.Equal(t, 4343, listener.SwarmPort)
	require.Equal(t, loadbalancer.TCP, listener.SwarmProtocol)
	require.Equal(t, "secret.com", listener.host())
	require.Equal(t, loadbalancer.HTTPS, listener.protocol())
	require.Equal(t, 443, listener.extPort())
}

func TestExternalLoadBalancerListenersFromServiceWithNoLabels(t *testing.T) {
	certLabel := "emptylabel"
	healthLabel := "emptylabel"

	s := swarm.Service{}
	s.Spec.Name = "web1"
	s.Spec.EndpointSpec = &swarm.EndpointSpec{
		Ports: []swarm.PortConfig{
			{
				Protocol:      swarm.PortConfigProtocol("tcp"),
				TargetPort:    uint32(8080),
				PublishedPort: uint32(8080),
				PublishMode:   swarm.PortConfigPublishModeIngress,
			},
			{
				Protocol:      swarm.PortConfigProtocol("tcp"),
				TargetPort:    uint32(4343),
				PublishedPort: uint32(4343),
				PublishMode:   swarm.PortConfigPublishModeIngress,
			},
		},
	}
	s.Endpoint.Ports = []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8080),
			PublishedPort: uint32(8080),
			PublishMode:   swarm.PortConfigPublishModeIngress,
		},
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(4343),
			PublishedPort: uint32(4343),
			PublishMode:   swarm.PortConfigPublishModeIngress,
		},
	}

	listenersByHost := externalLoadBalancerListenersFromServices([]swarm.Service{s}, true, LabelExternalLoadBalancerSpec, certLabel, healthLabel)
	require.NotNil(t, listenersByHost)
	require.Equal(t, 1, len(listenersByHost))

	hostname := HostNotSpecified
	listeners, has := listenersByHost[hostname]
	require.True(t, has)
	require.Equal(t, 2, len(listeners))
	require.True(t, findListener(listeners, loadbalancer.TCP, HostNotSpecified, 8080, 8080))
	require.True(t, findListener(listeners, loadbalancer.TCP, HostNotSpecified, 4343, 4343))
}
