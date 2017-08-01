package ingress

import (
	"time"

	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

func (c *Controller) syncRoutesL4() error {
	// to avoid multiple updates when ELBs have aliases need to agregate all of them by elb than just hostname
	// since different hostnames can point to the same ELB.
	targets := map[loadbalancer.L4][]loadbalancer.Route{}
	routesByVhost, err := c.routes()
	if err != nil {
		return err
	}

	// Need to process for each ELB known because it's possible that we'd have to remove all listeners in an ELB.
	// when there are no listeners to be created from all the services.
	elbs, err := c.l4s()
	if err != nil {
		return err
	}
	for vhost, elb := range elbs {
		// If a given elb does not have any vhosts and thus routes associated with it, it will have
		// an entry in the targets map, but with no routes.  Then the empty routes slice will be sent
		// to the elb to effectively remove the routes/ listeners.
		targets[elb] = append(targets[elb], routesByVhost[vhost]...)
	}

	log.Debug("expose l4", "targets", len(targets), "targets", targets)

	for elb, routes := range targets {
		log.Info("Configuring", "name", elb.Name(), "routes", routes)
		err := configureL4(elb, routes, c.options)
		if err != nil {
			log.Warn("Cannot configure L4", "name", elb.Name(), "routes", routes)
			continue
		}
	}
	return nil
}

func (c *Controller) syncBackends() error {
	groupsByVhost, err := c.groups()
	if err != nil {
		return err
	}

	loadbalancersByVhost, err := c.l4s()
	if err != nil {
		return err
	}

	// process by vhost and loadbalancer and keep a list of unresolved vhosts
	// for which we do not have any backends

	unresolved := []types.Vhost{}
	for vhost, l4 := range loadbalancersByVhost {

		groupIDs, has := groupsByVhost[vhost]
		if !has {
			unresolved = append(unresolved, vhost)
			continue
		}

		// we have backends and loadbalancers
		registered := mapset.NewSet()
		if backends, err := l4.Backends(); err != nil {
			log.Warn("error getting backends", "err", err)
			continue
		} else {
			for _, b := range backends {
				registered.Add(b)
			}
		}

		// all the nodes from all the groups and nodes
		nodes := mapset.NewSet()

		instanceIDs, _ := c.instanceIDs()
		for _, id := range instanceIDs[vhost] {
			nodes.Add(id)
		}

		groupPlugin, err := c.groupPlugin()
		if err != nil {
			return err
		}
		for _, gid := range groupIDs {

			desc, err := groupPlugin.DescribeGroup(gid)
			if err != nil {
				log.Warn("error describing group", "id", gid, "err", err)
				continue
			}

			for _, inst := range desc.Instances {
				nodes.Add(inst.ID)
			}
		}

		// compute the difference between registered and nodes
		list := []instance.ID{}
		for n := range registered.Difference(nodes).Iter() {
			list = append(list, n.(instance.ID))
		}
		if result, err := l4.RegisterBackends(list); err != nil {
			log.Warn("error registering backends", "err", err)
		} else {
			log.Info("registered backends", "vhost", vhost, "result", result)
		}

		list = []instance.ID{}
		for n := range nodes.Difference(registered).Iter() {
			list = append(list, n.(instance.ID))
		}
		if result, err := l4.DeregisterBackends(list); err != nil {
			log.Warn("error de-registering backends", "err", err)
		} else {
			log.Info("deregistered backends", "vhost", vhost, "result", result)
		}

	}

	return nil

}

func (c *Controller) syncHealthChecks() error {
	targets := map[loadbalancer.L4][]types.HealthCheck{}
	healthChecksByVhost, err := c.healthChecks()
	if err != nil {
		return err
	}

	// Need to process for each ELB known because it's possible that we'd have to remove all listeners in an ELB.
	// when there are no listeners to be created from all the services.
	elbs, err := c.l4s()
	if err != nil {
		return err
	}
	for vhost, elb := range elbs {
		targets[elb] = append(targets[elb], healthChecksByVhost[vhost]...)
	}

	log.Debug("configure healthchecks", "targets", targets)

	for elb, healthChecks := range targets {
		log.Info("Configuring healthcheck", "name", elb.Name())
		for _, healthCheck := range healthChecks {
			if healthCheck.Port > 0 {
				log.Info("HEALTH CHECK - Configuring the health check to ping", "port", healthCheck.Port)
				_, err := elb.ConfigureHealthCheck(healthCheck.Port,
					healthCheck.Healthy, healthCheck.Unhealthy,
					time.Duration(healthCheck.IntervalSeconds)*time.Second,
					time.Duration(healthCheck.TimeoutSeconds)*time.Second)
				if err != nil {
					log.Warn("err config health check", "err", err)
					return err
				}
				log.Info("HEALTH CHECK CONFIGURED", "port", healthCheck.Port, "config", healthCheck)
			}
		}
	}
	return nil
}
