package aws

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/instance"
	"sort"
)

const (
	// ClusterTag is the AWS tag name used to track instances managed by this machete instance.
	ClusterTag = "machete-cluster"

	// GroupTag is the AWS tag name used to track instances included in a group.
	GroupTag = "machete-group"
)

type provisioner struct {
	client  ec2iface.EC2API
	cluster spi.ClusterID
}

// NewInstanceProvisioner creates a new provisioner.
func NewInstanceProvisioner(client ec2iface.EC2API, cluster spi.ClusterID) instance.Provisioner {
	return &provisioner{client: client, cluster: cluster}
}

func makePointerSlice(stackSlice []string) []*string {
	pointerSlice := []*string{}
	for i := range stackSlice {
		pointerSlice = append(pointerSlice, &stackSlice[i])
	}
	return pointerSlice
}

func createInstance(client ec2iface.EC2API, request createInstanceRequest) (*ec2.Instance, error) {
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

func (p provisioner) tagInstance(request createInstanceRequest, instance *ec2.Instance) error {
	tags := []*ec2.Tag{}

	// Gather the tag keys in sorted order, to provide predictable tag order.  This is
	// particularly useful for tests.
	var keys []string
	for k := range request.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		key := k
		value := request.Tags[key]
		tags = append(tags, &ec2.Tag{Key: &key, Value: &value})
	}

	// Add cluster and group tags
	tags = append(
		tags,
		&ec2.Tag{Key: aws.String(ClusterTag), Value: aws.String(string(p.cluster))},
		&ec2.Tag{Key: aws.String(GroupTag), Value: aws.String(string(request.Group))})

	_, err := p.client.CreateTags(&ec2.CreateTagsInput{Resources: []*string{instance.InstanceId}, Tags: tags})
	return err
}

// Provision creates a new instance.
func (p provisioner) Provision(req string) (*instance.ID, error) {
	request := createInstanceRequest{}
	err := json.Unmarshal([]byte(req), &request)
	if err != nil {
		return nil, spi.NewError(spi.ErrBadInput, fmt.Sprintf("Invalid input formatting: %s", err))
	}

	if request.Group == "" {
		return nil, spi.NewError(spi.ErrBadInput, "'group' field must not be blank")
	}

	ec2Instance, err := createInstance(p.client, request)
	if err != nil {
		return nil, err
	}

	id := (*instance.ID)(ec2Instance.InstanceId)

	err = p.tagInstance(request, ec2Instance)
	if err != nil {
		return id, err
	}

	return id, nil
}

// Destroy terminates an existing instance.
func (p provisioner) Destroy(id instance.ID) error {
	result, err := p.client.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: []*string{aws.String(string(id))}})

	if err != nil {
		return err
	}

	if len(result.TerminatingInstances) != 1 {
		// There was no match for the instance ID.
		return spi.NewError(spi.ErrBadInput, "No matching instance")
	}

	return nil
}

func describeGroupRequest(cluster spi.ClusterID, id instance.GroupID, nextToken *string) *ec2.DescribeInstancesInput {
	return &ec2.DescribeInstancesInput{
		NextToken: nextToken,
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", ClusterTag)),
				Values: []*string{aws.String(string(cluster))},
			},
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", GroupTag)),
				Values: []*string{aws.String(string(id))},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("pending"),
					aws.String("running"),
				},
			},
		},
	}
}

func (p provisioner) describeInstances(group instance.GroupID, nextToken *string) ([]instance.ID, error) {
	result, err := p.client.DescribeInstances(describeGroupRequest(p.cluster, group, nextToken))
	if err != nil {
		return nil, err
	}

	ids := []instance.ID{}
	for _, reservation := range result.Reservations {
		for _, ec2Instance := range reservation.Instances {
			ids = append(ids, instance.ID(*ec2Instance.InstanceId))
		}
	}

	if result.NextToken != nil {
		// There are more pages of results.
		remainingPages, err := p.describeInstances(group, result.NextToken)
		if err != nil {
			return nil, err
		}

		ids = append(ids, remainingPages...)
	}

	return ids, nil
}

// ListGroup returns all instances included in a group.
func (p provisioner) ListGroup(group instance.GroupID) ([]instance.ID, error) {
	return p.describeInstances(group, nil)
}
