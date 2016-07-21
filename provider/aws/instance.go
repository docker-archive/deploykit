package aws

import (
	"encoding/json"
	"errors"
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

type createInstanceRequest struct {
	Group             instance.GroupID      `json:"group"`
	Tags              map[string]string     `json:"tags"`
	RunInstancesInput ec2.RunInstancesInput `json:"run_instances_input"`
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

	reservation, err := p.client.RunInstances(&request.RunInstancesInput)
	if err != nil {
		return nil, err
	}

	if reservation == nil || len(reservation.Instances) != 1 {
		return nil, spi.NewError(spi.ErrUnknown, "Unexpected AWS API response")
	}
	ec2Instance := reservation.Instances[0]

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
