package swarm

import (
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/stretchr/testify/require"
)

func TestListener(t *testing.T) {
	var emptyCert, emptyHealthPath *string
	l, err := newListener("foo", 30000, "http://:80", emptyCert, emptyHealthPath)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.HTTP, l.protocol())
	require.Equal(t, int(80), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())

	l, err = newListener("foo", 30000, "http://", emptyCert, emptyHealthPath)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.HTTP, l.protocol())
	require.Equal(t, int(80), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())

	l, err = newListener("foo", 30000, "http://localswarm:8080", emptyCert, emptyHealthPath)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.HTTP, l.protocol())
	require.Equal(t, int(8080), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, "localswarm", l.host())
}

func TestListenerSSLCertNoPort(t *testing.T) {
	var emptyCert *string
	cert := "asn:blah"
	healthPath := "/health"
	healthPathPort := "/health@443"

	// has cert and port is 443, so it should be SSL.
	l, err := newListener("foo", 30000, "tcp://:443", &cert, &healthPathPort)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.SSL, l.protocol())
	require.Equal(t, int(443), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, &cert, l.CertASN())
	require.Equal(t, map[int]string{443: "SSL"}, l.CertPorts())
	r := l.asRoute()
	require.Equal(t, loadbalancer.SSL, r.Protocol)
	require.Equal(t, &cert, r.Certificate)
	require.Equal(t, &healthPath, r.HealthMonitorPath)

	// has cert but since port wasn't specified, it defaults to 443
	// since port isn't 443, then this is not SSL.
	l, err = newListener("foo", 30000, "tcp://:444", &cert, &healthPathPort)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.TCP, l.protocol())
	require.Equal(t, int(444), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, (*string)(nil), l.CertASN())
	require.Equal(t, map[int]string{443: "SSL"}, l.CertPorts())
	r = l.asRoute()
	require.Equal(t, loadbalancer.TCP, r.Protocol)
	require.Equal(t, loadbalancer.TCP, r.LoadBalancerProtocol)
	require.Equal(t, (*string)(nil), r.Certificate)
	require.Equal(t, (*string)(nil), r.HealthMonitorPath)

	// no cert so not SSL.
	l, err = newListener("foo", 30000, "tcp://:443", emptyCert, &healthPathPort)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.TCP, l.protocol())
	require.Equal(t, int(443), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, emptyCert, l.CertASN())
	require.Equal(t, map[int]string{443: "SSL"}, l.CertPorts())
	r = l.asRoute()
	require.Equal(t, loadbalancer.TCP, r.Protocol)
	require.Equal(t, loadbalancer.TCP, r.LoadBalancerProtocol) // no cert
	require.Equal(t, emptyCert, r.Certificate)
	require.Equal(t, &healthPath, r.HealthMonitorPath)
}

