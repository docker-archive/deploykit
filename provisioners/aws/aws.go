package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	api "github.com/docker/libmachete"
	"time"
)

type provisioner struct {
	client ec2iface.EC2API
}

func init() {
	impl := &provisioner{}
	api.Register("aws", impl)
}

// NewProvisioner returns a provisioner implementation given the EC2 API client.
func NewProvisioner(client ec2iface.EC2API) api.Provisioner {
	return &provisioner{client: client}
}

// CreateClient creates the actual EC2 API client.
func CreateClient(region, accessKey, secretKey, sessionToken string, retryCount int) ec2iface.EC2API {
	return ec2.New(session.New(aws.NewConfig().
		WithRegion(region).
		WithCredentials(credentials.NewStaticCredentials(accessKey, secretKey, sessionToken)).
		WithLogger(getLogger()).
		WithLogLevel(aws.LogDebugWithHTTPBody).
		WithMaxRetries(retryCount)))
}

func getInstanceSync(client ec2iface.EC2API, instanceID string) (*ec2.Instance, error) {
	result, err := client.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{&instanceID},
	})
	if err != nil {
		return nil, err
	}
	if len(result.Reservations) != 1 || len(result.Reservations[0].Instances) != 1 {
		return nil, &ErrUnexpectedResponse{}
	}
	return result.Reservations[0].Instances[0], nil
}

func tagSync(client ec2iface.EC2API, request CreateInstanceRequest, instance *ec2.Instance) error {
	tags := []*ec2.Tag{} // TODO - add a default tag for machine name?

	for k, v := range request.Tags {
		key := k
		value := v
		tags = append(tags, &ec2.Tag{
			Key:   &key,
			Value: &value,
		})
	}

	_, err := client.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{instance.InstanceId},
		Tags:      tags,
	})
	return err
}

func createSync(client ec2iface.EC2API, request CreateInstanceRequest) (*ec2.Instance, error) {
	reservation, err := client.RunInstances(&ec2.RunInstancesInput{
		ImageId:  &request.ImageID,
		MinCount: aws.Int64(1),
		MaxCount: aws.Int64(1),
		Placement: &ec2.Placement{
			AvailabilityZone: &request.AvailabilityZone,
		},
		KeyName:      &request.KeyName,
		InstanceType: &request.InstanceType,
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{{
			DeviceIndex:              aws.Int64(0), // eth0
			Groups:                   makePointerSlice(request.SecurityGroupIds),
			SubnetId:                 &request.SubnetID,
			AssociatePublicIpAddress: &request.AssociatePublicIPAddress,
			DeleteOnTermination:      &request.DeleteOnTermination,
		}},
		Monitoring: &ec2.RunInstancesMonitoringEnabled{
			Enabled: &request.Monitoring,
		},
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Name: &request.IamInstanceProfile,
		},
		EbsOptimized: &request.EbsOptimized,
		BlockDeviceMappings: []*ec2.BlockDeviceMapping{
			{
				DeviceName: &request.BlockDeviceName,
				Ebs: &ec2.EbsBlockDevice{
					VolumeSize:          &request.RootSize,
					VolumeType:          &request.VolumeType,
					DeleteOnTermination: &request.DeleteOnTermination,
				},
			},
		},
	})

	if err != nil {
		return nil, err
	}

	if reservation == nil || len(reservation.Instances) != 1 {
		return nil, &ErrUnexpectedResponse{}
	}
	return reservation.Instances[0], nil
}

func (p *provisioner) Create(req interface{}) (<-chan api.CreateEvent, error) {
	request, is := req.(CreateInstanceRequest)
	if !is {
		return nil, &ErrInvalidRequest{}
	}

	if err := request.Validate(); err != nil {
		return nil, err
	}

	events := make(chan api.CreateEvent)
	go func() {
		defer close(events)

		events <- api.CreateEvent{Type: api.CreateStarted}

		instance, err := createSync(p.client, request)
		if err != nil {
			events <- api.CreateEvent{
				Error: err,
				Type:  api.CreateError,
			}
			return
		}

		WaitUntil(30, 10*time.Second,
			func() (bool, error) {
				inst, err := getInstanceSync(p.client, *instance.InstanceId)
				return inst != nil && *inst.State.Code == int64(16), err
			})

		err = tagSync(p.client, request, instance)
		if err != nil {
			events <- api.CreateEvent{
				Error: err,
				Type:  api.CreateError,
			}
			return
		}
		events <- api.CreateEvent{
			Type:       api.CreateCompleted,
			ResourceID: *instance.InstanceId,
		}
		return

	}()

	return events, nil
}
