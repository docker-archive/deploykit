package ingress

import (
	"fmt"

	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// configureL4 configures a L4 loadbalancer with the desired routes and given options
func configureL4(elb loadbalancer.L4, desired []loadbalancer.Route, options Options) error {

	// Process the listeners
	routes, err := elb.Routes()
	if err != nil {
		log.Warn("Error describing L4", "err", err)
		return err
	}
	log.Debug("describe L4", "routes", routes)

	log.Info("Listeners to sync with L4:", "desired", desired)
	toCreate := []loadbalancer.Route{}
	toChange := []loadbalancer.Route{}
	toRemove := []loadbalancer.Route{}

	// Index the listener set up
	listenerIndex := map[string]loadbalancer.Route{}
	listenerIndexKey := func(p loadbalancer.Protocol, extPort, instancePort uint32) string {
		return fmt.Sprintf("%v/%5d/%5d", p, extPort, instancePort)
	}

	for _, l := range desired {
		instancePort, hasListener := findRoutePort(routes, l.LoadBalancerPort, l.Protocol)

		if !hasListener {
			toCreate = append(toCreate, l)
		} else if instancePort != l.Port {
			toChange = append(toChange, l)
		}

		listenerIndex[listenerIndexKey(l.Protocol, l.LoadBalancerPort, instancePort)] = l
	}
	log.Debug("Listener", "index", listenerIndex)

	for _, route := range routes {
		protocol := route.Protocol
		lbPort := route.LoadBalancerPort
		instancePort := route.Port
		cert := route.Certificate

		if _, has := listenerIndex[listenerIndexKey(protocol, lbPort, instancePort)]; !has {
			toRemove = append(toRemove, loadbalancer.Route{
				Port:             instancePort,
				Protocol:         protocol,
				LoadBalancerPort: lbPort,
				Certificate:      cert,
			})
		} else {
			log.Info("keeping", "protocol", protocol, "port", lbPort, "instancePort", instancePort)
		}
	}

	log.Info("listeners to create:", "list", toCreate)
	log.Info("listeners to change:", "list", toChange)
	log.Info("listeners to remove:", "list", toRemove)

	// Now we have a list of targets to create
	for _, l := range toCreate {
		log.Info("CREATE", "name", elb.Name(), "listener", l)

		_, err := elb.Publish(l) // No SSL cert yet..
		if err != nil {
			log.Warn("err publishing", "route", l, "err", err)
			return err
		}
		log.Info("CREATED", "name", elb.Name(), "listener", l)
	}
	for _, l := range toChange {
		log.Info("CHANGE", "name", elb.Name(), "listener", l)

		_, err := elb.Unpublish(l.LoadBalancerPort)
		if err != nil {
			log.Warn("err unpublishing", "route", l, "err", err)
			return err
		}
		_, err = elb.Publish(l) // No SSL cert yet..
		if err != nil {
			log.Warn("err publishing", "route", l, "err", err)
			return err
		}
		log.Info("CHANGED", "name", elb.Name(), "listener", l)
	}
	for _, l := range toRemove {
		log.Info("REMOVE", "name", elb.Name(), "listener", l)

		if options.RemoveListeners {
			_, err := elb.Unpublish(l.LoadBalancerPort)
			if err != nil {
				log.Warn("err unpublishing", "route", l, "err", err)
				return err
			}
			log.Info("REMOVED", "name", elb.Name(), "listener", l)
		}
	}
	return nil
}

func findRoutePort(routes []loadbalancer.Route, loadbalancerPort uint32,
	protocol loadbalancer.Protocol) (uint32, bool) {

	for _, route := range routes {
		if route.LoadBalancerPort == loadbalancerPort && route.Protocol == protocol {
			return route.Port, true
		}
	}
	return 0, false
}