func TestListenerSSLCertWithPorts(t *testing.T) {
	asn := "asn:blah"
	certOnePort := asn + "@443"
	certOnePort2 := asn + "@442"
	certTwoPorts := asn + "@443,442"
	certEmptyPorts := asn + "@"
	certHTTPSPort := asn + "@HTTPS:443"
	certHTTPSPortSSLPORT := asn + "@HTTPS:443,444"
	healthPath := "/health"
	healthPathPort1 := healthPath + "@443"
	healthPathPort2 := healthPath + "@442"
	healthPathPort3 := healthPath + "@444"
	healthPathTwoPorts := healthPath + "@443,442"
	healthPathEmptyPorts := healthPath + "@"

	// has cert and port is 443, so it should be SSL.
	l, err := newListener("foo", 30000, "tcp://:443", &certOnePort, &healthPathPort1)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.SSL, l.protocol())
	require.Equal(t, int(443), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, &asn, l.CertASN())
	require.Equal(t, map[int]string{443: "SSL"}, l.CertPorts())
	r := l.asRoute()
	require.Equal(t, loadbalancer.SSL, r.Protocol)
	require.Equal(t, loadbalancer.SSL, r.LoadBalancerProtocol)
	require.Equal(t, asn, *r.Certificate)
	// Health port matches actual
	require.Equal(t, &healthPath, r.HealthMonitorPath)

	// has cert with port 442, this should be SSL.
	l, err = newListener("foo", 30000, "tcp://:442", &certOnePort2, &healthPathPort2)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.SSL, l.protocol())
	require.Equal(t, int(442), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, &asn, l.CertASN())
	require.Equal(t, map[int]string{442: "SSL"}, l.CertPorts())
	r = l.asRoute()
	require.Equal(t, loadbalancer.SSL, r.Protocol)
	require.Equal(t, asn, *r.Certificate)
	// Health port matches actual
	require.Equal(t, &healthPath, r.HealthMonitorPath)

	// cert has 2 ports, 442 is one of them, assume SSL
	l, err = newListener("foo", 30000, "tcp://:442", &certTwoPorts, &healthPathTwoPorts)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.SSL, l.protocol())
	require.Equal(t, int(442), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, &asn, l.CertASN())
	require.Equal(t, map[int]string{443: "SSL", 442: "SSL"}, l.CertPorts())
	r = l.asRoute()
	require.Equal(t, loadbalancer.SSL, r.Protocol)
	require.Equal(t, asn, *r.Certificate)
	// Health port matches one of the ports
	require.Equal(t, &healthPath, r.HealthMonitorPath)

	// cert but no port, assume port 443, this is 442 so loadbalancer.TCP not SSL
	l, err = newListener("foo", 30000, "tcp://:442", &certEmptyPorts, &healthPathEmptyPorts)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.TCP, l.protocol())
	require.Equal(t, int(442), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, (*string)(nil), l.CertASN())
	require.Equal(t, map[int]string{443: "SSL"}, l.CertPorts())
	r = l.asRoute()
	require.Equal(t, loadbalancer.TCP, r.Protocol)
	require.Equal(t, (*string)(nil), r.Certificate)
	// Health port makes no assumptions, if no matches returns nothing
	require.Equal(t, (*string)(nil), r.HealthMonitorPath)

	// cert but no port, assume port 443
	l, err = newListener("foo", 30000, "tcp://:443", &certEmptyPorts, &healthPathEmptyPorts)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.SSL, l.protocol())
	require.Equal(t, int(443), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, &asn, l.CertASN())
	require.Equal(t, map[int]string{443: "SSL"}, l.CertPorts())
	r = l.asRoute()
	require.Equal(t, loadbalancer.SSL, r.Protocol)
	require.Equal(t, asn, *r.Certificate)
	// Health port makes no assumptions, if no matches returns nothing
	require.Equal(t, (*string)(nil), r.HealthMonitorPath)

	// cert but HTTPS port, verify it
	l, err = newListener("foo", 30000, "tcp://:443", &certHTTPSPort, &healthPathPort1)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.HTTP, l.protocol())
	require.Equal(t, loadbalancer.HTTPS, l.loadbalancerProtocol())
	require.Equal(t, int(443), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, &asn, l.CertASN())
	require.Equal(t, map[int]string{443: "HTTPS"}, l.CertPorts())
	r = l.asRoute()
	require.Equal(t, loadbalancer.HTTP, r.Protocol)
	require.Equal(t, loadbalancer.HTTPS, r.LoadBalancerProtocol)
	require.Equal(t, asn, *r.Certificate)
	require.Equal(t, &healthPath, r.HealthMonitorPath)

	// cert but HTTPS port and SSL port (no schema specified), verify SSL
	l, err = newListener("foo", 30000, "tcp://:444", &certHTTPSPortSSLPORT, &healthPathPort3)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.SSL, l.protocol())
	require.Equal(t, int(444), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, &asn, l.CertASN())
	require.Equal(t, map[int]string{443: "HTTPS", 444: "SSL"}, l.CertPorts())
	r = l.asRoute()
	require.Equal(t, loadbalancer.SSL, r.Protocol)
	require.Equal(t, asn, *r.Certificate)
	require.Equal(t, &healthPath, r.HealthMonitorPath)

	// cert but HTTPS port and SSL port (no schema specified), verify unspecified port is TCP
	l, err = newListener("foo", 30000, "tcp://:8080", &certHTTPSPortSSLPORT, &healthPathPort1)
	require.NoError(t, err)

	require.Equal(t, loadbalancer.TCP, l.protocol())
	require.Equal(t, int(8080), l.extPort())
	require.Equal(t, int(30000), l.SwarmPort)
	require.Equal(t, "foo", l.Service)
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, (*string)(nil), l.CertASN())
	require.Equal(t, map[int]string{443: "HTTPS", 444: "SSL"}, l.CertPorts())
	r = l.asRoute()
	require.Equal(t, loadbalancer.TCP, r.Protocol)
	require.Equal(t, (*string)(nil), r.Certificate)
	require.Equal(t, (*string)(nil), r.HealthMonitorPath)
}

