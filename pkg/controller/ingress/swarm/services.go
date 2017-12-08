package swarm

import (
	"github.com/docker/docker/api/types/swarm"
	ingress "github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

const (

	// LabelExternalLoadBalancerSpec is the label for marking a service as public facing.
	// The value is a url.  The protocol, host and port will be extracted from the URL
	// to configure the external load balancer.  The ELB will be configured to listen
	// at the specified port (or 80), using the specified protocol, and the host is used to
	// select which load balancer in the config file on the manager nodes /var/lib/docker/editions/lb.config
	// TODO(chungers) - While the hostname is used to select the ELB to use, we will also provide support
	// for HTTP/S vhosts in the future if the hostname is not matched to an ELB, we will select a top level ELB
	// and then use the subdomain in the hostname to configure a HAProxy with http header routing.
	LabelExternalLoadBalancerSpec = "docker.swarm.lb"
)

func toVhostRoutes(listeners map[string][]*listener) map[ingress.Vhost][]loadbalancer.Route {
	result := map[ingress.Vhost][]loadbalancer.Route{}
	for vhost, list := range listeners {
		routes := []loadbalancer.Route{}
		for _, l := range list {
			routes = append(routes, l.asRoute())
		}
		result[ingress.Vhost(vhost)] = routes
	}
	return result
}

func externalLoadBalancerListenersFromServices(services []swarm.Service,
	matchByLabels bool, lbSpecLabel, certLabel, healthLabel string) map[string][]*listener {

	// group the listeners by hostname.  hostname maps to a ELB somewhere else.
	listeners := map[string][]*listener{}
	for _, s := range services {

		// We index all the exposed ports by swarm port
		exposedPorts := map[int]swarm.PortConfig{}
		for _, exposed := range s.Endpoint.Ports {
			exposedPorts[int(exposed.PublishedPort)] = exposed
		}
		log.Debug("exposedPorts", "exposedPorts", exposedPorts, "V", debugV)

		if matchByLabels {
			// Now go through the list that we need to publish and match up the exposed ports
			for _, publish := range listenersFromLabel(s, lbSpecLabel, certLabel, healthLabel) {

				if sp, has := exposedPorts[int(publish.SwarmPort)]; has {

					// This is the case where we have a clear mapping of swarm port to url
					publish.SwarmProtocol = loadbalancer.ProtocolFromString(string(sp.Protocol))
					addListenerToHostMap(listeners, publish)

				} else if publish.SwarmPort == 0 && len(exposedPorts) == 1 {

					// This is the case where we have only one exposed port, and we don't specify the swarm port
					// because it's an randomly assigned port by the swarm manager.
					// We can't handle the case where there are more than one exposed port and we don't have explicit
					// swarm port to url mappings.
					for _, exposed := range exposedPorts {
						publish.SwarmProtocol = loadbalancer.ProtocolFromString(string(exposed.Protocol))
						publish.SwarmPort = int(exposed.PublishedPort)
						log.Debug("only one exposed port")
						break // Just grab the first one
					}
					addListenerToHostMap(listeners, publish)

				} else {

					// There are unresolved publishing listeners
					log.Warn("Cannot match exposed port in service", "service", s.Spec.Name, "publish", publish)

				}
			}
		}

		// Publish all exposed is always on
		for _, l := range listenersFromExposedPorts(s, certLabel, healthLabel) {
			addListenerToHostMap(listeners, l)
		}

	}
	return listeners
}

func findRoutePort(
	routes []loadbalancer.Route,
	loadbalancerPort int,
	protocol, lbProtocol loadbalancer.Protocol) (int, bool) {

	for _, route := range routes {
		if route.LoadBalancerPort == loadbalancerPort && route.Protocol == protocol && route.LoadBalancerProtocol == lbProtocol {
			return route.Port, true
		}
	}
	return 0, false
}
