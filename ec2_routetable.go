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

type awsRouteTablePlugin struct {
	client        ec2iface.EC2API
	namespaceTags map[string]string
}

// NewRouteTablePlugin returns a plugin.
func NewRouteTablePlugin(client ec2iface.EC2API, namespaceTags map[string]string) instance.Plugin {
	return &awsRouteTablePlugin{client: client, namespaceTags: namespaceTags}
}

type createRouteTableRequest struct {
	CreateRouteTableInput     ec2.CreateRouteTableInput
	AssociateRouteTableInputs []ec2.AssociateRouteTableInput
	CreateRouteInputs         []ec2.CreateRouteInput
	Tags                      map[string]string
}

func (p awsRouteTablePlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsRouteTablePlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createRouteTableRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	output, err := p.client.CreateRouteTable(&request.CreateRouteTableInput)
	if err != nil {
		return nil, fmt.Errorf("CreateRouteTable failed: %s", err)
	}
	id := instance.ID(*output.RouteTable.RouteTableId)

	for _, input := range request.AssociateRouteTableInputs {
		input.RouteTableId = output.RouteTable.RouteTableId
		if _, err := p.client.AssociateRouteTable(&input); err != nil {
			return &id, fmt.Errorf("AssociateRouteTable failed: %s", err)
		}
	}

	for _, input := range request.CreateRouteInputs {
		input.RouteTableId = output.RouteTable.RouteTableId
		if _, err := p.client.CreateRoute(&input); err != nil {
			return &id, fmt.Errorf("CreateRoute failed: %s", err)
		}
	}

	return &id, ec2CreateTags(p.client, id, request.Tags, spec.Tags, p.namespaceTags)
}

func (p awsRouteTablePlugin) Label(id instance.ID, labels map[string]string) error {
	return ec2CreateTags(p.client, id, labels)
}

func (p awsRouteTablePlugin) Destroy(id instance.ID) error {
	output, err := p.client.DescribeRouteTables(&ec2.DescribeRouteTablesInput{RouteTableIds: []*string{(*string)(&id)}})
	if err != nil {
		return fmt.Errorf("DescribeRouteTables failed: %s", err)
	}

	if len(output.RouteTables) > 0 {
		for _, a := range output.RouteTables[0].Associations {
			_, err := p.client.DisassociateRouteTable(&ec2.DisassociateRouteTableInput{AssociationId: a.RouteTableAssociationId})
			if err != nil {
				return fmt.Errorf("DisassociateRouteTable failed: %s", err)
			}
		}
	}

	if _, err := p.client.DeleteRouteTable(&ec2.DeleteRouteTableInput{RouteTableId: (*string)(&id)}); err != nil {
		return fmt.Errorf("DeleteRouteTable failed: %s", err)
	}
	return nil
}

func (p awsRouteTablePlugin) DescribeInstances(labels map[string]string, properties bool) ([]instance.Description, error) {
	_, tags := mergeTags(labels, p.namespaceTags)

	filters := []*ec2.Filter{}
	for key, value := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(value)},
		})
	}

	output, err := p.client.DescribeRouteTables(&ec2.DescribeRouteTablesInput{Filters: filters})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("DescribeRouteTables failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, routeTable := range output.RouteTables {
		tags := map[string]string{}
		for _, tag := range routeTable.Tags {
			if tag.Key != nil && tag.Value != nil {
				tags[*tag.Key] = *tag.Value
			}
		}
		descriptions = append(descriptions, instance.Description{ID: instance.ID(*routeTable.RouteTableId), Tags: tags})
	}
	return descriptions, nil
}
