package swarm

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

type listener struct {
	Service       string
	URL           *url.URL
	SwarmPort     int
	SwarmProtocol loadbalancer.Protocol
	Certificate   *string
}

func newListener(service string, swarmPort int, urlStr string, cert *string) (*listener, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return &listener{
		Service:     service,
		URL:         u,
		SwarmPort:   swarmPort,
		Certificate: cert,
	}, nil
}

func (l *listener) asRoute() loadbalancer.Route {
	return loadbalancer.Route{
		Port:                 l.SwarmPort,
		Protocol:             l.protocol(),
		LoadBalancerPort:     l.extPort(),
		LoadBalancerProtocol: l.loadbalancerProtocol(),
		Certificate:          l.CertASN(),
	}
}

// Get the ASN value from the service label
// Normal format would be label=asn@port,port it will just return the asn part
// or nil if no certificate.
func (l *listener) CertASN() *string {
	if l.Certificate == nil {
		return nil
	}
	asn := strings.Split(*l.Certificate, "@")[0]
	if asn != "" {
		return &asn
	}
	return nil
}

// Get the ports that are associated with the certificate from the service label
func (l *listener) CertPorts() []int {
	if l.Certificate == nil {
		// if we have not Certificate then return 443
		return []int{443}
	}
	parts := strings.Split(*l.Certificate, "@")
	if len(parts) > 1 {
		var finalPorts = []int{}
		ports := strings.Split(parts[1], ",")
		log.Debug("ports", "ports ", ports)
		if len(ports) > 0 {
			log.Debug("Number of ports", "count", len(ports))
			for _, port := range ports {
				log.Debug("port", "port: ", port, "V", debugV)
				if port != "" {
					j, err := strconv.ParseUint(port, 10, 32)
					if err != nil {
						log.Error("Can't convert to int: ", "port", port, "err", err)
					}
					finalPorts = append(finalPorts, int(j))
				}
			}
		}
		log.Debug("final ports", "ports", finalPorts)
		if len(finalPorts) == 0 {
			// this would happen if there was an ASN like this
			// asn:blah@
			// an at symbol but no ports after, if that is the case
			// default to 443.
			finalPorts = append(finalPorts, int(443))
		}
		return finalPorts
	}
	// if there is no port, default to 443
	return []int{443}
}

// String value of the listener, good for log statements.
func (l *listener) String() string {
	return fmt.Sprintf("service(%s):%d ==> %s", l.Service, l.SwarmPort, l.URL)
}

