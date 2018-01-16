package ingress

import (
	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/controller/ingress/types"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/template"
)

func (c *managed) syncRoutesL4() error {
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

	log.Debug("expose l4", "targets", len(targets), "targets", targets, "meta", c.spec.Metadata)

	for elb, routes := range targets {
		log.Debug("Configuring", "name", elb.Name(), "routes", routes)
		err := configureL4(elb, routes, c.options)
		if err != nil {
			log.Warn("Cannot configure L4", "name", elb.Name(), "routes", routes, "meta", c.spec.Metadata)
			continue
		}
	}
	return nil
}

func (c *managed) getSourceKeySelectorTemplate() (*template.Template, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.options.SourceKeySelector != "" {
		if c.sourceKeySelectorTemplate == nil {
			t, err := types.TemplateFrom([]byte(c.options.SourceKeySelector))
			if err != nil {
				return nil, err
			}
			c.sourceKeySelectorTemplate = t
		}
	}

	return c.sourceKeySelectorTemplate, nil
}

func (c *managed) syncBackends() error {
	groupsByVhost, err := c.groups()
	log.Debug("Groups by vhost", "groups", groupsByVhost, "meta", c.spec.Metadata, "fsm", c.stateMachine.ID())

	if err != nil {
		return err
	}

	loadbalancersByVhost, err := c.l4s()
	log.Debug("L4s by vhost", "l4s", loadbalancersByVhost)
	if err != nil {
		return err
	}

	// process by vhost and loadbalancer and keep a list of unresolved vhosts
	// for which we do not have any backends

	unresolved := []types.Vhost{}
	for vhost, l4 := range loadbalancersByVhost {

		groups, has := groupsByVhost[vhost]
		if !has {
			unresolved = append(unresolved, vhost)
			continue
		}

		// we have backends and loadbalancers
		registered := mapset.NewSet()
		if backends, err := l4.Backends(); err != nil {
			log.Warn("error getting backends", "err", err, "meta", c.spec.Metadata)
			continue
		} else {
			for _, b := range backends {
				registered.Add(b)
			}
		}
		log.Debug("Registered backends", "backends", registered, "meta", c.spec.Metadata)

		// all the nodes from all the groups and nodes
		nodes := mapset.NewSet()

		instanceIDs, _ := c.instanceIDs()
		for _, id := range instanceIDs[vhost] {
			nodes.Add(id)
		}

		log.Debug("backend groups", "groups", groups, "meta", c.spec.Metadata)
		for _, g := range groups {

			gid := g.ID()
			groupPlugin, err := c.groupPlugin(g)
			if err != nil {
				return err
			}

			desc, err := groupPlugin.DescribeGroup(gid)
			if err != nil {
				// Failed to describe the group, since we do not know the members we do not want to proceed
				log.Warn("error describing group, not syncing backends", "id", gid, "err", err, "meta", c.spec.Metadata)
				return err
			}

			log.Debug("found backends", "groupID", gid, "desc", desc, "vhost", vhost, "L4", l4.Name(), "meta", c.spec.Metadata)

			for _, inst := range desc.Instances {
				t, err := c.getSourceKeySelectorTemplate()
				if err != nil {
					return err
				}
				if t == nil {
					nodes.Add(inst.ID)
				} else {
					view, err := t.Render(inst)
					if err != nil {
						log.Error("cannot index entry", "instance.ID", inst.ID, "instance.tags", inst.Tags, "err", err, "meta", c.spec.Metadata)
						continue
					}
					nodes.Add(instance.ID(view))
				}
			}
		}

		log.Debug("Group data", "nodes", nodes)

		// compute the difference between registered and nodes
		toRemove := []instance.ID{}
		for n := range registered.Difference(nodes).Iter() {
			toRemove = append(toRemove, n.(instance.ID))
		}

		// Use Info logging only when making deltas
		logFn := log.Debug
		if len(toRemove) > 0 {
			logFn = log.Info
		}
		logFn("De-register backends", "instances", toRemove, "vhost", vhost, "L4", l4.Name(), "meta", c.spec.Metadata)

		if result, err := l4.DeregisterBackends(toRemove); err != nil {
			log.Warn("error deregistering backends", "toRemove", toRemove, "err", err, "meta", c.spec.Metadata)
		} else {
			logFn("deregistered backends", "vhost", vhost, "result", result, "meta", c.spec.Metadata)
		}

		toAdd := []instance.ID{}
		for n := range nodes.Difference(registered).Iter() {
			toAdd = append(toAdd, n.(instance.ID))
		}

		logFn = log.Debug
		if len(toAdd) > 0 {
			logFn = log.Info
		}

		logFn("Register backends", "instances", toAdd, "vhost", vhost, "L4", l4.Name(), "meta", c.spec.Metadata)
		if result, err := l4.RegisterBackends(toAdd); err != nil {
			log.Warn("error registering backends", "toAdd", toAdd, "err", err, "meta", c.spec.Metadata)
		} else {
			logFn("registered backends", "vhost", vhost, "result", result, "meta", c.spec.Metadata)
		}

	}

	return nil

}

func (c *managed) syncHealthChecks() error {
	targets := map[loadbalancer.L4][]loadbalancer.HealthCheck{}
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

	log.Debug("configure healthchecks", "targets", targets, "meta", c.spec.Metadata)

	for elb, healthChecks := range targets {
		log.Debug("Configuring healthcheck", "name", elb.Name(), "meta", c.spec.Metadata)
		for _, healthCheck := range healthChecks {
			if healthCheck.BackendPort > 0 {
				log.Info("HEALTH CHECK - Configuring the health check to ping", "port", healthCheck.BackendPort, "meta", c.spec.Metadata)
				_, err := elb.ConfigureHealthCheck(healthCheck)

				if err != nil {
					log.Warn("err config health check", "err", err)
					return err
				}
				log.Info("HEALTH CHECK CONFIGURED", "port", healthCheck.BackendPort, "config", healthCheck, "meta", c.spec.Metadata)
			}
		}
	}
	return nil
}
