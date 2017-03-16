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

type awsSecurityGroupPlugin struct {
	client        ec2iface.EC2API
	namespaceTags map[string]string
}

// NewSecurityGroupPlugin returns a plugin.
func NewSecurityGroupPlugin(client ec2iface.EC2API, namespaceTags map[string]string) instance.Plugin {
	return &awsSecurityGroupPlugin{client: client, namespaceTags: namespaceTags}
}

type createSecurityGroupRequest struct {
	CreateSecurityGroupInput           ec2.CreateSecurityGroupInput
	AuthorizeSecurityGroupEgressInput  *ec2.AuthorizeSecurityGroupEgressInput
	AuthorizeSecurityGroupIngressInput *ec2.AuthorizeSecurityGroupIngressInput
	Tags                               map[string]string
}

func (p awsSecurityGroupPlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsSecurityGroupPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createSecurityGroupRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	_, tags := mergeTags(spec.Tags, p.namespaceTags)
	path := newIamPath(tags)

	request.CreateSecurityGroupInput.GroupName = aws.String(path)
	output, err := p.client.CreateSecurityGroup(&request.CreateSecurityGroupInput)
	if err != nil {
		return nil, fmt.Errorf("CreateSecurityGroup failed: %s", err)
	}
	id := instance.ID(*output.GroupId)

	if request.AuthorizeSecurityGroupEgressInput != nil {
		request.AuthorizeSecurityGroupEgressInput.GroupId = output.GroupId
		if _, err := p.client.AuthorizeSecurityGroupEgress(request.AuthorizeSecurityGroupEgressInput); err != nil {
			return nil, fmt.Errorf("AuthorizeSecurityGroupEgress failed: %s", err)
		}
	}

	if request.AuthorizeSecurityGroupIngressInput != nil {
		request.AuthorizeSecurityGroupIngressInput.GroupId = output.GroupId
		if _, err := p.client.AuthorizeSecurityGroupIngress(request.AuthorizeSecurityGroupIngressInput); err != nil {
			return nil, fmt.Errorf("AuthorizeSecurityGroupIngress failed: %s", err)
		}
	}

	return &id, ec2CreateTags(p.client, id, request.Tags, spec.Tags, p.namespaceTags)
}

func (p awsSecurityGroupPlugin) Label(id instance.ID, labels map[string]string) error {
	return ec2CreateTags(p.client, id, labels)
}

func (p awsSecurityGroupPlugin) Destroy(id instance.ID) error {
	err := retry(30*time.Second, 500*time.Millisecond, func() error {
		_, err := p.client.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{GroupId: (*string)(&id)})
		return err
	})
	if err != nil {
		return fmt.Errorf("DeleteSecurityGroup failed: %s", err)
	}
	return nil
}

func (p awsSecurityGroupPlugin) DescribeInstances(labels map[string]string) ([]instance.Description, error) {
	_, tags := mergeTags(labels, p.namespaceTags)

	filters := []*ec2.Filter{}
	for key, value := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(value)},
		})
	}

	output, err := p.client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{Filters: filters})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("DescribeSecurityGroups failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, securityGroup := range output.SecurityGroups {
		tags := map[string]string{}
		for _, tag := range securityGroup.Tags {
			if tag.Key != nil && tag.Value != nil {
				tags[*tag.Key] = *tag.Value
			}
		}
		descriptions = append(descriptions, instance.Description{ID: instance.ID(*securityGroup.GroupId), Tags: tags})
	}
	return descriptions, nil
}