func TestImpliedSwarmPortToUrl(t *testing.T) {
	l, err := impliedSwarmPortToURL("foo", "30000=http://:8080")
	require.Error(t, err) // Because this is the explicit form

	l, err = impliedSwarmPortToURL("foo", "http://:8080")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(8080), l.extPort())
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, int(0), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = impliedSwarmPortToURL("foo", "https://")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(443), l.extPort())
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, int(0), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTPS, l.protocol())
	r := l.asRoute()
	require.Equal(t, loadbalancer.HTTPS, r.LoadBalancerProtocol)

	l, err = impliedSwarmPortToURL("foo", "http://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(80), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, int(0), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = impliedSwarmPortToURL("foo", "HTTP://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(80), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, int(0), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = impliedSwarmPortToURL("foo", "tcp://myapp.domain.com:2333")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(2333), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, int(0), l.SwarmPort)
	require.Equal(t, loadbalancer.TCP, l.protocol())

	l, err = impliedSwarmPortToURL("foo", "ssl://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(0), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, int(0), l.SwarmPort)
	require.Equal(t, loadbalancer.SSL, l.protocol())

	l, err = impliedSwarmPortToURL("foo", "https://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(443), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, int(0), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTPS, l.protocol())
}

func TestExplicitSwarmPortToUrl(t *testing.T) {
	l, err := explicitSwarmPortToURL("foo", "http://:8080")
	require.Error(t, err) // Because this is the implicit form

	l, err = explicitSwarmPortToURL("foo", "7000=http://:8080")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(8080), l.extPort())
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, int(7000), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = explicitSwarmPortToURL("foo", "8999=https://")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(443), l.extPort())
	require.Equal(t, HostNotSpecified, l.host())
	require.Equal(t, int(8999), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTPS, l.protocol())

	l, err = explicitSwarmPortToURL("foo", "80=http://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(80), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, int(80), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = explicitSwarmPortToURL("foo", "8088=HTTP://myapp.domain.com")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(80), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, int(8088), l.SwarmPort)
	require.Equal(t, loadbalancer.HTTP, l.protocol())

	l, err = explicitSwarmPortToURL("foo", "7543=tcp://myapp.domain.com:2333")
	require.NoError(t, err)
	require.NotNil(t, l.URL)
	require.Equal(t, int(2333), l.extPort())
	require.Equal(t, "myapp.domain.com", l.host())
	require.Equal(t, int(7543), l.SwarmPort)
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

	l := listenersFromLabel(s, LabelExternalLoadBalancerSpec, "", "")
	require.Equal(t, 0, len(l))
	require.NotNil(t, l)

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "http://:8080",
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec, "", "")
	require.NotNil(t, l)
	require.Equal(t, 1, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, HostNotSpecified, l[0].host())
	require.Equal(t, loadbalancer.HTTP, l[0].protocol())
	require.Equal(t, int(8080), l[0].extPort())

	require.Equal(t, int(0), l[0].SwarmPort)     // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid()) // not known yet.

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "http://",
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec, "", "")
	require.NotNil(t, l)
	require.Equal(t, 1, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, HostNotSpecified, l[0].host())
	require.Equal(t, loadbalancer.HTTP, l[0].protocol())
	require.Equal(t, int(80), l[0].extPort())

	require.Equal(t, int(0), l[0].SwarmPort)     // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid()) // not known yet.

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "https://app1.domain.com",
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec, "", "")
	require.NotNil(t, l)
	require.Equal(t, 1, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, "app1.domain.com", l[0].host())
	require.Equal(t, loadbalancer.HTTPS, l[0].protocol())
	require.Equal(t, int(443), l[0].extPort())

	require.Equal(t, int(0), l[0].SwarmPort)     // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid()) // not known yet.

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "tcp://app1.domain.com:2375",
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec, "", "")
	require.NotNil(t, l)
	require.Equal(t, 1, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, "app1.domain.com", l[0].host())
	require.Equal(t, loadbalancer.TCP, l[0].protocol())
	require.Equal(t, int(2375), l[0].extPort())

	require.Equal(t, int(0), l[0].SwarmPort)     // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid()) // not known yet.

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "tcp://app1.domain.com:2375, https://",
	}
	s.Endpoint.Ports = []swarm.PortConfig{} // no exposed ports
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec, "", "")
	require.NotNil(t, l)
	require.Equal(t, 2, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, "app1.domain.com", l[0].host())
	require.Equal(t, loadbalancer.TCP, l[0].protocol())
	require.Equal(t, int(2375), l[0].extPort())
	require.Equal(t, int(0), l[0].SwarmPort)     // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid()) // not known yet.
	require.Equal(t, "web1", l[1].Service)
	require.Equal(t, HostNotSpecified, l[1].host())
	require.Equal(t, loadbalancer.HTTPS, l[1].protocol())
	require.Equal(t, int(443), l[1].extPort())
	require.Equal(t, int(0), l[1].SwarmPort)     // implied, no explicit port=url mapping
	require.False(t, l[1].SwarmProtocol.Valid()) // not known yet.
}

