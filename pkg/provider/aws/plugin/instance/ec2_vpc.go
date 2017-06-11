package instance

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsVpcPlugin struct {
	client        ec2iface.EC2API
	namespaceTags map[string]string
}

// NewVpcPlugin returns a plugin.
func NewVpcPlugin(client ec2iface.EC2API, namespaceTags map[string]string) instance.Plugin {
	return &awsVpcPlugin{client: client, namespaceTags: namespaceTags}
}

type createVpcRequest struct {
	CreateVpcInput           ec2.CreateVpcInput
	ModifyVpcAttributeInputs []ec2.ModifyVpcAttributeInput
	Tags                     map[string]string
}

func (p awsVpcPlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsVpcPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createVpcRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	output, err := p.client.CreateVpc(&request.CreateVpcInput)
	if err != nil {
		return nil, fmt.Errorf("CreateVpc failed: %s", err)
	}
	id := instance.ID(*output.Vpc.VpcId)

	for _, input := range request.ModifyVpcAttributeInputs {
		input.VpcId = output.Vpc.VpcId
		if _, err := p.client.ModifyVpcAttribute(&input); err != nil {
			return &id, fmt.Errorf("ModifyVpcAttribute failed: %s", err)
		}
	}

	return &id, ec2CreateTags(p.client, id, request.Tags, spec.Tags, p.namespaceTags)
}

func (p awsVpcPlugin) Label(id instance.ID, labels map[string]string) error {
	return ec2CreateTags(p.client, id, labels)
}

func (p awsVpcPlugin) Destroy(id instance.ID) error {
	if _, err := p.client.DeleteVpc(&ec2.DeleteVpcInput{VpcId: (*string)(&id)}); err != nil {
		return fmt.Errorf("DeleteVpc failed: %s", err)
	}
	return nil
}

func (p awsVpcPlugin) DescribeInstances(labels map[string]string, properties bool) ([]instance.Description, error) {
	_, tags := mergeTags(labels, p.namespaceTags)

	filters := []*ec2.Filter{}
	for key, value := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(value)},
		})
	}

	output, err := p.client.DescribeVpcs(&ec2.DescribeVpcsInput{Filters: filters})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("DescribeVpcs failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, vpc := range output.Vpcs {
		tags := map[string]string{}
		for _, tag := range vpc.Tags {
			if tag.Key != nil && tag.Value != nil {
				tags[*tag.Key] = *tag.Value
			}
		}
		descriptions = append(descriptions, instance.Description{ID: instance.ID(*vpc.VpcId), Tags: tags})
	}
	return descriptions, nil
}
