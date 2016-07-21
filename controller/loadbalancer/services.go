package loadbalancer

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/libmachete/spi/loadbalancer"
	"time"
)

const (

	// LabelExternalLoadBalancerSpec is the label for marking a service as public facing.
	// The value is a url.  The protocol, host and port will be extracted from the URL
	// to configure the external load balancer.  The ELB will be configured to listen
	// at the specified port (or 80), using the specified protocol, and the host is used to
	// select which load balancer in the config file on the manager nodes /var/lib/docker/swarm/lb.config
	// TODO(chungers) - While the hostname is used to select the ELB to use, we will also provide support
	// for HTTP/S vhosts in the future if the hostname is not matched to an ELB, we will select a top level ELB
	// and then use the subdomain in the hostname to configure a HAProxy with http header routing.
	LabelExternalLoadBalancerSpec = "docker.swarm.lb"
)

// HealthCheck is the configuration for an operation to determine if a service is healthy.
type HealthCheck struct {
	Port            uint32
	Healthy         int
	Unhealthy       int
	IntervalSeconds int
	TimeoutSeconds  int
}

// VhostLoadBalancerMap is a function which returns a map of L4 load balancers by vhost
type VhostLoadBalancerMap func() map[string]loadbalancer.Driver

// ServiceAction defines the action to apply to a list of services
type ServiceAction func([]swarm.Service)

// Options contains options to control behavior of the ELB sync process.
type Options struct {
	RemoveListeners   bool
	HealthCheck       *HealthCheck
	PublishAllExposed bool
}

func externalLoadBalancerListenersFromServices(services []swarm.Service, label string,
	options Options) map[string][]*Listener {

	// group the listeners by hostname.  hostname maps to a ELB somewhere else.
	listeners := map[string][]*Listener{}
	for _, s := range services {

		// We index all the exposed ports by swarm port
		exposedPorts := map[uint32]swarm.PortConfig{}
		for _, exposed := range s.Endpoint.Ports {
			exposedPorts[exposed.PublishedPort] = exposed
		}

		// Now go through the list that we need to publish and match up the exposed ports
		for _, publish := range listenersFromLabel(s, label) {

			if sp, has := exposedPorts[publish.SwarmPort]; has {

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
					publish.SwarmPort = exposed.PublishedPort
					break // Just grab the first one
				}
				addListenerToHostMap(listeners, publish)

			} else {

				// There are unresolved publishing listeners
				log.Warningln("Cannot match exposed port in service", s.Spec.Name, "for", publish)

			}
		}

		if options.PublishAllExposed {
			for _, l := range listenersFromExposedPorts(s) {
				addListenerToHostMap(listeners, l)
			}
		}

	}
	return listeners
}

