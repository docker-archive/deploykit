package loadbalancer

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/swarm"
)

type listener struct {
	Service       string
	URL           *url.URL
	SwarmPort     uint32
	SwarmProtocol Protocol
	Certificate   *string
}

func newListener(service string, swarmPort uint32, urlStr string, cert *string) (*listener, error) {
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

func (l *listener) asRoute() Route {
	return Route{
		Port:             l.SwarmPort,
		Protocol:         l.protocol(),
		LoadBalancerPort: l.extPort(),
		Certificate:      l.CertASN(),
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
func (l *listener) CertPorts() []uint32 {
	if l.Certificate == nil {
		// if we have not Certificate then return 443
		return []uint32{443}
	}
	parts := strings.Split(*l.Certificate, "@")
	if len(parts) > 1 {
		var finalPorts = []uint32{}
		ports := strings.Split(parts[1], ",")
		log.Infoln("ports ", ports)
		if len(ports) > 0 {
			log.Infoln("# of ports ", len(ports))
			for _, port := range ports {
				log.Infoln("port: ", port)
				if port != "" {
					j, err := strconv.ParseUint(port, 10, 32)
					if err != nil {
						log.Infoln("Can't convert to int: ", port, "; error=", err)
					}
					finalPorts = append(finalPorts, uint32(j))
				}
			}
		}
		log.Infoln("finalPorts ", finalPorts)
		if len(finalPorts) == 0 {
			// this would happen if there was an ASN like this
			// asn:blah@
			// an at symbol but no ports after, if that is the case
			// default to 443.
			finalPorts = append(finalPorts, uint32(443))
		}
		return finalPorts
	} else {
		// if there is no port, default to 443
		return []uint32{443}
	}
}

// String value of the listener, good for log statements.
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

	// check if this should be SSL because it has a certificate.
	if l.Certificate != nil && intInSlice(l.extPort(), l.CertPorts()) {
		log.Infoln("port ", l.extPort(), " Is in ", l.CertPorts())
		scheme = string(SSL)
	} else {
		log.Infoln("cert is nil, or port ", l.extPort(), " Is NOT in ", l.CertPorts())
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
	if err != nil {
		return nil, fmt.Errorf("bad spec: %s for service %s", parts[1], service)
	}
	log.Infoln("swarmPort: ", swarmPort)
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
	var cert *string
	return newListener(service, uint32(0), serviceURL.String(), cert)
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

func intInSlice(a uint32, list []uint32) bool {
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
	requestedPublishPorts := map[uint32][]uint32{} // key - target port Y (app/container port), value = publish port X
	if service.Spec.EndpointSpec != nil {
		for _, p := range service.Spec.EndpointSpec.Ports {
			if p.PublishedPort > 0 && strings.EqualFold(string(p.PublishMode), "ingress") {
				// Only if the user has specify the desired publish port
				requestedPublishPorts[p.TargetPort] = append(requestedPublishPorts[p.TargetPort], p.PublishedPort)
			}
		}
	}

	log.Infoln("requestedPublishPorts: ", requestedPublishPorts)
	// Now look at the ports that are actually published:
	listeners := []*listener{}
	for _, exposed := range service.Endpoint.Ports {

		log.Infoln("exposed: ", exposed)
		if intInSlice(exposed.PublishedPort, requestedPublishPorts[exposed.TargetPort]) {
			cert := serviceCert(service, certLabel)

			log.Infoln("Cert: ", cert)
			urlString := fmt.Sprintf("%v://:%d", strings.ToLower(string(exposed.Protocol)), exposed.PublishedPort)
			log.Infoln("urlString: ", urlString)
			if listener, err := newListener(service.Spec.Name, exposed.PublishedPort, urlString, cert); err == nil {
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
			log.Warningln("Error=", err)
			continue
		}
		// need to add the certificate after it is parsed since service isn't available
		// inside of specParser.
		listener.Certificate = serviceCert(service, certLabel)

		listenersToPublish = append(listenersToPublish, listener)
	}
	return listenersToPublish
}
