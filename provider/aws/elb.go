package aws

import (
	"encoding/json"
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

// Options are the configuration parameters for the ELB provisioner.
type Options struct {
	Region  string
	Retries int
}

type elbDriver struct {
	client elbiface.ELBAPI
	name   string
}

// NewELBDriver creates an AWS-based ELB provisioner.
func NewELBDriver(client elbiface.ELBAPI, name string) (loadbalancer.Driver, error) {
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
func CreateELBClient(awsCredentials *credentials.Credentials, opt Options) elbiface.ELBAPI {
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

func (p *elbDriver) State() (loadbalancer.State, error) {
	v, err := p.client.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []*string{aws.String(p.name)},
	})
	if v == nil {
		v = &elb.DescribeLoadBalancersOutput{}
	}
	return describeResult(*v), err
}

// describeResult contains details about an existing ELB.
type describeResult elb.DescribeLoadBalancersOutput

// GetName returns the name of the load balancer
func (d describeResult) GetName() string {
	r := elb.DescribeLoadBalancersOutput(d)
	if len(r.LoadBalancerDescriptions) == 0 {
		return ""
	}

	if len(r.LoadBalancerDescriptions[0].ListenerDescriptions) == 0 {
		return ""
	}

	name := r.LoadBalancerDescriptions[0].LoadBalancerName

	if name != nil {
		return *name
	}
	return ""
}

// String returns a string representation of the struct (JSON)
func (d describeResult) String() string {
	buff, err := json.MarshalIndent(d, "   ", "   ")
	if err != nil {
		return fmt.Sprintf("%v", elb.DescribeLoadBalancersOutput(d))
	}
	return string(buff)
}

// HasListener returns true and the current backend port if there's a listener.
func (d describeResult) HasListener(extPort uint32, protocol loadbalancer.Protocol) (uint32, bool) {
	r := elb.DescribeLoadBalancersOutput(d)
	if len(r.LoadBalancerDescriptions) == 0 {
		return 0, false
	}

	if len(r.LoadBalancerDescriptions[0].ListenerDescriptions) == 0 {
		return 0, false
	}

	for _, ld := range r.LoadBalancerDescriptions[0].ListenerDescriptions {
		if ld.Listener == nil {
			return 0, false
		}
		if (ld.Listener.LoadBalancerPort != nil && uint32(*ld.Listener.LoadBalancerPort) == extPort) &&
			(ld.Listener.Protocol != nil && *ld.Listener.Protocol == string(protocol)) {
			return uint32(*ld.Listener.InstancePort), true
		}
	}
	return 0, false
}

// VisitListeners visits the list of listeners that are in the describe output
func (d describeResult) VisitListeners(v func(lbPort, instancePort uint32, protocol loadbalancer.Protocol)) {
	r := elb.DescribeLoadBalancersOutput(d)
	if len(r.LoadBalancerDescriptions) == 0 {
		return
	}

	if len(r.LoadBalancerDescriptions[0].ListenerDescriptions) == 0 {
		return
	}

	for _, ld := range r.LoadBalancerDescriptions[0].ListenerDescriptions {
		if ld.Listener == nil {
			continue
		}
		if ld.Listener.LoadBalancerPort != nil && ld.Listener.InstancePort != nil && ld.Listener.Protocol != nil {
			v(uint32(*ld.Listener.LoadBalancerPort),
				uint32(*ld.Listener.InstancePort),
				loadbalancer.ProtocolFromString(*ld.Listener.Protocol))
		}
	}
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

func (p *elbDriver) PublishService(ext loadbalancer.Protocol, extPort uint32,
	backend loadbalancer.Protocol, backendPort uint32) (loadbalancer.Result, error) {

	if ext == loadbalancer.Invalid || backend == loadbalancer.Invalid {
		return nil, fmt.Errorf("Bad protocol")
	}

	listener := &elb.Listener{
		InstancePort:     aws.Int64(int64(backendPort)),
		LoadBalancerPort: aws.Int64(int64(extPort)),
		Protocol:         aws.String(string(ext)),
		InstanceProtocol: aws.String(string(backend)),
	}

	// TODO(chungers) - Support SSL id

	return p.client.CreateLoadBalancerListeners(&elb.CreateLoadBalancerListenersInput{
		Listeners:        []*elb.Listener{listener},
		LoadBalancerName: aws.String(p.name),
	})
}

func (p *elbDriver) UnpublishService(extPort uint32) (loadbalancer.Result, error) {
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
