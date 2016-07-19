package azure

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/loadbalancer"
	"net/http"
	"time"
)

var (
	ErrBadData = fmt.Errorf("bad-data")
)

type Options struct {
	Environment    string
	SubscriptionID string
	OAuthClientID  string // The app client id
	PollingDelay   int

	ADClientID     string // AD client app id
	ADClientSecret string // AD client secret key

	ResourceGroupName string
}

type albProvisioner struct {
	client        *network.LoadBalancersClient
	name          string
	resourceGroup string
}

func NewALBProvisioner(client *network.LoadBalancersClient, resourceGroup, name string) (loadbalancer.L4Provisioner, error) {
	return &albProvisioner{
		client:        client,
		name:          name,
		resourceGroup: resourceGroup,
	}, nil
}

type autorestResp autorest.Response

func (a autorestResp) String() string {
	return fmt.Sprintf("%v", a)
}

func wrap(a autorest.Response, e error) (loadbalancer.Result, error) {
	return autorestResp(a), e
}

type describeResult network.LoadBalancer

func (p *albProvisioner) Name() string {
	return p.name
}

func (p *albProvisioner) State() (loadbalancer.State, error) {
	lb, err := p.currentState()
	if err != nil {
		return nil, err
	}
	return describeResult(*lb), nil
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

func (p *albProvisioner) PublishService(ext loadbalancer.Protocol, extPort uint32,
	backend loadbalancer.Protocol, backendPort uint32) (loadbalancer.Result, error) {

	lb, err := p.currentState()
	if err != nil {
		return nil, err
	}

	index := getRules(lb)
	if len(index) == 0 {
		return nil, ErrBadData
	}

	backendPool := getBackendPool(lb)
	if backendPool == nil {
		return nil, ErrBadData
	}

	rule := &network.LoadBalancingRule{}
	if _, has := index[extPort]; has {
		rule = index[extPort]
	} else {
		index[extPort] = rule
	}

	name := fmt.Sprintf("%s-%d-%d-%s", p.Name(), extPort, backendPort, string(ext))
	frontendPort := int32(extPort)
	backPort := int32(backendPort)
	timeoutMinutes := int32(5)

	rule.Name = &name
	rule.Properties.Protocol = toProtocol(ext)
	rule.Properties.FrontendPort = &frontendPort
	rule.Properties.BackendPort = &backPort
	rule.Properties.BackendAddressPool = backendPool

	// TODO - this is tedious and not very flexible... the function signature does not allow for setting
	// specific properties like 'EnableFloatingIP'.  Should implement this as template.
	rule.Properties.LoadDistribution = network.Default
	rule.Properties.IdleTimeoutInMinutes = &timeoutMinutes

	updated := []network.LoadBalancingRule{}
	for _, r := range index {
		updated = append(updated, *r)
	}

	cancel := make(chan struct{})
	lb.Properties.LoadBalancingRules = &updated
	return wrap(p.client.CreateOrUpdate(p.resourceGroup, p.name, *lb, cancel))
}

func (p *albProvisioner) UnpublishService(extPort uint32) (loadbalancer.Result, error) {
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

	cancel := make(chan struct{})
	lb.Properties.LoadBalancingRules = &filtered
	return wrap(p.client.CreateOrUpdate(p.resourceGroup, p.name, *lb, cancel))
}

func getBackendPool(lb *network.LoadBalancer) *network.SubResource {
	if lb.Properties == nil {
		return nil
	}
	if lb.Properties.BackendAddressPools == nil {
		return nil
	}
	for _, b := range *lb.Properties.BackendAddressPools {
		if b.Properties == nil {
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

func getRules(lb *network.LoadBalancer) map[uint32]*network.LoadBalancingRule {
	index := map[uint32]*network.LoadBalancingRule{}
	if lb.Properties == nil {
		return index
	}

	if lb.Properties.LoadBalancingRules == nil {
		return index
	}

	for _, r := range *lb.Properties.LoadBalancingRules {
		if r.Properties == nil {
			continue
		}
		if r.Properties.FrontendPort != nil {
			copy := r
			index[uint32(*r.Properties.FrontendPort)] = &copy
		}
	}
	return index
}

func getProbes(lb *network.LoadBalancer) map[uint32]*network.Probe {
	index := map[uint32]*network.Probe{}
	if lb.Properties == nil {
		return index
	}

	if lb.Properties.LoadBalancingRules == nil {
		return index
	}

	for _, p := range *lb.Properties.Probes {
		if p.Properties == nil {
			continue
		}
		if p.Properties.Port != nil {
			copy := p
			index[uint32(*p.Properties.Port)] = &copy
		}
	}
	return index
}

func buildProbe(name string, protocol loadbalancer.Protocol, backendPort uint32, interval time.Duration,
	unhealthy int) *network.Probe {
	port := int32(backendPort)
	intervalSeconds := int32(interval.Seconds())
	numProbes := int32(unhealthy)
	return &network.Probe{
		Name: &name,
		Properties: &network.ProbePropertiesFormat{
			Protocol:          toProbeProtocol(protocol),
			Port:              &port,
			IntervalInSeconds: &intervalSeconds,
			NumberOfProbes:    &numProbes,
		},
	}
}

func (p *albProvisioner) ConfigureHealthCheck(backendPort uint32, healthy, unhealthy int,
	interval, timeout time.Duration) (loadbalancer.Result, error) {
	lb, err := p.currentState()
	if err != nil {
		return nil, err
	}

	port := loadbalancer.TCP
	name := fmt.Sprintf("%s-%d-%s", p.Name(), backendPort, string(port))
	probe := buildProbe(name, port, backendPort, interval, unhealthy)
	index := getProbes(lb)
	if _, has := index[backendPort]; has {
		probe = index[backendPort]
	} else {
		index[backendPort] = probe
	}

	updated := []network.Probe{}
	for _, r := range index {
		updated = append(updated, *r)
	}

	cancel := make(chan struct{})
	lb.Properties.Probes = &updated
	return wrap(p.client.CreateOrUpdate(p.resourceGroup, p.name, *lb, cancel))
}

func (p *albProvisioner) RegisterBackend(id string, more ...string) (loadbalancer.Result, error) {
	// Not implemented because instances are added to LB via ScaleSet
	return nil, fmt.Errorf("not-implemented")
}

func (p *albProvisioner) DeregisterBackend(id string, more ...string) (loadbalancer.Result, error) {
	// Not implemented because instances are added to LB via ScaleSet
	return nil, fmt.Errorf("not-implemented")
}

func (p *albProvisioner) currentState() (*network.LoadBalancer, error) {
	lb, err := p.client.Get(p.resourceGroup, p.name, "")
	if err != nil {
		return nil, err
	}
	return &lb, nil
}

func (d describeResult) String() string {
	buff, err := json.MarshalIndent(d, "   ", "   ")
	if err != nil {
		return fmt.Sprintf("%v", network.LoadBalancer(d))
	}
	return string(buff)
}

func (d describeResult) GetName() string {
	lb := network.LoadBalancer(d)
	name := lb.Name
	if name != nil {
		return *name
	}
	return ""
}

func (d describeResult) HasListener(extPort uint32, protocol loadbalancer.Protocol) (uint32, bool) {
	lb := network.LoadBalancer(d)
	if lb.Properties == nil {
		return 0, false
	}
	if lb.Properties.LoadBalancingRules == nil || len(*lb.Properties.LoadBalancingRules) == 0 {
		return 0, false
	}
	for _, rule := range *lb.Properties.LoadBalancingRules {
		if rule.Properties == nil {
			continue
		}

		var frontendPort, backendPort uint32
		if rule.Properties.FrontendPort != nil {
			frontendPort = uint32(*rule.Properties.FrontendPort)
		}
		if rule.Properties.BackendPort != nil {
			backendPort = uint32(*rule.Properties.BackendPort)
		}
		if loadbalancer.ProtocolFromString(string(rule.Properties.Protocol)) == protocol && frontendPort == extPort {
			return backendPort, true
		}
	}
	return 0, false
}

func (d describeResult) VisitListeners(v func(lbPort, instancePort uint32, protocol loadbalancer.Protocol)) {
	lb := network.LoadBalancer(d)
	if lb.Properties == nil {
		return
	}
	if lb.Properties.LoadBalancingRules == nil || len(*lb.Properties.LoadBalancingRules) == 0 {
		return
	}
	for _, rule := range *lb.Properties.LoadBalancingRules {
		if rule.Properties == nil {
			continue
		}

		var frontendPort, backendPort uint32
		if rule.Properties.FrontendPort != nil {
			frontendPort = uint32(*rule.Properties.FrontendPort)
		}
		if rule.Properties.BackendPort != nil {
			backendPort = uint32(*rule.Properties.BackendPort)
		}
		protocol := loadbalancer.ProtocolFromString(string(rule.Properties.Protocol))
		v(frontendPort, backendPort, protocol)
	}
}

func CreateALBClient(cred *Credential, opt Options) (*network.LoadBalancersClient, error) {
	env, ok := environments[opt.Environment]
	if !ok {
		return nil, fmt.Errorf("No valid environment")
	}

	c := network.NewLoadBalancersClientWithBaseURI(env.ResourceManagerEndpoint, opt.SubscriptionID)
	if cred.authorizer != nil {
		c.Authorizer = cred.authorizer
	} else {
		c.Authorizer = &cred.Token
	}
	c.Client.UserAgent += fmt.Sprintf(";docker4azure/%s", version)

	c.RequestInspector = func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			log.Debug("Azure request", r)
			return p.Prepare(r)
		})
	}
	c.ResponseInspector = func(r autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(resp *http.Response) error {
			log.Debug("Azure response", resp)
			return r.Respond(resp)
		})
	}
	c.PollingDelay = time.Second * time.Duration(opt.PollingDelay)
	return &c, nil
}
