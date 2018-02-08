package loadbalancer

import (
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/provider/azure/plugin"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

var (
	// ErrBadData - bad data from backend
	ErrBadData = fmt.Errorf("bad-data")
)

var (
	log    = logutil.New("module", "azure/instance")
	debugV = logutil.V(500)
)

type albDriver struct {
	*network.LoadBalancersClient
	name    string
	options plugin.Options
}

// NewL4Plugin creates a load balancer driver
func NewL4Plugin(name string, options plugin.Options) (loadbalancer.L4, error) {
	client := network.NewLoadBalancersClient(options.SubscriptionID)
	client.Authorizer = autorest.NewBearerAuthorizer(options)

	return &albDriver{
		LoadBalancersClient: &client,
		name:                name,
		options:             options,
	}, nil
}

func (p *albDriver) Name() string {
	return p.name
}

func toProbeProtocol(p loadbalancer.Protocol) network.ProbeProtocol {
	switch p {
	case loadbalancer.HTTP, loadbalancer.HTTPS:
		return network.ProbeProtocolHTTP
	case loadbalancer.TCP, loadbalancer.SSL:
		return network.ProbeProtocolTCP
	}
	return network.ProbeProtocolTCP
}

func toProtocol(p loadbalancer.Protocol) network.TransportProtocol {
	switch p {
	case loadbalancer.TCP, loadbalancer.HTTP, loadbalancer.HTTPS, loadbalancer.SSL:
		return network.TransportProtocolTCP
	case loadbalancer.UDP:
		return network.TransportProtocolUDP
	}
	return network.TransportProtocolTCP
}

func (p *albDriver) Publish(route loadbalancer.Route) (loadbalancer.Result, error) {

	lb, err := p.currentState()
	if err != nil {
		return nil, err
	}

	index := getRules(lb)

	backendPool := getBackendPool(lb)
	if backendPool == nil {
		return nil, ErrBadData
	}

	frontendIPConfig := getFrontendIPConfig(lb)
	if frontendIPConfig == nil {
		return nil, ErrBadData
	}

	rule := &network.LoadBalancingRule{
		LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{},
	}
	if _, has := index[route.LoadBalancerPort]; has {
		rule = index[route.LoadBalancerPort]
	} else {
		index[route.LoadBalancerPort] = rule
	}

	name := fmt.Sprintf("%s-%d-%d-%s", p.Name(), route.LoadBalancerPort, route.Port, route.Protocol)
	frontendPort := int32(route.LoadBalancerPort)
	backPort := int32(route.Port)
	timeoutMinutes := int32(5)

	rule.Name = &name
	rule.LoadBalancingRulePropertiesFormat.Protocol = toProtocol(route.Protocol)
	rule.LoadBalancingRulePropertiesFormat.FrontendPort = &frontendPort
	rule.LoadBalancingRulePropertiesFormat.BackendPort = &backPort
	rule.LoadBalancingRulePropertiesFormat.BackendAddressPool = backendPool
	rule.LoadBalancingRulePropertiesFormat.FrontendIPConfiguration = frontendIPConfig

	// TODO - this is tedious and not very flexible... the function signature does not allow for setting
	// specific properties like 'EnableFloatingIP'.  Should implement this as template.
	rule.LoadBalancingRulePropertiesFormat.LoadDistribution = network.Default
	rule.LoadBalancingRulePropertiesFormat.IdleTimeoutInMinutes = &timeoutMinutes

	updated := []network.LoadBalancingRule{}
	for _, r := range index {
		updated = append(updated, *r)
	}
	lb.LoadBalancerPropertiesFormat.LoadBalancingRules = &updated

	lbChan, errChan := p.CreateOrUpdate(p.options.ResourceGroup, p.name, *lb, nil)

	return lbResponse(<-lbChan), <-errChan
}

type lbResponse network.LoadBalancer

func (l lbResponse) String() string {
	if l.Name != nil {
		return *l.Name
	}
	return "nil"
}

func (p *albDriver) Unpublish(extPort int) (loadbalancer.Result, error) {
	lb, err := p.currentState()
	if err != nil {
		return nil, err
	}

	index := getRules(lb)
	if _, has := index[extPort]; has {
		delete(index, extPort)
	}

	filtered := []network.LoadBalancingRule{}
	for _, r := range index {
		filtered = append(filtered, *r)
	}
	lb.LoadBalancerPropertiesFormat.LoadBalancingRules = &filtered

	lbChan, errChan := p.CreateOrUpdate(p.options.ResourceGroup, p.name, *lb, nil)

	return lbResponse(<-lbChan), <-errChan
}

func getFrontendIPConfig(lb *network.LoadBalancer) *network.SubResource {
	if lb.LoadBalancerPropertiesFormat == nil {
		return nil
	}
	if lb.LoadBalancerPropertiesFormat.FrontendIPConfigurations == nil {
		return nil
	}
	for _, b := range *lb.LoadBalancerPropertiesFormat.FrontendIPConfigurations {
		if b.FrontendIPConfigurationPropertiesFormat == nil {
			continue
		}
		if b.ID == nil {
			continue
		}
		return &network.SubResource{
			ID: b.ID,
		}
	}
	return nil
}

func getBackendPool(lb *network.LoadBalancer) *network.SubResource {
	if lb.LoadBalancerPropertiesFormat == nil {
		return nil
	}
	if lb.LoadBalancerPropertiesFormat.BackendAddressPools == nil {
		return nil
	}
	for _, b := range *lb.LoadBalancerPropertiesFormat.BackendAddressPools {
		if b.BackendAddressPoolPropertiesFormat == nil {
			continue
		}
		if b.ID == nil {
			continue
		}
		return &network.SubResource{
			ID: b.ID,
		}
	}
	return nil
}

func getRules(lb *network.LoadBalancer) map[int]*network.LoadBalancingRule {
	index := map[int]*network.LoadBalancingRule{}
	if lb.LoadBalancerPropertiesFormat == nil {
		return index
	}

	if lb.LoadBalancerPropertiesFormat.LoadBalancingRules == nil {
		return index
	}

	for _, r := range *lb.LoadBalancerPropertiesFormat.LoadBalancingRules {
		if r.LoadBalancingRulePropertiesFormat == nil {
			continue
		}
		if r.LoadBalancingRulePropertiesFormat.FrontendPort != nil {
			copy := r
			index[int(*r.LoadBalancingRulePropertiesFormat.FrontendPort)] = &copy
		}
	}
	return index
}

func getProbes(lb *network.LoadBalancer) map[int]*network.Probe {
	index := map[int]*network.Probe{}
	if lb.LoadBalancerPropertiesFormat == nil {
		return index
	}

	if lb.LoadBalancerPropertiesFormat.LoadBalancingRules == nil {
		return index
	}

	for _, p := range *lb.LoadBalancerPropertiesFormat.Probes {
		if p.ProbePropertiesFormat == nil {
			continue
		}
		if p.ProbePropertiesFormat.Port != nil {
			copy := p
			index[int(*p.ProbePropertiesFormat.Port)] = &copy
		}
	}
	return index
}

func buildProbe(name string, protocol loadbalancer.Protocol, backendPort int, interval time.Duration,
	unhealthy int) *network.Probe {
	port := int32(backendPort)
	intervalSeconds := int32(interval.Seconds())
	numProbes := int32(unhealthy)
	return &network.Probe{
		Name: &name,
		ProbePropertiesFormat: &network.ProbePropertiesFormat{
			Protocol:          toProbeProtocol(protocol),
			Port:              &port,
			IntervalInSeconds: &intervalSeconds,
			NumberOfProbes:    &numProbes,
		},
	}
}

func (p *albDriver) ConfigureHealthCheck(hc loadbalancer.HealthCheck) (loadbalancer.Result, error) {
	lb, err := p.currentState()
	if err != nil {
		return nil, err
	}

	if string(hc.Protocol) == "" {
		hc.Protocol = loadbalancer.TCP
	}

	name := fmt.Sprintf("%s-%d-%s", p.Name(), hc.BackendPort, string(hc.Protocol))
	probe := buildProbe(name, hc.Protocol, hc.BackendPort, hc.Interval, hc.Unhealthy)
	index := getProbes(lb)
	if _, has := index[hc.BackendPort]; has {
		probe = index[hc.BackendPort]
	} else {
		index[hc.BackendPort] = probe
	}

	updated := []network.Probe{}
	for _, r := range index {
		updated = append(updated, *r)
	}
	lb.LoadBalancerPropertiesFormat.Probes = &updated

	lbChan, errChan := p.CreateOrUpdate(p.options.ResourceGroup, p.name, *lb, nil)

	return lbResponse(<-lbChan), <-errChan
}

func (p *albDriver) Backends() ([]instance.ID, error) {
	return nil, fmt.Errorf("not-implemented")
}

func (p *albDriver) RegisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	// Not implemented because instances are added to LB via ScaleSet
	return nil, fmt.Errorf("not-implemented")
}

func (p *albDriver) DeregisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	// Not implemented because instances are added to LB via ScaleSet
	return nil, fmt.Errorf("not-implemented")
}

func (p *albDriver) currentState() (*network.LoadBalancer, error) {
	lb, err := p.Get(p.options.ResourceGroup, p.name, "")
	if err != nil {
		return nil, err
	}
	return &lb, nil
}

func (p *albDriver) Routes() ([]loadbalancer.Route, error) {
	lbState, err := p.currentState()
	if err != nil {
		return nil, err
	}

	routes := []loadbalancer.Route{}

	if lbState.LoadBalancerPropertiesFormat != nil && lbState.LoadBalancerPropertiesFormat.LoadBalancingRules != nil {
		for _, rule := range *lbState.LoadBalancerPropertiesFormat.LoadBalancingRules {
			routes = append(routes, loadbalancer.Route{
				Port:             int(*rule.BackendPort),
				Protocol:         loadbalancer.ProtocolFromString(string(rule.Protocol)),
				LoadBalancerPort: int(*rule.FrontendPort),
			})
		}
	}

	return routes, nil
}
