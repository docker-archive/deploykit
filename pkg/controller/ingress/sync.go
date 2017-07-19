package ingress

import (
	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// SyncRoutes returns a work function that can be used in the poller
func SyncRoutes(routes func() (map[Vhost][]loadbalancer.Route, error),
	loadbalancers func() (map[Vhost]loadbalancer.L4, error),
	configure func(loadbalancer.L4, []loadbalancer.Route, Options) error,
	options Options) func() error {

	return func() error {
		// to avoid multiple updates when ELBs have aliases need to agregate all of them by elb than just hostname
		// since different hostnames can point to the same ELB.
		targets := map[loadbalancer.L4][]loadbalancer.Route{}
		listenersByHost, err := routes()
		if err != nil {
			return err
		}

		// Need to process for each ELB known because it's possible that we'd have to remove all listeners in an ELB.
		// when there are no listeners to be created from all the services.
		elbs, err := loadbalancers()
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
			// This is the case when there are absolutely no routes to publish... we need
			// synchronize and clean up any unmanaged listeners.
			cleaned := map[string]interface{}{}
			for _, elb := range elbs {
				if _, has := cleaned[elb.Name()]; !has {
					err := configure(elb, []loadbalancer.Route{}, options)
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
			err := configure(elb, listeners, options)
			if err != nil {
				log.Warn("Cannot configure L4", "name", elb.Name(), "listeners", listeners)
				continue
			}
		}
		return nil
	}
}

// SyncBackends returns a work function that can be used in the poller
func SyncBackends(groupPlugin group.Plugin,
	groups func() (map[Vhost][]group.ID, error),
	loadbalancers func() (map[Vhost]loadbalancer.L4, error),
	configure func(loadbalancer.L4, []instance.ID, Options) error,
	options Options) func() error {

	return func() error {

		groupsByVhost, err := groups()
		if err != nil {
			return err
		}

		loadbalancersByVhost, err := loadbalancers()
		if err != nil {
			return err
		}

		// process by vhost and loadbalancer and keep a list of unresolved vhosts
		// for which we do not have any backends

		unresolved := []Vhost{}
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

			nodes := mapset.NewSet()
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
}