func (l *listener) extPort() int {
	hostport := ":80"
	scheme := "http"
	if l.URL != nil {
		hostport = l.URL.Host
		scheme = l.URL.Scheme
	}

	parts := strings.Split(hostport, ":")
	if len(parts) > 1 {
		p, _ := strconv.Atoi(parts[1])
		return int(p)
	}
	switch scheme {
	case "http":
		return int(80)
	case "https":
		return int(443)
	default:
		return int(0) // Intentionally invalid
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

// Protocol gets the network protocol used by the load balancer.
func (l *listener) loadbalancerProtocol() loadbalancer.Protocol {
	scheme := ""
	if l.URL != nil {
		scheme = l.URL.Scheme
	}

	// check if this should be SSL because it has a certificate.
	if l.Certificate != nil && intInSlice(l.extPort(), l.CertPorts()) {
		log.Infoln("port ", l.extPort(), " Is in ", l.CertPorts())
		scheme = string(loadbalancer.SSL)
	} else {
		log.Infoln("cert is nil, or port ", l.extPort(), " Is NOT in ", l.CertPorts())
	}

	return loadbalancer.ProtocolFromString(scheme)
}

// Protocol gets the network protocol used by the service.
func (l *listener) protocol() loadbalancer.Protocol {
	scheme := ""
	if l.URL != nil {
		scheme = l.URL.Scheme
	}

	// check if this should be SSL because it has a certificate.
	if l.Certificate != nil && intInSlice(l.extPort(), l.CertPorts()) {
		log.Debug("ext port is in cert ports", "extPort", l.extPort(), "certPorts", l.CertPorts())
		scheme = string(loadbalancer.SSL)
	} else {
		log.Debug("cert is nil, or port is not in cert ports ", "extPort", l.extPort(), "certPorts", l.CertPorts())
	}

	return loadbalancer.ProtocolFromString(scheme)
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
	if err != nil {
		return nil, fmt.Errorf("bad spec: %s for service %s", parts[1], service)
	}
	log.Debug("found swarmPort", "swarmPort", swarmPort, "V", debugV)
	listener.SwarmPort = int(swarmPort)
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
	var cert *string
	return newListener(service, int(0), serviceURL.String(), cert)
}

func serviceCert(service swarm.Service, certLabel string) *string {
	// given a service and a certLabel, look for the service Label
	// to see if there is a value for the ACM ARN for an SSL cert
	// for this listener.
	var cert *string
	if certLabel == "" {
		return cert
	}
	labelCert, hasCert := service.Spec.Labels[certLabel]
	if hasCert {
		cert = &labelCert
	}
	return cert

}

func addListenerToHostMap(m map[string][]*listener, l *listener) {
	host := l.host()
	if _, has := m[host]; !has {
		m[host] = []*listener{}
	}
	m[host] = append(m[host], l)
}

func intInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// listenersFromExposedPorts generates a list of listeners, where the URL is defaulted to the swarm port and default ELB
func listenersFromExposedPorts(service swarm.Service, certLabel string) []*listener {

	// Rule on publishing ports
	// If the user specified -p X:Y, then we publish the port to the ELB.  If the port
	// was assigned by Swarm, then we leave it alone.

	// Given -p X:Y option when starting up services, we look at what's requested and what's actually published:
	requestedPublishPorts := map[int][]int{} // key - target port Y (app/container port), value = publish port X
	if service.Spec.EndpointSpec != nil {
		for _, p := range service.Spec.EndpointSpec.Ports {
			if p.PublishedPort > 0 && strings.EqualFold(string(p.PublishMode), "ingress") {
				// Only if the user has specify the desired publish port
				requestedPublishPorts[int(p.TargetPort)] = append(requestedPublishPorts[int(p.TargetPort)], int(p.PublishedPort))
			}
		}
	}

	log.Debug("requestedPublishPorts", "requestedPublishPorts", requestedPublishPorts, "V", debugV)
	// Now look at the ports that are actually published:
	listeners := []*listener{}
	for _, exposed := range service.Endpoint.Ports {

		log.Debug("Exposed port", "exposed", exposed, "V", debugV)
		if intInSlice(int(exposed.PublishedPort), requestedPublishPorts[int(exposed.TargetPort)]) {
			cert := serviceCert(service, certLabel)

			log.Debug("Cert: ", cert, "V", debugV)
			urlString := fmt.Sprintf("%v://:%d", strings.ToLower(string(exposed.Protocol)), exposed.PublishedPort)
			log.Debug("urlString", "urlString", urlString, "V", debugV)
			if listener, err := newListener(service.Spec.Name, int(exposed.PublishedPort), urlString, cert); err == nil {
				listeners = append(listeners, listener)
			} else {
				log.Error("Error creating listener for exposed port", "exposed", exposed, "err", err)
			}

		} else {
			log.Debug("Skipping exposed port", "exposed", exposed)
		}
	}
	return listeners
}

// listenersFromLabel generates a list of listeners based on what's specified in the label.  The listeners
// are then matched against the exposed ports in the service.
func listenersFromLabel(service swarm.Service, label string, certLabel string) []*listener {
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
			log.Error("Err parsing spec", "err", err)
			continue
		}
		// need to add the certificate after it is parsed since service isn't available
		// inside of specParser.
		listener.Certificate = serviceCert(service, certLabel)

		listenersToPublish = append(listenersToPublish, listener)
	}
	return listenersToPublish
}
