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
	"github.com/docker/libmachete/spi/loadbalancer"
	"time"
)

// ELBOptions are the configuration parameters for the ELB provisioner.
type ELBOptions struct {
	Region  string
	Retries int
}

type elbDriver struct {
	client elbiface.ELBAPI
	name   string
}

// NewLoadBalancerDriver creates an AWS-based ELB provisioner.
func NewLoadBalancerDriver(client elbiface.ELBAPI, name string) (loadbalancer.Driver, error) {
	return &elbDriver{
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

func (p *elbDriver) Name() string {
	return p.name
}

// Routes lists all registered routes.
func (p *elbDriver) Routes() ([]loadbalancer.Route, error) {
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
				Port:             uint32(*listener.Listener.InstancePort),
				Protocol:         loadbalancer.ProtocolFromString(*listener.Listener.Protocol),
				LoadBalancerPort: uint32(*listener.Listener.LoadBalancerPort),
			})
		}
	}

	return routes, nil
}

func instances(instanceID string, otherIDs ...string) []*elb.Instance {
	instances := []*elb.Instance{
		{
			InstanceId: aws.String(instanceID),
		},
	}
	for _, id := range otherIDs {
		instances = append(instances, &elb.Instance{InstanceId: aws.String(id)})
	}
	return instances
}

func (p *elbDriver) RegisterBackend(instanceID string, otherIDs ...string) (loadbalancer.Result, error) {
	return p.client.RegisterInstancesWithLoadBalancer(&elb.RegisterInstancesWithLoadBalancerInput{
		Instances:        instances(instanceID, otherIDs...),
		LoadBalancerName: aws.String(p.name),
	})
}

func (p *elbDriver) DeregisterBackend(instanceID string, otherIDs ...string) (loadbalancer.Result, error) {
	return p.client.DeregisterInstancesFromLoadBalancer(&elb.DeregisterInstancesFromLoadBalancerInput{
		Instances:        instances(instanceID, otherIDs...),
		LoadBalancerName: aws.String(p.name),
	})
}

func (p *elbDriver) Publish(route loadbalancer.Route) (loadbalancer.Result, error) {

	if route.Protocol == loadbalancer.Invalid {
		return nil, fmt.Errorf("Bad protocol")
	}

	listener := &elb.Listener{
		InstancePort:     aws.Int64(int64(route.Port)),
		LoadBalancerPort: aws.Int64(int64(route.LoadBalancerPort)),
		Protocol:         aws.String(string(route.Protocol)),
		InstanceProtocol: aws.String(string(route.Protocol)),
	}

	// TODO(chungers) - Support SSL id

	return p.client.CreateLoadBalancerListeners(&elb.CreateLoadBalancerListenersInput{
		Listeners:        []*elb.Listener{listener},
		LoadBalancerName: aws.String(p.name),
	})
}

func (p *elbDriver) Unpublish(extPort uint32) (loadbalancer.Result, error) {
	return p.client.DeleteLoadBalancerListeners(&elb.DeleteLoadBalancerListenersInput{
		LoadBalancerPorts: []*int64{aws.Int64(int64(extPort))},
		LoadBalancerName:  aws.String(p.name),
	})
}

func (p *elbDriver) ConfigureHealthCheck(backendPort uint32, healthy, unhealthy int,
	interval, timeout time.Duration) (loadbalancer.Result, error) {

	return p.client.ConfigureHealthCheck(&elb.ConfigureHealthCheckInput{
		HealthCheck: &elb.HealthCheck{
			HealthyThreshold:   aws.Int64(int64(healthy)),
			Interval:           aws.Int64(int64(interval.Seconds())),
			Target:             aws.String(fmt.Sprintf("TCP:%d", backendPort)),
			Timeout:            aws.Int64(int64(timeout.Seconds())),
			UnhealthyThreshold: aws.Int64(int64(unhealthy)),
		},
		LoadBalancerName: aws.String(p.name),
	})
}
