package loadbalancer

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/types/swarm"
	"net/url"
	"strconv"
	"strings"
)

type listener struct {
	Service       string
	URL           *url.URL
	SwarmPort     uint32
	SwarmProtocol Protocol
}

func newListener(service string, swarmPort uint32, urlStr string) (*listener, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return &listener{
		Service:   service,
		URL:       u,
		SwarmPort: swarmPort,
	}, nil
}

func (l *listener) asRoute() Route {
	return Route{
		Port:             l.SwarmPort,
		Protocol:         l.protocol(),
		LoadBalancerPort: l.extPort(),
	}
}

func (l *listener) String() string {
	return fmt.Sprintf("service(%s):%d ==> %s", l.Service, l.SwarmPort, l.URL)
}

func (l *listener) extPort() uint32 {
	hostport := ":80"
	scheme := "http"
	if l.URL != nil {
		hostport = l.URL.Host
		scheme = l.URL.Scheme
	}

	parts := strings.Split(hostport, ":")
	if len(parts) > 1 {
		p, _ := strconv.Atoi(parts[1])
		return uint32(p)
	}
	switch scheme {
	case "http":
		return uint32(80)
	case "https":
		return uint32(443)
	default:
		return uint32(0) // Intentionally invalid
	}
}

func (l *listener) host() string {
	hostport := ":80"
	if l.URL != nil {
		hostport = l.URL.Host
	}
	h := strings.Split(hostport, ":")[0]
	if h == "" {
		return "default"
	}
	return h
}

// Protocol gets the network protocol used by the service.
func (l *listener) protocol() Protocol {
	scheme := ""
	if l.URL != nil {
		scheme = l.URL.Scheme
	}
	return ProtocolFromString(scheme)
}

// explicitSwarmPortToURL is explicit mapping in the format of {swarm_port}={url}
func explicitSwarmPortToURL(service, spec string) (*listener, error) {
	parts := strings.Split(strings.Trim(spec, " \t"), "=")
	if len(parts) != 2 {
		return nil, fmt.Errorf("bad spec: %s for service %s", spec, service)
	}
	swarmPort, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("bad spec: %s for service %s", spec, service)
	}

	listener, err := impliedSwarmPortToURL(service, parts[1])
	listener.SwarmPort = uint32(swarmPort)
	return listener, nil
}

// impliedSwarmPortToURL is implied when only one exposed port exists for the service.  It's just a {url}
func impliedSwarmPortToURL(service, spec string) (*listener, error) {
	if strings.Index(spec, "=") > -1 {
		return nil, fmt.Errorf("bad format:%s", spec)
	}
	serviceURL, err := url.Parse(spec)
	if err != nil {
		return nil, err
	}
	return newListener(service, uint32(0), serviceURL.String())
}

func addListenerToHostMap(m map[string][]*listener, l *listener) {
	host := l.host()
	if _, has := m[host]; !has {
		m[host] = []*listener{}
	}
	m[host] = append(m[host], l)
}

// listenersFromExposedPorts generates a list of listeners, where the URL is defaulted to the swarm port and default ELB
func listenersFromExposedPorts(service swarm.Service) []*listener {

	// Rule on publishing ports
	// If the user specified -p X:Y, then we publish the port to the ELB.  If the port
	// was assigned by Swarm, then we leave it alone.

	// Given -p X:Y option when starting up services, we look at what's requested and what's actually published:
	requestedPublishPorts := map[uint32]uint32{} // key - target port Y (app/container port), value = publish port X
	if service.Spec.EndpointSpec != nil {
		for _, p := range service.Spec.EndpointSpec.Ports {
			if p.PublishedPort > 0 {
				// Only if the user has specify the desired publish port
				requestedPublishPorts[p.TargetPort] = p.PublishedPort
			}
		}
	}

	// Now look at the ports that are actually published:
	listeners := []*listener{}
	for _, exposed := range service.Endpoint.Ports {

		requestedPublishPort, has := requestedPublishPorts[exposed.TargetPort]
		if has && requestedPublishPort == exposed.PublishedPort {

			urlString := fmt.Sprintf("%v://:%d", strings.ToLower(string(exposed.Protocol)), exposed.PublishedPort)
			if listener, err := newListener(service.Spec.Name, exposed.PublishedPort, urlString); err == nil {
				listeners = append(listeners, listener)
			} else {
				log.Warningln("Error creating listener for exposed port:", exposed, "err=", err)
			}

		} else {
			log.Infoln("Skipping exposed port:", exposed)
		}
	}
	return listeners
}

// listenersFromLabel generates a list of listeners based on what's specified in the label.  The listeners
// are then matched against the exposed ports in the service.
func listenersFromLabel(service swarm.Service, label string) []*listener {
	// get all the specs -- the label determines what elb listeners are to be created.
	labelValue, has := service.Spec.Labels[label]
	if !has || labelValue == "" {
		return []*listener{}
	}

	// If the swarm exposes more than one port, we'd need to have explicit mapping; otherwise
	// it's impossible to determine which one of the exposed ports maps to which url.
	// However, if there's only one exposed port, it's actually ok to have more than one url mapping.

	listenersToPublish := []*listener{}

	// spec can be comma-delimited list
	for _, spec := range strings.Split(labelValue, ",") {

		specParser := impliedSwarmPortToURL
		if strings.Index(spec, "=") > -1 {
			specParser = explicitSwarmPortToURL
		}

		listener, err := specParser(service.Spec.Name, strings.Trim(spec, " \t"))
		if err != nil {
			log.Warningln("Error=", err)
			continue
		}
		listenersToPublish = append(listenersToPublish, listener)
	}
	return listenersToPublish
}
