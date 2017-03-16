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

type awsInternetGatewayPlugin struct {
	client        ec2iface.EC2API
	namespaceTags map[string]string
}

// NewInternetGatewayPlugin returns a plugin.
func NewInternetGatewayPlugin(client ec2iface.EC2API, namespaceTags map[string]string) instance.Plugin {
	return &awsInternetGatewayPlugin{client: client, namespaceTags: namespaceTags}
}

type createInternetGatewayRequest struct {
	CreateInternetGatewayInput ec2.CreateInternetGatewayInput
	AttachInternetGatewayInput *ec2.AttachInternetGatewayInput
	Tags                       map[string]string
}

func (p awsInternetGatewayPlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsInternetGatewayPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createInternetGatewayRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	output, err := p.client.CreateInternetGateway(&request.CreateInternetGatewayInput)
	if err != nil {
		return nil, fmt.Errorf("CreateInternetGateway failed: %s", err)
	}
	id := instance.ID(*output.InternetGateway.InternetGatewayId)

	if request.AttachInternetGatewayInput != nil {
		request.AttachInternetGatewayInput.InternetGatewayId = output.InternetGateway.InternetGatewayId
		if _, err := p.client.AttachInternetGateway(request.AttachInternetGatewayInput); err != nil {
			return &id, fmt.Errorf("AttachInternetGateway failed: %s", err)
		}
	}

	return &id, ec2CreateTags(p.client, id, request.Tags, spec.Tags, p.namespaceTags)
}

func (p awsInternetGatewayPlugin) Label(id instance.ID, labels map[string]string) error {
	return ec2CreateTags(p.client, id, labels)
}

func (p awsInternetGatewayPlugin) Destroy(id instance.ID) error {
	output, err := p.client.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{InternetGatewayIds: []*string{(*string)(&id)}})
	if err != nil {
		return fmt.Errorf("DescribeInternetGateways failed: %s", err)
	}

	if len(output.InternetGateways) > 0 {
		for _, a := range output.InternetGateways[0].Attachments {
			_, err := p.client.DetachInternetGateway(&ec2.DetachInternetGatewayInput{
				InternetGatewayId: (*string)(&id),
				VpcId:             a.VpcId,
			})
			if err != nil {
				return fmt.Errorf("DetachInternetGateway failed: %s", err)
			}
		}
	}

	if _, err := p.client.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{InternetGatewayId: (*string)(&id)}); err != nil {
		return fmt.Errorf("DeleteInternetGateway failed: %s", err)
	}
	return nil
}

func (p awsInternetGatewayPlugin) DescribeInstances(labels map[string]string) ([]instance.Description, error) {
	_, tags := mergeTags(labels, p.namespaceTags)

	filters := []*ec2.Filter{}
	for key, value := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(value)},
		})
	}

	output, err := p.client.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{Filters: filters})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("DescribeInternetGateways failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, internetGateway := range output.InternetGateways {
		tags := map[string]string{}
		for _, tag := range internetGateway.Tags {
			if tag.Key != nil && tag.Value != nil {
				tags[*tag.Key] = *tag.Value
			}
		}
		descriptions = append(descriptions, instance.Description{ID: instance.ID(*internetGateway.InternetGatewayId), Tags: tags})
	}
	return descriptions, nil
}
