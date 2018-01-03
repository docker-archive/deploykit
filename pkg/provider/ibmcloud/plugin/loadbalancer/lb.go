package ibmcloud

import (
	"fmt"
	"strings"
	"sync"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/softlayer/softlayer-go/datatypes"
	"github.com/softlayer/softlayer-go/services"
	"github.com/softlayer/softlayer-go/session"
)

// For logging
var (
	logger = logutil.New("module", "ibmcloud/loadbalancer")

	debugV1 = logutil.V(100)
	debugV2 = logutil.V(500)
	debugV3 = logutil.V(1000)
)

const (
	// Number and delay for retries when calling a IBM Cloud LBaaS API
	// and it returns an UPDATE_PENDING response.
	updateRetryCount = 30
	updateRetryTime  = 100 * time.Millisecond

	// Hardcoded IBM Cloud endpoint
	softLayerEndpointURL = "https://api.softlayer.com/rest/v3.1"
)

// Result string
type lbResult string

// LBOptions are the configuration parameters for the LB provisioner.
type LBOptions struct {
	uuid string
}

// ibmcloudlb captures the options for starting up the plugin.
type ibmcloudlb struct {
	// name
	name string

	// UUID of the IBM Cloud load balancer backing this
	uuid string

	// mutex to prevent overlaps of calls. May not be needed.
	lock sync.Mutex

	// Softlayer user name
	softlayerUsername string

	// Softlayer API Key
	softlayerAPIKey string

	// Session
	session *session.Session

	// Loadbalancer service
	lbService services.Network_LBaaS_LoadBalancer

	// Loadbalancer Listener service
	lbListenerService services.Network_LBaaS_Listener

	// Loadbalancer Member service
	lbMemberService services.Network_LBaaS_Member

	// Loadbalancer health Monitor  service
	lbHealthMonitorService services.Network_LBaaS_HealthMonitor

	// SSL Certificate service
	certService services.Security_Certificate
}

// NewIBMCloudLBPlugin returns a L4 loadbalancer backed by the IBM cloud
func NewIBMCloudLBPlugin(username, apikey, lbName, lbUUID string) (loadbalancer.L4, error) {
	logger.Info("NewIBMCloudLBPlugin", "Name", lbName)
	lb := &ibmcloudlb{
		softlayerUsername: username,
		softlayerAPIKey:   apikey,
		name:              lbName,
		uuid:              lbUUID,
	}

	// Create the session object and get the services object once
	lb.session = session.New(lb.softlayerUsername, lb.softlayerAPIKey, softLayerEndpointURL).SetRetries(3)
	lb.lbService = services.GetNetworkLBaaSLoadBalancerService(lb.session)
	lb.certService = services.GetSecurityCertificateService(lb.session)
	lb.lbListenerService = services.GetNetworkLBaaSListenerService(lb.session)
	lb.lbMemberService = services.GetNetworkLBaaSMemberService(lb.session)
	lb.lbHealthMonitorService = services.GetNetworkLBaaSHealthMonitorService(lb.session)

	return lb, nil
}

func (r lbResult) String() string {
	return string(r)
}

// stringPtr returns a pointer to the string value passed in.
func stringPtr(v string) *string {
	return &v
}

// intPtr returns a pointer to the int value passed in.
func intPtr(v int) *int {
	return &v
}

// max returns the max int of two passed in ints.
func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// Name is the name of the load balancer
func (l *ibmcloudlb) Name() string {
	logger.Debug("Name", "V", debugV1)
	return l.name
}

// Routes lists all known routes.
func (l *ibmcloudlb) Routes() ([]loadbalancer.Route, error) {
	logger.Debug("Routes", "V", debugV3)
	l.lock.Lock()
	defer l.lock.Unlock()

	// Output array
	out := []loadbalancer.Route{}

	// Get the listeners for the loadbalancer
	lb, err := l.lbService.Mask("listeners;listeners.defaultPool;listeners.defaultPool.healthMonitor").GetLoadBalancer(&l.uuid)
	if err != nil {
		return out, err
	}

	// Iterate over the listeners filling the output array
	for _, listener := range lb.Listeners {
		var cert datatypes.Security_Certificate
		if listener.TlsCertificateId != nil {
			cert, err = l.certService.Id(*listener.TlsCertificateId).GetObject()
			if err != nil {
				return out, err
			}
		}
		r := loadbalancer.Route{
			LoadBalancerPort:     *listener.ProtocolPort,
			LoadBalancerProtocol: loadbalancer.ProtocolFromString(*listener.Protocol),
			Port:                 *listener.DefaultPool.ProtocolPort,
			Protocol:             loadbalancer.ProtocolFromString(*listener.DefaultPool.Protocol),
			Certificate:          cert.CommonName,
			HealthMonitorPath:    listener.DefaultPool.HealthMonitor.UrlPath,
		}
		out = append(out, r)
	}

	logger.Debug("Routes", "name", l.name, "routes", out, "V", debugV3)
	return out, nil
}