func configureL4(elb loadbalancer.Driver, listeners []*Listener, options Options) error {
	// Process the listeners
	describe, err := elb.State()
	if err != nil {
		log.Warningln("Error describing ELB err=", err)
		return err
	}
	log.Debugln("describe elb=", describe)

	log.Infoln("Listeners to sync with ELB:", listeners)
	toCreate := []*Listener{}
	toChange := []*Listener{}
	toRemove := []*Listener{}

	// Index the listener set up
	listenerIndex := map[string]*Listener{}
	listenerIndexKey := func(p loadbalancer.Protocol, extPort, instancePort uint32) string {
		return fmt.Sprintf("%v/%5d/%5d", p, extPort, instancePort)
	}

	for _, l := range listeners {
		instancePort, hasListener := describe.HasListener(l.ExtPort(), l.Protocol())

		if !hasListener {
			toCreate = append(toCreate, l)
		} else if instancePort != l.SwarmPort {
			toChange = append(toChange, l)
		}

		listenerIndex[listenerIndexKey(l.Protocol(), l.ExtPort(), instancePort)] = l
	}
	log.Debugln("ListenerIndex=", listenerIndex)

	describe.VisitListeners(
		func(lbPort, instancePort uint32, protocol loadbalancer.Protocol) {
			if _, has := listenerIndex[listenerIndexKey(protocol, lbPort, instancePort)]; !has {

				l, err := NewListener("delete", instancePort, fmt.Sprintf("%v://:%d", protocol, lbPort))
				if err == nil {
					toRemove = append(toRemove, l)
				} else {
					log.Warningln("error deleting listener protocol=", protocol, "lbPort=", lbPort, "instancePort=", instancePort)
				}
			} else {
				log.Infoln("keeping protocol=", protocol, "port=", lbPort, "instancePort=", instancePort)
			}
		})

	log.Infoln("listeners to create:", toCreate)
	log.Infoln("listeners to change:", toChange)
	log.Infoln("listeners to remove:", toRemove)

	// Now we have a list of targets to create
	for _, l := range toCreate {
		log.Infoln("CREATE on", elb.Name(), "listener", l)

		_, err := elb.PublishService(l.Protocol(), l.ExtPort(), l.Protocol(), l.SwarmPort) // No SSL cert yet..
		if err != nil {
			log.Warningln("err unpublishing", l, "err=", err)
			return err
		}
		log.Infoln("CREATED on", elb.Name(), "listener", l)
	}
	for _, l := range toChange {
		log.Infoln("CHANGE on", elb.Name(), "listener", l)

		_, err := elb.UnpublishService(l.ExtPort())
		if err != nil {
			log.Warningln("err unpublishing", l, "err=", err)
			return err
		}
		_, err = elb.PublishService(l.Protocol(), l.ExtPort(), l.Protocol(), l.SwarmPort) // No SSL cert yet..
		if err != nil {
			log.Warningln("err publishing", l, "err=", err)
			return err
		}
		log.Infoln("CHANGED on", elb.Name(), "listener", l)
	}
	for _, l := range toRemove {
		log.Infoln("REMOVE on", elb.Name(), "listener", l)

		if options.RemoveListeners {
			_, err := elb.UnpublishService(l.ExtPort())
			if err != nil {
				log.Warningln("err unpublishing", l, "err=", err)
				return err
			}
			log.Infoln("REMOVED on", elb.Name(), "listener", l)
		}
	}

	// Configure health check.
	// ELB only has one health check port and that determines if the backend is out of service or not.
	// This presents a problem where if we have more than one service, the ELB may think a service is down
	// when only one of our services is out.  We probably need to have a way to do health checks on the services
	// ourselves and then update the health check when we detect that one of the services is down so that ELB doesn't
	// shut everything down.

	if options.HealthCheck != nil {
		options.HealthCheck.Port = 0
		if len(toCreate) > 0 {
			options.HealthCheck.Port = toCreate[0].SwarmPort
		} else if len(toChange) > 0 {
			options.HealthCheck.Port = toChange[0].SwarmPort
		}

		if options.HealthCheck.Port > 0 {
			log.Infoln("HEALTH CHECK - Configuring the health check to ping port:", options.HealthCheck.Port)
			_, err := elb.ConfigureHealthCheck(options.HealthCheck.Port,
				options.HealthCheck.Healthy, options.HealthCheck.Unhealthy,
				time.Duration(options.HealthCheck.IntervalSeconds)*time.Second,
				time.Duration(options.HealthCheck.TimeoutSeconds)*time.Second)
			if err != nil {
				log.Warningln("err config health check, err=", err)
				return err
			}
			log.Infoln("HEALTH CHECK CONFIGURED on port:", options.HealthCheck.Port, "config=", options.HealthCheck)
		}
	}
	return nil
}

// ExposeServicePortInExternalLoadBalancer creates a ServiceAction to expose a service in an ELB.
func ExposeServicePortInExternalLoadBalancer(elbMap VhostLoadBalancerMap, options Options) ServiceAction {

	return func(services []swarm.Service) {

		// to avoid multiple dates when ELBs have aliases need to agregate all of them by elb than just hostname
		// since different hostnames can point to the same ELB.
		targets := map[loadbalancer.Driver][]*Listener{}

		listenersByHost := externalLoadBalancerListenersFromServices(services, LabelExternalLoadBalancerSpec, options)

		// Need to process for each ELB known because it's possible that we'd have to remove all listeners in an ELB.
		// when there are no listeners to be created from all the services.
		elbs := elbMap()
		for hostname, elb := range elbs {
			if _, has := targets[elb]; !has {
				add := []*Listener{}
				if list, exists := listenersByHost[hostname]; exists {
					add = list
				}
				targets[elb] = add
			} else {
				if list, exists := listenersByHost[hostname]; exists {
					for _, l := range list {
						targets[elb] = append(targets[elb], l)
					}
				}
			}
		}

		log.Debugln("Targets=", len(targets), "targets=", targets)
		if len(targets) == 0 {
			// This is the case when there are absolutely no services in the swarm... we need
			// synchronize and clean up any unmanaged listeners.
			cleaned := map[string]interface{}{}
			for _, elb := range elbs {
				if _, has := cleaned[elb.Name()]; !has {
					err := configureL4(elb, []*Listener{}, options)
					if err != nil {
						log.Warning("Cannot clean up ELB %s:", elb.Name())
						continue
					}
					cleaned[elb.Name()] = nil
				}
			}

		} else {
			for elb, listeners := range targets {
				log.Infoln("Configuring", elb.Name())
				err := configureL4(elb, listeners, options)
				if err != nil {
					log.Warning("Cannot configure ELB %s with listeners:", elb.Name(), listeners)
					continue
				}
			}
		}
	}
}