func TestListenersToPublishExplicitMapping(t *testing.T) {
	s := swarm.Service{}
	s.Spec.Name = "web1"

	l := listenersFromLabel(s, LabelExternalLoadBalancerSpec, "", "")
	require.Equal(t, 0, len(l))
	require.NotNil(t, l)

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "30000=http://:8080",
	}
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec, "", "")
	require.NotNil(t, l)
	require.Equal(t, 1, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, HostNotSpecified, l[0].host())
	require.Equal(t, loadbalancer.HTTP, l[0].protocol())
	require.Equal(t, int(8080), l[0].extPort())
	require.Equal(t, int(30000), l[0].SwarmPort) // implied, no explicit port=url mapping
	require.False(t, l[0].SwarmProtocol.Valid()) // not known yet.

	s.Spec.Labels = map[string]string{
		LabelExternalLoadBalancerSpec: "30000=https://, 4040=tcp://foo.com:4040",
	}
	l = listenersFromLabel(s, LabelExternalLoadBalancerSpec, "", "")
	require.NotNil(t, l)
	require.Equal(t, 2, len(l))
	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, HostNotSpecified, l[0].host())
	require.Equal(t, loadbalancer.HTTPS, l[0].protocol())
	require.Equal(t, int(443), l[0].extPort())
	require.Equal(t, int(30000), l[0].SwarmPort)
	require.False(t, l[0].SwarmProtocol.Valid())

	require.Equal(t, "web1", l[1].Service)
	require.Equal(t, "foo.com", l[1].host())
	require.Equal(t, loadbalancer.TCP, l[1].protocol())
	require.Equal(t, int(4040), l[1].extPort())
	require.Equal(t, int(4040), l[1].SwarmPort)
	require.False(t, l[1].SwarmProtocol.Valid())
}

func TestListenersFromExposedPorts(t *testing.T) {
	s := swarm.Service{}
	s.Spec.Name = "web1"

	l := listenersFromExposedPorts(s, "emptyLabel", "emptyLabel")
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

	l = listenersFromExposedPorts(s, "emptyLabel", "emptyLabel")
	require.Equal(t, 0, len(l))
	require.NotNil(t, l)

	// Now another case with user defined publish ports
	s.Spec.EndpointSpec = &swarm.EndpointSpec{
		Ports: []swarm.PortConfig{
			{
				Protocol:      swarm.PortConfigProtocol("tcp"),
				TargetPort:    uint32(8080),
				PublishedPort: uint32(8080),
				PublishMode:   swarm.PortConfigPublishModeIngress,
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
			PublishMode:   swarm.PortConfigPublishModeIngress,
		},
		{
			Protocol:      swarm.PortConfigProtocol("tcp"),
			TargetPort:    uint32(8081),
			PublishedPort: uint32(30000), // assigned port -- not at what user requested
		},
	}

	l = listenersFromExposedPorts(s, "emptyLabel", "emptyLabel")
	require.Equal(t, 1, len(l))
	require.NotNil(t, l)

	require.Equal(t, "web1", l[0].Service)
	require.Equal(t, HostNotSpecified, l[0].host())
	require.Equal(t, loadbalancer.TCP, l[0].protocol())
	require.Equal(t, 8080, l[0].extPort())
	require.Equal(t, 8080, l[0].SwarmPort)
	require.False(t, l[0].SwarmProtocol.Valid())
}