// Publish publishes a route in the LB by adding a load balancing rule
func (l *ibmcloudlb) Publish(route loadbalancer.Route) (loadbalancer.Result, error) {
	logger.Info("Publish", "name", l.name, "route", route)
	l.lock.Lock()
	defer l.lock.Unlock()

	// Get the listeners for the loadbalancer
	lb, err := l.lbService.Mask("listeners").GetLoadBalancer(&l.uuid)
	if err != nil {
		return nil, err
	}

	// Iterate over the listeners to see is the specified port is already in use.
	// Log Info message if there is.
	for _, listener := range lb.Listeners {
		if *listener.ProtocolPort == route.LoadBalancerPort && *listener.ProvisioningStatus != "DELETE_PENDING" {
			logger.Error("Publish", "name", l.name, "route", route, "Duplicate Port", route.LoadBalancerPort)
			return lbResult("publish"), nil
		}
	}

	// Get certificate id if certificate name is provided
	var certID int
	if route.Certificate != nil && *route.Certificate != "" {
		// The certificate string is the common name of the certificate.
		cert, err := l.certService.FindByCommonName(route.Certificate)
		if err != nil {
			return nil, err
		}
		if len(cert) == 1 {
			certID = *cert[0].Id
		} else if len(cert) == 0 {
			return nil, fmt.Errorf("No certificate found with common name '%s'", *route.Certificate)
		} else {
			return nil, fmt.Errorf("Cannot identify an unique certificate with common name '%s' (%d certificates found)", *route.Certificate, len(cert))
		}
	}

	// InfraKit doesn't define a method, so default to round robin
	lbMethod := "ROUNDROBIN"

	// Build the array for the softlayer call with the new route
	lbRoute := datatypes.Network_LBaaS_LoadBalancerProtocolConfiguration{
		BackendPort:         &route.Port,
		FrontendPort:        &route.LoadBalancerPort,
		FrontendProtocol:    stringPtr(strings.ToUpper(string(route.LoadBalancerProtocol))),
		BackendProtocol:     stringPtr(strings.ToUpper(string(route.Protocol))),
		LoadBalancingMethod: &lbMethod,
	}
	if certID != 0 {
		lbRoute.TlsCertificateId = &certID
	}
	lbRouteArray := []datatypes.Network_LBaaS_LoadBalancerProtocolConfiguration{lbRoute}

	// Call to add route to loadbalancer
	attempt := 0
	for {
		attempt++
		lb, err = l.lbListenerService.
			Mask("listeners.defaultPool.healthMonitor").
			UpdateLoadBalancerProtocols(&l.uuid, lbRouteArray)
		if err == nil {
			break
		}
		// Check for the lb is in state UPDATE_PENDING. (HTTP 500)
		if strings.Contains(err.Error(), "UPDATE_PENDING") && attempt <= updateRetryCount {
			logger.Debug("Publish", "name", l.name, "state", "Update pending", "V", debugV1)
			time.Sleep(updateRetryTime)
		} else {
			return nil, err
		}
	}

	// The backend protocol needs to be HTTP and the monitor url path set before we make the call to update
	if strings.ToUpper(string(route.Protocol)) == "HTTP" && route.HealthMonitorPath != nil && *route.HealthMonitorPath != "" {
		// The route has been successfully added.  Now update the health monitor path.
		var hmArray []datatypes.Network_LBaaS_LoadBalancerHealthMonitorConfiguration
		for _, lbListener := range lb.Listeners {
			if lbListener.DefaultPool == nil ||
				lbListener.DefaultPool.HealthMonitor == nil ||
				lbListener.DefaultPool.ProtocolPort == nil {
				continue
			}
			// Find the pool that corresponds to the specified backend port
			if *lbListener.DefaultPool.ProtocolPort == route.Port {
				hmConfig := datatypes.Network_LBaaS_LoadBalancerHealthMonitorConfiguration{
					BackendPort:       &route.Port,
					HealthMonitorUuid: lbListener.DefaultPool.HealthMonitor.Uuid,
					BackendProtocol:   lbListener.DefaultPool.HealthMonitor.MonitorType,
					MaxRetries:        lbListener.DefaultPool.HealthMonitor.MaxRetries,
					Interval:          lbListener.DefaultPool.HealthMonitor.Interval,
					Timeout:           lbListener.DefaultPool.HealthMonitor.Timeout,
					UrlPath:           route.HealthMonitorPath,
				}
				hmArray = append(hmArray, hmConfig)
				break
			}
		}

		// Make the call to update the health monitor
		attempt = 0
		for {
			attempt++
			_, err = l.lbHealthMonitorService.UpdateLoadBalancerHealthMonitors(&l.uuid, hmArray)
			if err == nil {
				break
			}
			// Check for the lb is in state UPDATE_PENDING. (HTTP 500)
			if strings.Contains(err.Error(), "UPDATE_PENDING") && attempt <= updateRetryCount {
				logger.Debug("Publish", "name", l.name, "state", "Update pending", "V", debugV1)
				time.Sleep(updateRetryTime)
			} else {
				return nil, err
			}
		}
	}

	return lbResult("publish"), nil
}

