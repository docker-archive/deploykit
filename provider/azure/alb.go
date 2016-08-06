package azure

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/loadbalancer"
	"net/http"
	"time"
)

var (
	// ErrBadData - bad data from backend
	ErrBadData = fmt.Errorf("bad-data")
)

// Options - options for Azure
type Options struct {
	Environment            string
	SubscriptionID         string
	OAuthClientID          string // The app client id
	PollingDelaySeconds    int    // The number of seconds to delay between polls
	PollingDurationSeconds int    // The number of seconds to poll for async status before cancel
	ADClientID             string // AD client app id
	ADClientSecret         string // AD client secret key

	ResourceGroupName string
}

type albDriver struct {
	client        *network.LoadBalancersClient
	name          string
	resourceGroup string
	options       Options
}

// NewLoadBalancerDriver creates a load balancer driver
func NewLoadBalancerDriver(client *network.LoadBalancersClient, opt Options, name string) (loadbalancer.Driver, error) {
	return &albDriver{
		client:        client,
		name:          name,
		resourceGroup: opt.ResourceGroupName,
		options:       opt,
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
		Properties: &network.LoadBalancingRulePropertiesFormat{},
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
	rule.Properties.Protocol = toProtocol(route.Protocol)
	rule.Properties.FrontendPort = &frontendPort
	rule.Properties.BackendPort = &backPort
	rule.Properties.BackendAddressPool = backendPool
	rule.Properties.FrontendIPConfiguration = frontendIPConfig

	// TODO - this is tedious and not very flexible... the function signature does not allow for setting
	// specific properties like 'EnableFloatingIP'.  Should implement this as template.
	rule.Properties.LoadDistribution = network.Default
	rule.Properties.IdleTimeoutInMinutes = &timeoutMinutes

	updated := []network.LoadBalancingRule{}
	for _, r := range index {
		updated = append(updated, *r)
	}
	lb.Properties.LoadBalancingRules = &updated
	return p.asyncUpdate(lb)
}

func (p *albDriver) Unpublish(extPort uint32) (loadbalancer.Result, error) {
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
	lb.Properties.LoadBalancingRules = &filtered
	return p.asyncUpdate(lb)
}

// Azure's autorest library will poll a backend after a REST api call
// https://github.com/Azure/azure-sdk-for-go/tree/master/arm#making-asynchronous-requests
// This makes it problematic because in the server case we may not want to block for a long time
// This function will run the actual call in a goroutine with a timer that will cancel the polling
// after a configurable duration.  The code is set up so that the caller has the option to
// block after calling publish or unpublish, by calling the WaitFor function which will then
// try to read on the channel that will ultimately return the result of the api call.
// In the CLI version of publish/unpublish, for example, in cmd/alb.go, the CLI program has to
// block until the transaction completes otherwise the program will exit right away.  In the
// server case (the controller, in cmd/run.go), we use the asynchronous call so that polling for
// services isn't blocked by the autorest polling.
func (p *albDriver) asyncUpdate(lb *network.LoadBalancer) (loadbalancer.Result, error) {
	copy := *lb // not a deep copy but fields will have same pointer values in case lb is gc'd.
	return runAsyncForDuration(p.options.PollingDurationSeconds,
		func(cancel <-chan struct{}) (loadbalancer.Result, error) {
			return wrap(p.client.CreateOrUpdate(p.resourceGroup, p.name, copy, cancel))
		})
}

// run a task asynchronously for the duration specified. Work is provided as a function which can
// accept a chanel for cancellation.
func runAsyncForDuration(seconds int, work func(cancel <-chan struct{}) (loadbalancer.Result, error)) (loadbalancer.Result, error) {
	// Allocate a chanel and goroutine that will after some polling duration, close
	// the channel to cancel any outstanding request / polling / retries by the autorest library.
	cancel := make(chan struct{})
	go func() {
		timer := time.After(time.Duration(seconds) * time.Second)
		<-timer
		close(cancel)
	}()
	// The done channel allows the long running api call to send either an error or successful
	// response to any client which chooses to block via the function WaitFor.
	done := make(chan interface{})
	go func() {
		result, err := work(cancel)
		if err != nil {
			done <- err
		} else {
			done <- result
		}
		close(done)
	}()
	return &asyncResponse{
		done: done,
	}, nil
}

type asyncResponse struct {
	done <-chan interface{}
}

// String implements the Result interface (stringer)
func (a asyncResponse) String() string {
	// In this case we don't want to block (this may be called from a logging printf...
	return "operation started"
}

// WaitFor allows the caller to actually block for the real result
func WaitFor(result loadbalancer.Result, err error) (loadbalancer.Result, error) {
	r, is := result.(*asyncResponse)
	if !is {
		return result, err // don't block
	}
	out := <-r.done // won't block if closed
	switch out := out.(type) {
	case loadbalancer.Result:
		return out, nil
	case error:
		return nil, out
	}
	return result, err
}

type autorestResp autorest.Response

func (a autorestResp) String() string {
	return fmt.Sprintf("%v", autorest.Response(a))
}

func wrap(a autorest.Response, e error) (loadbalancer.Result, error) {
	return autorestResp(a), e
}

func getFrontendIPConfig(lb *network.LoadBalancer) *network.SubResource {
	if lb.Properties == nil {
		return nil
	}
	if lb.Properties.FrontendIPConfigurations == nil {
		return nil
	}
	for _, b := range *lb.Properties.FrontendIPConfigurations {
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

func (p *albDriver) ConfigureHealthCheck(backendPort uint32, healthy, unhealthy int,
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
	lb.Properties.Probes = &updated
	return p.asyncUpdate(lb)
}

func (p *albDriver) RegisterBackend(id string, more ...string) (loadbalancer.Result, error) {
	// Not implemented because instances are added to LB via ScaleSet
	return nil, fmt.Errorf("not-implemented")
}

func (p *albDriver) DeregisterBackend(id string, more ...string) (loadbalancer.Result, error) {
	// Not implemented because instances are added to LB via ScaleSet
	return nil, fmt.Errorf("not-implemented")
}

func (p *albDriver) currentState() (*network.LoadBalancer, error) {
	lb, err := p.client.Get(p.resourceGroup, p.name, "")
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

	if lbState.Properties != nil && lbState.Properties.LoadBalancingRules != nil {
		for _, rule := range *lbState.Properties.LoadBalancingRules {
			routes = append(routes, loadbalancer.Route{
				Port:             uint32(*rule.Properties.BackendPort),
				Protocol:         loadbalancer.ProtocolFromString(string(rule.Properties.Protocol)),
				LoadBalancerPort: uint32(*rule.Properties.FrontendPort),
			})
		}
	}

	return routes, nil
}

// CreateALBClient creates a client of the SDK
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
	c.PollingDelay = time.Second * time.Duration(opt.PollingDelaySeconds)

	return &c, nil
}
