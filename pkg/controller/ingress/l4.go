package ingress

import (
	"github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

type operation int

const (
	createOp operation = iota
	changeOp
	noop
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

	for _, l := range desired {
		op := getRouteOperation(&routes, l)

		switch op {
		case createOp:
			toCreate = append(toCreate, l)
		case changeOp:
			toChange = append(toChange, l)
		}
	}

	logFn := log.Debug
	if len(toCreate) > 0 || len(toChange) > 0 || len(routes) > 0 {
		logFn = log.Info
	}
	logFn("listeners to create:", "list", toCreate)
	logFn("listeners to change:", "list", toChange)
	logFn("listeners to remove:", "list", routes)

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
	// Anything left in the routes list should be removed
	for _, l := range routes {

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

func getRouteOperation(routes *[]loadbalancer.Route, desired loadbalancer.Route) operation {
	var desiredCert, desiredHmPath string
	if desired.Certificate != nil {
		desiredCert = *desired.Certificate
	}
	if desired.HealthMonitorPath != nil {
		desiredHmPath = *desired.HealthMonitorPath
	}
	for index, route := range *routes {
		var routeHmPath, routeCert string
		if route.Certificate != nil {
			routeCert = *route.Certificate
		}
		if route.HealthMonitorPath != nil {
			routeHmPath = *route.HealthMonitorPath
		}
		if route.LoadBalancerPort == desired.LoadBalancerPort &&
			route.Protocol == desired.Protocol &&
			route.LoadBalancerProtocol == desired.LoadBalancerProtocol {
			// Found a match, remove from the list
			*routes = append((*routes)[:index], (*routes)[index+1:]...)
			if route.Port != desired.Port || routeCert != desiredCert || routeHmPath != desiredHmPath {
				return changeOp
			}
			return noop
		}
	}
	return createOp
}
