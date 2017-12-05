package ingress

import (
	"fmt"

	"github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// configureL4 configures a L4 loadbalancer with the desired routes and given options
func configureL4(elb loadbalancer.L4, desired []loadbalancer.Route, options types.Options) error {

	// Process the listeners
	routes, err := elb.Routes()
	if err != nil {
		log.Warn("Error describing L4", "err", err)
		return err
	}
	log.Debug("describe L4", "routes", routes)

	log.Debug("Listeners to sync with L4", "desired", desired)

	toCreate := []loadbalancer.Route{}
	toChange := []loadbalancer.Route{}
	toRemove := []loadbalancer.Route{}

	// Index the listener set up
	listenerIndex := map[string]loadbalancer.Route{}
	listenerIndexKey := func(p, lp loadbalancer.Protocol, extPort, instancePort int) string {
		return fmt.Sprintf("%v/%v/%5d/%5d", p, lp, extPort, instancePort)
	}

	for _, l := range desired {
		instancePort, hasListener := findRoutePort(routes, l.LoadBalancerPort, l.Protocol, l.LoadBalancerProtocol)

		if !hasListener {
			toCreate = append(toCreate, l)
		} else if instancePort != l.Port {
			toChange = append(toChange, l)
		}

		listenerIndex[listenerIndexKey(l.LoadBalancerProtocol, l.Protocol, l.LoadBalancerPort, instancePort)] = l
	}
	log.Debug("Listener", "index", listenerIndex)

	for _, route := range routes {
		instanceProtocol := route.Protocol
		lbProtocol := route.LoadBalancerProtocol
		lbPort := route.LoadBalancerPort
		instancePort := route.Port
		cert := route.Certificate

		if _, has := listenerIndex[listenerIndexKey(lbProtocol, instanceProtocol, lbPort, instancePort)]; !has {
			toRemove = append(toRemove, loadbalancer.Route{
				Port:                 instancePort,
				Protocol:             instanceProtocol,
				LoadBalancerPort:     lbPort,
				LoadBalancerProtocol: lbProtocol,
				Certificate:          cert,
			})
		} else {
			log.Debug("keeping", "protocol", lbProtocol, "instanceProtocol", instanceProtocol, "port", lbPort, "instancePort", instancePort)
		}
	}

	logFn := log.Debug
	if len(toCreate) > 0 || len(toChange) > 0 || len(toRemove) > 0 {
		logFn = log.Info
	}
	logFn("listeners to create:", "list", toCreate)
	logFn("listeners to change:", "list", toChange)
	logFn("listeners to remove:", "list", toRemove)

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
		_, err := elb.Unpublish(l.LoadBalancerPort)
		if err != nil {
			log.Warn("err unpublishing", "route", l, "err", err)
			return err
		}
		log.Info("REMOVED", "name", elb.Name(), "listener", l)
	}
	return nil
}

func findRoutePort(routes []loadbalancer.Route, loadbalancerPort int,
	protocol, loadbalancerProtocol loadbalancer.Protocol) (int, bool) {

	for _, route := range routes {
		if route.LoadBalancerPort == loadbalancerPort && route.Protocol == protocol && route.LoadBalancerProtocol == loadbalancerProtocol {
			return route.Port, true
		}
	}
	return 0, false
}
