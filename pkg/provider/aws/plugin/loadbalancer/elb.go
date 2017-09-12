package aws

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elb/elbiface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
)

// ELBOptions are the configuration parameters for the ELB provisioner.
type ELBOptions struct {
	Region  string
	Retries int
}

type elbPlugin struct {
	client elbiface.ELBAPI
	name   string
}

// NewELBPlugin creates an AWS-based ELB provisioner.
func NewELBPlugin(client elbiface.ELBAPI, name string) (loadbalancer.L4, error) {
	return &elbPlugin{
		client: client,
		name:   name,
	}, nil
}

// Credentials allocates a credential object that has the access key and secret id.
func Credentials(cred *Credential) *credentials.Credentials {
	staticCred := new(Credential)
	if cred != nil {
		staticCred = cred
	}

	return credentials.NewChainCredentials([]credentials.Provider{
		&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(session.New())},
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{},
		staticCred,
	})
}

// CreateELBClient creates an AWS ELB API client.
func CreateELBClient(awsCredentials *credentials.Credentials, opt ELBOptions) elbiface.ELBAPI {
	region := opt.Region
	if region == "" {
		region, _ = GetRegion()
	}

	log.Infoln("ELB Client in region", region)

	return elb.New(session.New(aws.NewConfig().
		WithRegion(region).
		WithCredentials(awsCredentials).
		WithLogger(getLogger()).
		WithLogLevel(aws.LogDebugWithHTTPBody).
		WithMaxRetries(opt.Retries)))
}

func (p *elbPlugin) Name() string {
	return p.name
}

// Routes lists all registered routes.
func (p *elbPlugin) Routes() ([]loadbalancer.Route, error) {
	output, err := p.client.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []*string{aws.String(p.name)},
	})
	if err != nil {
		return nil, err
	}

	routes := []loadbalancer.Route{}

	if len(output.LoadBalancerDescriptions) > 0 && output.LoadBalancerDescriptions[0].ListenerDescriptions != nil {
		for _, listener := range output.LoadBalancerDescriptions[0].ListenerDescriptions {
			routes = append(routes, loadbalancer.Route{
				Port:             int(*listener.Listener.InstancePort),
				Protocol:         loadbalancer.ProtocolFromString(*listener.Listener.Protocol),
				LoadBalancerPort: int(*listener.Listener.LoadBalancerPort),
				Certificate:      listener.Listener.SSLCertificateId,
			})
		}
	}

	return routes, nil
}

func (p *elbPlugin) RegisterBackends(ids []instance.ID) (loadbalancer.Result, error) {

	addInstances := []*elb.Instance{}
	for _, instanceID := range ids {
		addInstance := &elb.Instance{}
		strID := string(instanceID)
		addInstance.InstanceId = &strID
		addInstances = append(addInstances, addInstance)
	}

	return p.client.RegisterInstancesWithLoadBalancer(&elb.RegisterInstancesWithLoadBalancerInput{
		Instances:        addInstances,
		LoadBalancerName: aws.String(p.name),
	})
}

func (p *elbPlugin) DeregisterBackends(ids []instance.ID) (loadbalancer.Result, error) {

	rmInstances := []*elb.Instance{}
	for _, instanceID := range ids {
		rmInstance := &elb.Instance{}
		strID := string(instanceID)
		rmInstance.InstanceId = &strID
		rmInstances = append(rmInstances, rmInstance)
	}

	return p.client.DeregisterInstancesFromLoadBalancer(&elb.DeregisterInstancesFromLoadBalancerInput{
		Instances:        rmInstances,
		LoadBalancerName: aws.String(p.name),
	})
}

func (p *elbPlugin) Backends() ([]instance.ID, error) {
	output, err := p.client.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []*string{
			aws.String(p.name),
		},
	})
	if err != nil {
		return []instance.ID{}, err
	}

	instanceIDs := []instance.ID{}

	if len(output.LoadBalancerDescriptions) == 0 {
		return instanceIDs, nil
	}

	for _, loadBalancerDescription := range output.LoadBalancerDescriptions {
		for _, lbInstance := range loadBalancerDescription.Instances {
			instanceIDs = append(instanceIDs, instance.ID(*lbInstance.InstanceId))
		}
	}
	return instanceIDs, nil
}

func (p *elbPlugin) Publish(route loadbalancer.Route) (loadbalancer.Result, error) {

	if route.Protocol == loadbalancer.Invalid {
		return nil, fmt.Errorf("Bad protocol")
	}
	instanceProtocol := aws.String(string(route.Protocol))
	if route.Protocol == loadbalancer.SSL {
		// SSL needs to point to TCP internally
		instanceProtocol = aws.String(string(loadbalancer.TCP))
	} else if route.Protocol == loadbalancer.HTTPS {
		// HTTPS has to point to HTTP internally
		instanceProtocol = aws.String(string(loadbalancer.HTTP))
	}

	listener := &elb.Listener{
		InstancePort:     aws.Int64(int64(route.Port)),
		LoadBalancerPort: aws.Int64(int64(route.LoadBalancerPort)),
		Protocol:         aws.String(string(route.Protocol)),
		InstanceProtocol: instanceProtocol,
		SSLCertificateId: route.Certificate,
	}

	return p.client.CreateLoadBalancerListeners(&elb.CreateLoadBalancerListenersInput{
		Listeners:        []*elb.Listener{listener},
		LoadBalancerName: aws.String(p.name),
	})
}

func (p *elbPlugin) Unpublish(extPort int) (loadbalancer.Result, error) {
	return p.client.DeleteLoadBalancerListeners(&elb.DeleteLoadBalancerListenersInput{
		LoadBalancerPorts: []*int64{aws.Int64(int64(extPort))},
		LoadBalancerName:  aws.String(p.name),
	})
}

func (p *elbPlugin) ConfigureHealthCheck(hc loadbalancer.HealthCheck) (loadbalancer.Result, error) {

	return p.client.ConfigureHealthCheck(&elb.ConfigureHealthCheckInput{
		HealthCheck: &elb.HealthCheck{
			HealthyThreshold:   aws.Int64(int64(hc.Healthy)),
			Interval:           aws.Int64(int64(hc.Interval.Seconds())),
			Target:             aws.String(fmt.Sprintf("TCP:%d", hc.BackendPort)),
			Timeout:            aws.Int64(int64(hc.Timeout.Seconds())),
			UnhealthyThreshold: aws.Int64(int64(hc.Unhealthy)),
		},
		LoadBalancerName: aws.String(p.name),
	})
}