// Unpublish dissociates the load balancer from the backend service at the given port.
func (l *ibmcloudlb) Unpublish(extPort int) (loadbalancer.Result, error) {
	logger.Info("Unpublish", "name", l.name, "extPort", extPort)
	l.lock.Lock()
	defer l.lock.Unlock()

	// Get the listeners for the loadbalancer
	lb, err := l.lbService.Mask("listeners").GetLoadBalancer(&l.uuid)
	if err != nil {
		return nil, err
	}

	// Iterate over the listeners.
	listenerFound := false
	for _, listener := range lb.Listeners {
		// If listener is found, remove it and continue.
		if *listener.ProtocolPort == extPort {
			listenerFound = true
			listenerToRemove := []string{*listener.Uuid}
			attempt := 0
			for {
				attempt++
				_, err = l.lbListenerService.DeleteLoadBalancerProtocols(&l.uuid, listenerToRemove)
				if err == nil {
					break
				}
				// Check for the lb is in state UPDATE_PENDING. (HTTP 500)
				if strings.Contains(err.Error(), "UPDATE_PENDING") && attempt <= updateRetryCount {
					logger.Debug("Unpublish", "name", l.name, "state", "Update pending", "V", debugV1)
					time.Sleep(updateRetryTime)
				} else {
					return nil, err
				}
			}
		}
	}

	// Listener was not found,so return an error
	if listenerFound == false {
		logger.Error("unpublish", "name", l.name, "listener not found", extPort)
		return lbResult(""), fmt.Errorf("unknown port %v", extPort)
	}

	return lbResult("unpublish"), nil
}

// ConfigureHealthCheck configures the health checks for instance removal and reconfiguration
// The parameters healthy and unhealthy indicate the number of consecutive success or fail pings required to
// mark a backend instance as healthy or unhealthy. The ping occurs on the backendPort parameter and
// at the interval specified.
func (l *ibmcloudlb) ConfigureHealthCheck(hc loadbalancer.HealthCheck) (loadbalancer.Result, error) {
	logger.Debug("ConfigureHealthCheck", "name", l.name, "heathCheck", hc, "V", debugV3)
	l.lock.Lock()
	defer l.lock.Unlock()

	hmConfig := datatypes.Network_LBaaS_LoadBalancerHealthMonitorConfiguration{
		BackendPort: &hc.BackendPort,
		// IBM Clouud only has a single retry config, so make it the max of the two: healthy and unhealthy
		MaxRetries: intPtr(max(hc.Healthy, hc.Unhealthy)),
		Interval:   intPtr(int(hc.Interval / time.Second)),
		Timeout:    intPtr(int(hc.Timeout / time.Second)),
	}
	hmArray := []datatypes.Network_LBaaS_LoadBalancerHealthMonitorConfiguration{hmConfig}

	_, err := l.lbHealthMonitorService.UpdateLoadBalancerHealthMonitors(&l.uuid, hmArray)
	if err != nil {
		return nil, err
	}

	return lbResult("healthcheck"), nil
}

// Note: The Backend API calls normally get instance IDs for the backends. Since the
// IBM Cloud LBaaS APIs do not have a way to map instance IDs to IPs, we use a
// SourceKeySelector in the ingress yaml:
//
// SourceKeySelector: \{\{ $x := .Properties | jsonDecode \}\}\{\{ $x.ipv4_address_private \}\}
//
// to have InfraKit give us the private ip address for the instance, and also use the ip
// address to calculate what instances need to be added or removed.

