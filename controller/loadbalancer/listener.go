package loadbalancer

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/libmachete/spi/loadbalancer"
	"net/url"
	"strconv"
	"strings"
)

// Listener monitors a swarm service.
type Listener struct {
	Service       string
	URL           *url.URL
	SwarmPort     uint32
	SwarmProtocol loadbalancer.Protocol
}

// NewListener creates a new swarm listener.
func NewListener(service string, swarmPort uint32, urlStr string) (*Listener, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return &Listener{
		Service:   service,
		URL:       u,
		SwarmPort: swarmPort,
	}, nil
}

func (l *Listener) String() string {
	return fmt.Sprintf("service(%s):%d ==> %s", l.Service, l.SwarmPort, l.URL)
}

// ExtPort gets the TCP port of the service.
func (l *Listener) ExtPort() uint32 {
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

// Host gets the host name of the service.
func (l *Listener) Host() string {
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
func (l *Listener) Protocol() loadbalancer.Protocol {
	scheme := ""
	if l.URL != nil {
		scheme = l.URL.Scheme
	}
	return loadbalancer.ProtocolFromString(scheme)
}

// ExplicitSwarmPortToURL is explicit mapping in the format of {swarm_port}={url}
func ExplicitSwarmPortToURL(service, spec string) (*Listener, error) {
	parts := strings.Split(strings.Trim(spec, " \t"), "=")
	if len(parts) != 2 {
		return nil, fmt.Errorf("bad spec: %s for service %s", spec, service)
	}
	swarmPort, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("bad spec: %s for service %s", spec, service)
	}

	listener, err := ImpliedSwarmPortToURL(service, parts[1])
	listener.SwarmPort = uint32(swarmPort)
	return listener, nil
}

// ImpliedSwarmPortToURL is implied when only one exposed port exists for the service.  It's just a {url}
func ImpliedSwarmPortToURL(service, spec string) (*Listener, error) {
	if strings.Index(spec, "=") > -1 {
		return nil, fmt.Errorf("bad format:%s", spec)
	}
	serviceURL, err := url.Parse(spec)
	if err != nil {
		return nil, err
	}
	return NewListener(service, uint32(0), serviceURL.String())
}

func addListenerToHostMap(m map[string][]*Listener, l *Listener) {
	host := l.Host()
	if _, has := m[host]; !has {
		m[host] = []*Listener{}
	}
	m[host] = append(m[host], l)
}

// Generate a list of listeners, where the URL are defaulted to the swarm port and default ELB
func listenersFromExposedPorts(service swarm.Service) []*Listener {

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
	listeners := []*Listener{}
	for _, exposed := range service.Endpoint.Ports {

		requestedPublishPort, has := requestedPublishPorts[exposed.TargetPort]
		if has && requestedPublishPort == exposed.PublishedPort {

			urlString := fmt.Sprintf("%v://:%d", strings.ToLower(string(exposed.Protocol)), exposed.PublishedPort)
			if listener, err := NewListener(service.Spec.Name, exposed.PublishedPort, urlString); err == nil {
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

// Generates a list of listeners based on what's specifiedi in the label.  The listeners
// are then matched against the exposed ports in the service.
func listenersFromLabel(service swarm.Service, label string) []*Listener {
	// get all the specs -- the label determines what elb listeners are to be created.
	labelValue, has := service.Spec.Labels[label]
	if !has || labelValue == "" {
		return []*Listener{}
	}

	// If the swarm exposes more than one port, we'd need to have explicit mapping; otherwise
	// it's impossible to determine which one of the exposed ports maps to which url.
	// However, if there's only one exposed port, it's actually ok to have more than one url mapping.

	listenersToPublish := []*Listener{}

	// spec can be comma-delimited list
	for _, spec := range strings.Split(labelValue, ",") {

		specParser := ImpliedSwarmPortToURL
		if strings.Index(spec, "=") > -1 {
			specParser = ExplicitSwarmPortToURL
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
