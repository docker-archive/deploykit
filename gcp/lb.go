package gcp

import (
	"fmt"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/editions/pkg/loadbalancer"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	"context"
)

// NewLoadBalancerDriver creates a load balancer driver
func NewLoadBalancerDriver(name string) (loadbalancer.Driver, error) {
	log.Debugln("Create gcp load balancer driver for", name)

	project, err := findProject()
	if err != nil {
		return nil, err
	}

	lbFirewall, err := findLBFirewallRule()
	if err != nil {
		return nil, err
	}

	return &lbDriver{
		name:       name,
		project:    project,
		lbFirewall: lbFirewall,
	}, nil
}

type lbDriver struct {
	name       string
	project    string
	lbFirewall string
}

// Name is the name of the load balancer
func (d *lbDriver) Name() string {
	return d.name
}

// Routes lists all known routes.
func (d *lbDriver) Routes() ([]loadbalancer.Route, error) {
	log.Debugln("List routes for", d.Name())

	routes := []loadbalancer.Route{}

	firewall, err := d.getFirewall()
	if err != nil {
		return nil, err
	}

	for _, allowed := range firewall.Allowed {
		for _, allowedPort := range allowed.Ports {
			port, err := strconv.ParseUint(allowedPort, 10, 32)
			if err != nil {
				return nil, err
			}

			routes = append(routes, loadbalancer.Route{
				Port:             uint32(port),
				Protocol:         loadbalancer.ProtocolFromString(allowed.IPProtocol),
				LoadBalancerPort: uint32(port),
			})
		}
	}

	log.Debugln("Found", len(routes), routes)

	return routes, nil
}

// Publish publishes a route in the LB by adding a load balancing rule
func (d *lbDriver) Publish(route loadbalancer.Route) (loadbalancer.Result, error) {
	log.Debugln("Publish", route.LoadBalancerPort, "on", d.Name())

	firewall, err := d.getFirewall()
	if err != nil {
		return nil, err
	}

	for _, allowed := range firewall.Allowed {
		for _, allowedPort := range allowed.Ports {
			port, errConv := strconv.ParseUint(allowedPort, 10, 32)
			if errConv != nil {
				return nil, errConv
			}

			if uint32(port) == route.Port {
				return NewResult("Already published"), nil
			}
		}

		// Add port
		if loadbalancer.ProtocolFromString(allowed.IPProtocol) == route.Protocol {
			allowed.Ports = append(allowed.Ports, fmt.Sprintf("%d", route.Port))
		}
	}

	err = d.updateFirewall(firewall)
	if err != nil {
		return nil, err
	}

	return NewResult("Published"), nil
}

// UnpublishService dissociates the load balancer from the backend service at the given port.
func (d *lbDriver) Unpublish(extPort uint32) (loadbalancer.Result, error) {
	log.Debugln("Unpublish", extPort, "on", d.Name())

	firewall, err := d.getFirewall()
	if err != nil {
		return nil, err
	}

	found := false

	for _, allowed := range firewall.Allowed {
		allowedPorts := []string{}

		for _, allowedPort := range allowed.Ports {
			port, errConv := strconv.ParseUint(allowedPort, 10, 32)
			if errConv != nil {
				return nil, errConv
			}

			if uint32(port) == extPort {
				// Ignore port
				found = true
			} else {
				allowedPorts = append(allowedPorts, allowedPort)
			}
		}

		allowed.Ports = allowedPorts
	}

	if !found {
		return NewResult("Already unpublished"), nil
	}

	err = d.updateFirewall(firewall)
	if err != nil {
		return nil, err
	}

	return NewResult("Unpublished"), nil
}

// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
// The parameters healthy and unhealthy indicate the number of consecutive success or fail pings required to
// mark a backend instance as healthy or unhealthy.   The ping occurs on the backendPort parameter and
// at the interval specified.
func (d *lbDriver) ConfigureHealthCheck(backendPort uint32, healthy, unhealthy int, interval, timeout time.Duration) (loadbalancer.Result, error) {
	return NewResult("Not implemented"), nil
}

// RegisterBackend registers instances identified by the IDs to the LB's backend pool
func (d *lbDriver) RegisterBackend(id string, more ...string) (loadbalancer.Result, error) {
	return NewResult("Not implemented"), nil
}

// DeregisterBackend removes the specified instances from the backend pool
func (d *lbDriver) DeregisterBackend(id string, more ...string) (loadbalancer.Result, error) {
	return NewResult("Not implemented"), nil
}

func (d *lbDriver) service() (*compute.Service, error) {
	client, err := google.DefaultClient(context.Background(), compute.ComputeScope)
	if err != nil {
		return nil, err
	}

	return compute.New(client)
}

func (d *lbDriver) getFirewall() (*compute.Firewall, error) {
	service, err := d.service()
	if err != nil {
		return nil, err
	}

	return service.Firewalls.Get(d.project, d.lbFirewall).Do()
}

func (d *lbDriver) updateFirewall(firewall *compute.Firewall) error {
	service, err := d.service()
	if err != nil {
		return err
	}

	op, err := service.Firewalls.Update(d.project, d.lbFirewall, firewall).Do()
	if err != nil {
		return err
	}

	for {
		if op.Status == "DONE" {
			if op.Error != nil {
				return fmt.Errorf("Operation error: %v", *op.Error.Errors[0])
			}

			return nil
		}

		time.Sleep(1 * time.Second)

		op, err = service.GlobalOperations.Get(d.project, op.Name).Do()
		if err != nil {
			return err
		}
	}
}
