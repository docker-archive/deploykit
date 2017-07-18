package ingress

import (
	"fmt"
	"time"

	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

func findRoutePort(routes []loadbalancer.Route, loadbalancerPort uint32,
	protocol loadbalancer.Protocol) (uint32, bool) {

	for _, route := range routes {
		if route.LoadBalancerPort == loadbalancerPort && route.Protocol == protocol {
			return route.Port, true
		}
	}
	return 0, false
}

// ConfigureL4 configures a L4 loadbalancer with the desired routes and given options
func ConfigureL4(elb loadbalancer.L4, desired []loadbalancer.Route, options Options) error {
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

	// Configure health check.
	// L4 only has one health check port and that determines if the backend is out of service or not.
	// This presents a problem where if we have more than one service, the L4 may think a service is down
	// when only one of our services is out.  We probably need to have a way to do health checks on the services
	// ourselves and then update the health check when we detect that one of the services is down so that L4 doesn't
	// shut everything down.

	if options.HealthCheck != nil {
		options.HealthCheck.Port = 0
		if len(toCreate) > 0 {
			options.HealthCheck.Port = toCreate[0].Port
		} else if len(toChange) > 0 {
			options.HealthCheck.Port = toChange[0].Port
		}

		if options.HealthCheck.Port > 0 {
			log.Info("HEALTH CHECK - Configuring the health check to ping", "port", options.HealthCheck.Port)
			_, err := elb.ConfigureHealthCheck(options.HealthCheck.Port,
				options.HealthCheck.Healthy, options.HealthCheck.Unhealthy,
				time.Duration(options.HealthCheck.IntervalSeconds)*time.Second,
				time.Duration(options.HealthCheck.TimeoutSeconds)*time.Second)
			if err != nil {
				log.Warn("err config health check", "err", err)
				return err
			}
			log.Info("HEALTH CHECK CONFIGURED", "port", options.HealthCheck.Port, "config", options.HealthCheck)
		}
	}
	return nil
}

// ExposeL4 configures the L4 loadbalancers
func ExposeL4(elbMap func() (map[Vhost]loadbalancer.L4, error),
	routes func() (map[Vhost][]loadbalancer.Route, error),
	options Options) error {

	// to avoid multiple updates when ELBs have aliases need to agregate all of them by elb than just hostname
	// since different hostnames can point to the same ELB.
	targets := map[loadbalancer.L4][]loadbalancer.Route{}
	listenersByHost, err := routes()
	if err != nil {
		return err
	}

	// Need to process for each ELB known because it's possible that we'd have to remove all listeners in an ELB.
	// when there are no listeners to be created from all the services.
	elbs, err := elbMap()
	if err != nil {
		return err
	}
	for hostname, elb := range elbs {
		if _, has := targets[elb]; !has {
			add := []loadbalancer.Route{}
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

	log.Debug("expose l4", "targets", len(targets), "targets", targets)

	if len(targets) == 0 {
		// This is the case when there are absolutely no services in the swarm... we need
		// synchronize and clean up any unmanaged listeners.
		cleaned := map[string]interface{}{}
		for _, elb := range elbs {
			if _, has := cleaned[elb.Name()]; !has {
				err := ConfigureL4(elb, []loadbalancer.Route{}, options)
				if err != nil {
					log.Warn("Cannot clean up L4", "name", elb.Name())
					continue
				}
				cleaned[elb.Name()] = nil
			}
		}

		return nil
	}

	for elb, listeners := range targets {
		log.Info("Configuring", "name", elb.Name())
		err := ConfigureL4(elb, listeners, options)
		if err != nil {
			log.Warn("Cannot configure L4", "name", elb.Name(), "listeners", listeners)
			continue
		}
	}
	return nil
}