// RegisterBackend registers instances identified by the IDs to the LB's backend pool.
// The instance.IDs passed into this code are IPv4 addresses used by the IBM Cloud LBaaS
// APIs. This does require a SourceKeySelector in the ingress yaml file so the controller
// uses the private IP address.
func (l *ibmcloudlb) RegisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	logger.Debug("RegisterBackends", "name", l.name, "ids", ids, "V", debugV2)
	l.lock.Lock()
	defer l.lock.Unlock()

	// Set default weight to 50 to match LB default
	weight := 50

	// Output array of members added
	membersToAdd := []datatypes.Network_LBaaS_LoadBalancerServerInstanceInfo{}

	// Get current list of backend members
	lb, err := l.lbService.Mask("members").GetLoadBalancer(&l.uuid)
	if err != nil {
		return nil, err
	}

	// Iterate over the list of members to add checking if they already exist in the loadbalancer.
	// If not, add member.
	for _, id := range ids {
		ip := string(id)
		ipFound := false
		for _, member := range lb.Members {
			if *member.Address == ip {
				ipFound = true
				logger.Error("RegisterBackends", "name", l.name, "backend already exists", ip)
				break
			}
		}
		if ipFound == false {
			member := datatypes.Network_LBaaS_LoadBalancerServerInstanceInfo{
				PrivateIpAddress: &ip,
				Weight:           &weight,
			}
			membersToAdd = append(membersToAdd, member)
		}
	}

	// If there are any members to add, add them.
	if len(membersToAdd) > 0 {
		attempt := 0
		for {
			attempt++
			_, err = l.lbMemberService.AddLoadBalancerMembers(&l.uuid, membersToAdd)
			if err == nil {
				break
			}
			// Check for the lb is in state UPDATE_PENDING. (HTTP 500)
			if strings.Contains(err.Error(), "UPDATE_PENDING") && attempt <= updateRetryCount {
				logger.Debug("RegisterBackends", "name", l.name, "state", "Update pending", "V", debugV1)
				time.Sleep(updateRetryTime)
			} else {
				return nil, err
			}
		}
	}

	logger.Debug("RegisterBackends", "name", l.name, "backends", membersToAdd, "V", debugV2)
	return lbResult("register"), nil
}

// DeregisterBackend removes the specified instances from the backend pool.
// The instance.IDs passed into this code are IPv4 addresses used by the IBM Cloud LBaaS
// APIs. This does require a SourceKeySelector in the ingress yaml file so the controller
// uses the private IP address.
func (l *ibmcloudlb) DeregisterBackends(ids []instance.ID) (loadbalancer.Result, error) {
	logger.Debug("DeregisterBackends", "name", l.name, "ids", ids, "V", debugV2)
	l.lock.Lock()
	defer l.lock.Unlock()

	// Output array of members removed
	membersToRemove := []string{}

	// Get current list of backend members
	lb, err := l.lbService.Mask("members").GetLoadBalancer(&l.uuid)
	if err != nil {
		return nil, err
	}

	// Iterate over the list of members to remove checking if they exist in the loadbalancer.
	// If they do, add member to removal list. If not, just log it and continue.
	for _, id := range ids {
		ipFound := false
		ip := string(id)
		for _, member := range lb.Members {
			if *member.Address == ip {
				membersToRemove = append(membersToRemove, *member.Uuid)
				ipFound = true
				break
			}
		}
		if ipFound == false {
			logger.Error("DeregisterBackends", "name", l.name, "backend not found", ip)
		}
	}
	// If there are any members to remove, remove them.
	if len(membersToRemove) > 0 {
		attempt := 0
		for {
			attempt++
			_, err = l.lbMemberService.DeleteLoadBalancerMembers(&l.uuid, membersToRemove)
			if err == nil {
				break
			}
			// Check for the lb is in state UPDATE_PENDING. (HTTP 500)
			if strings.Contains(err.Error(), "UPDATE_PENDING") && attempt <= updateRetryCount {
				logger.Debug("DeregisterBackends", "name", l.name, "state", "Update pending", "V", debugV1)
				time.Sleep(updateRetryTime)
			} else {
				return nil, err
			}
		}
	}

	logger.Debug("DeregisterBackends", "name", l.name, "backends", membersToRemove, "V", debugV2)
	return lbResult("deregister"), nil
}

// Backends returns a list of backends
// The instance.IDs passed into this code are IPv4 addresses used by the IBM Cloud LBaaS
// APIs. This does require a SourceKeySelector in the ingress yaml file so the controller
// uses the private IP address.
func (l *ibmcloudlb) Backends() ([]instance.ID, error) {
	logger.Debug("Backends", "name", l.name, "V", debugV3)
	l.lock.Lock()
	defer l.lock.Unlock()

	// Output array of members
	out := []instance.ID{}

	// Get current list of backend members
	lb, err := l.lbService.Mask("members").GetLoadBalancer(&l.uuid)
	if err != nil {
		return out, err
	}

	// Iterate over the listeners filling the output array
	for _, member := range lb.Members {
		out = append(out, instance.ID(*member.Address))
	}

	logger.Debug("Backends", "name", l.name, "backends", out, "V", debugV3)
	return out, nil
}
