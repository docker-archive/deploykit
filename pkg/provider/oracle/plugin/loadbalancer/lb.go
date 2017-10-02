package oracle

import (
	"fmt"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/provider/oracle/client/core"
	olb "github.com/docker/infrakit/pkg/provider/oracle/client/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/ryanuber/go-glob"
)

var (
	log    = logutil.New("module", "oracle/loadbalancer")
	debugV = logutil.V(400)
)

type lbDriver struct {
	client      *olb.Client
	name        string
	componentID string
	stack       string
}

// Options - options for Oracle
type Options struct {
	UserID      string
	ComponentID string
	TenancyID   string
	Fingerprint string
	KeyFile     string
	Region      string
	StackName   string
}

// NewLoadBalancerDriver creates a load balancer driver
func NewLoadBalancerDriver(client *olb.Client, name string, options *Options) (loadbalancer.L4, error) {
	return &lbDriver{
		client:      client,
		name:        name,
		componentID: options.ComponentID,
		stack:       options.StackName,
	}, nil
}

// Name is the name of the load balancer
func (d *lbDriver) Name() string {
	return d.name
}

// Routes lists all known routes.
func (d *lbDriver) Routes() ([]loadbalancer.Route, error) {
	// List all loadbalancers to filter out the main one
	lbOC, bmcErr := d.client.GetLoadBalancer(d.name)
	if bmcErr != nil {
		log.Error("error", "err", bmcErr)
		return nil, bmcErr
	}
	routes := []loadbalancer.Route{}
	for _, backendSet := range lbOC.BackendSets {
		log.Debug("routes", "backendSet", backendSet)
		routes = append(routes, loadbalancer.Route{
			Port:             backendSet.HealthChecker.Port,
			Protocol:         loadbalancer.ProtocolFromString(backendSet.HealthChecker.Protocol),
			LoadBalancerPort: backendSet.HealthChecker.Port,
		})
	}

	return []loadbalancer.Route{}, nil
}

// Publish publishes a route in the LB by adding a load balancing rule
func (d *lbDriver) Publish(route loadbalancer.Route) (loadbalancer.Result, error) {
	log.Debug("Publish", "port", route.LoadBalancerPort, "name", d.name)
	name := fmt.Sprintf("%s-%d-%d", route.Protocol, route.LoadBalancerPort, route.Port)
	frontendPort := int(route.LoadBalancerPort)
	backendPort := int(route.Port)
	timeoutMilliseconds := int(5 * 60 * 100)

	backends, err := d.backendServers(backendPort)
	if err != nil {
		return nil, err
	}

	backendSet := &olb.BackendSet{
		Name:   name,
		Policy: "ROUND_ROBIN",
		HealthChecker: &olb.HealthChecker{
			Protocol: string(route.Protocol),
			Port:     backendPort,
			URLPath:  "/",
			Timeout:  timeoutMilliseconds,
		},
		Backends: backends,
	}
	// Create backendSet
	if ok, err := d.client.CreateBackendSet(d.name, backendSet); !ok {
		log.Error("Failed to create BackendSet config", "err", err.Message)
	}
	listener := &olb.Listener{
		BackendSetName: name,
		Name:           fmt.Sprintf("%s-listener", name),
		Port:           frontendPort,
		Protocol:       string(route.Protocol),
	}
	if ok, err := d.client.CreateListener(d.name, listener); !ok {
		log.Error("Failed to create Listener config", "err", err)
	}

	return Response(fmt.Sprintf("Frontend Port %d mapped to Backend Port %d published", frontendPort, backendPort)), nil
}

// Unpublish dissociates the load balancer from the backend service at the given port.
func (d *lbDriver) Unpublish(extPort int) (loadbalancer.Result, error) {
	// remove listener & remove backendSet
	port := int(extPort)
	name := fmt.Sprintf("%s-%d-%d", string(loadbalancer.HTTP), port, port)
	if ok, err := d.client.DeleteListener(d.name, fmt.Sprintf("%s-listener", name)); !ok {
		log.Error("Failed to delete Listener", "err", err)
	} else {
		log.Info("Deleted listener for instance", "extPort", extPort)
	}
	if ok, err := d.client.DeleteBackendSet(d.name, name); !ok {
		log.Error("Failed to delete BackendSet", "err", err)
	}
	return Response(fmt.Sprintf("Frontend and Backend Port %d have been unpublished", port)), nil
}

// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
// The parameters healthy and unhealthy indicate the number of consecutive success or fail pings required to
// mark a backend instance as healthy or unhealthy.   The ping occurs on the backendPort parameter and
// at the interval specified.
func (d *lbDriver) ConfigureHealthCheck(hc loadbalancer.HealthCheck) (loadbalancer.Result, error) {
	return nil, fmt.Errorf("not-implemented")
}

// RegisterBackend registers instances identified by the IDs to the LB's backend pool
func (d *lbDriver) RegisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	return nil, fmt.Errorf("not-implemented")
}

// DeregisterBackend removes the specified instances from the backend pool
func (d *lbDriver) DeregisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	return nil, fmt.Errorf("not-implemented")
}

// filterLoadBalancers returns a filtered list of instances based on the filter provided
func filterLoadBalancers(loadBalancers []olb.LoadBalancer, filter string) []olb.LoadBalancer {
	finalLoadBalancers := loadBalancers[:0]
	for _, loadBalancer := range loadBalancers {
		conditional := true
		if filter != "" {
			conditional = conditional && glob.Glob(filter, loadBalancer.DisplayName)
		}
		if conditional {
			finalLoadBalancers = append(finalLoadBalancers, loadBalancer)
		}
	}
	return finalLoadBalancers
}

// Backends returns the list of server backends.  The private IP address is used as the instance.ID.
func (d *lbDriver) Backends() ([]instance.ID, error) {
	ids := []instance.ID{}

	coreClient := core.NewClient(d.client.Client, d.componentID)
	options := &core.InstancesParameters{
		Filter: &core.InstanceFilter{
			DisplayName:    fmt.Sprintf("%s-*", d.stack),
			LifeCycleState: "running",
		},
	}
	instances, bmcErr := coreClient.ListInstances(options)
	if bmcErr != nil {
		log.Error("error", "err", bmcErr)
		return nil, bmcErr
	}
	for _, node := range instances {
		vNics := coreClient.ListVNic(node.ID)
		instanceID := instance.ID(vNics[0].PrivateIP)
		ids = append(ids, instanceID)
	}
	return ids, nil

}

func (d *lbDriver) backendServers(port int) ([]olb.Backend, error) {
	// Create instances client
	coreClient := core.NewClient(d.client.Client, d.componentID)

	options := &core.InstancesParameters{
		Filter: &core.InstanceFilter{
			DisplayName:    fmt.Sprintf("%s-*", d.stack),
			LifeCycleState: "running",
		},
	}
	instances, bmcErr := coreClient.ListInstances(options)
	if bmcErr != nil {
		log.Error("error", "err", bmcErr)
		return nil, bmcErr
	}
	backends := []olb.Backend{}
	for _, instance := range instances {
		vNics := coreClient.ListVNic(instance.ID)
		backends = append(backends, olb.Backend{
			IPAddress: vNics[0].PrivateIP,
			Port:      port,
		})
	}
	return backends, nil
}
