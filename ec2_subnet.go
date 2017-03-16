package instance

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsSubnetPlugin struct {
	client        ec2iface.EC2API
	namespaceTags map[string]string
}

// NewSubnetPlugin returns a plugin.
func NewSubnetPlugin(client ec2iface.EC2API, namespaceTags map[string]string) instance.Plugin {
	return &awsSubnetPlugin{client: client, namespaceTags: namespaceTags}
}

type createSubnetRequest struct {
	Tags              map[string]string
	CreateSubnetInput ec2.CreateSubnetInput
}

func (p awsSubnetPlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsSubnetPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createSubnetRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	output, err := p.client.CreateSubnet(&request.CreateSubnetInput)
	if err != nil {
		return nil, fmt.Errorf("CreateSubnet failed: %s", err)
	}
	id := instance.ID(*output.Subnet.SubnetId)

	return &id, ec2CreateTags(p.client, id, request.Tags, spec.Tags, p.namespaceTags)
}

func (p awsSubnetPlugin) Label(id instance.ID, labels map[string]string) error {
	ec2Tags := []*ec2.Tag{}
	for key, value := range labels {
		ec2Tags = append(ec2Tags, &ec2.Tag{Key: aws.String(key), Value: aws.String(value)})
	}

	if _, err := p.client.CreateTags(&ec2.CreateTagsInput{Resources: []*string{(*string)(&id)}, Tags: ec2Tags}); err != nil {
		return fmt.Errorf("CreateTags failed: %s", err)
	}
	return nil
}

func (p awsSubnetPlugin) Destroy(id instance.ID) error {
	err := retry(30*time.Second, 500*time.Millisecond, func() error {
		_, err := p.client.DeleteSubnet(&ec2.DeleteSubnetInput{SubnetId: (*string)(&id)})
		return err
	})
	if err != nil {
		return fmt.Errorf("DeleteSubnet failed: %s", err)
	}
	return nil
}

func (p awsSubnetPlugin) DescribeInstances(labels map[string]string) ([]instance.Description, error) {
	_, tags := mergeTags(labels, p.namespaceTags)

	filters := []*ec2.Filter{}
	for key, value := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(value)},
		})
	}

	output, err := p.client.DescribeSubnets(&ec2.DescribeSubnetsInput{Filters: filters})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("DescribeSubnets failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, subnet := range output.Subnets {
		tags := map[string]string{}
		for _, tag := range subnet.Tags {
			if tag.Key != nil && tag.Value != nil {
				tags[*tag.Key] = *tag.Value
			}
		}
		descriptions = append(descriptions, instance.Description{ID: instance.ID(*subnet.SubnetId), Tags: tags})
	}
	return descriptions, nil
}
