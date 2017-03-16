package instance

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elb/elbiface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsLoadBalancerPlugin struct {
	client        elbiface.ELBAPI
	namespaceTags map[string]string
}

// NewLoadBalancerPlugin returns a plugin.
func NewLoadBalancerPlugin(client elbiface.ELBAPI, namespaceTags map[string]string) instance.Plugin {
	return &awsLoadBalancerPlugin{client: client, namespaceTags: namespaceTags}
}

type createLoadBalancerRequest struct {
	CreateLoadBalancerInput           elb.CreateLoadBalancerInput
	ConfigureHealthCheckInput         *elb.ConfigureHealthCheckInput
	ModifyLoadBalancerAttributesInput *elb.ModifyLoadBalancerAttributesInput
	Tags                              map[string]string
}

func (p awsLoadBalancerPlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsLoadBalancerPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createLoadBalancerRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	name := newLoadBalancerName(spec.Tags, p.namespaceTags)

	request.CreateLoadBalancerInput.LoadBalancerName = aws.String(name)
	if _, err := p.client.CreateLoadBalancer(&request.CreateLoadBalancerInput); err != nil {
		return nil, fmt.Errorf("CreateLoadBalancer failed: %s", err)
	}
	id := instance.ID(name)

	if request.ConfigureHealthCheckInput != nil {
		request.ConfigureHealthCheckInput.LoadBalancerName = request.CreateLoadBalancerInput.LoadBalancerName
		if _, err := p.client.ConfigureHealthCheck(request.ConfigureHealthCheckInput); err != nil {
			return &id, fmt.Errorf("ConfigureHealthCheck failed: %s", err)
		}
	}

	if request.ModifyLoadBalancerAttributesInput != nil {
		request.ModifyLoadBalancerAttributesInput.LoadBalancerName = request.CreateLoadBalancerInput.LoadBalancerName
		if _, err := p.client.ModifyLoadBalancerAttributes(request.ModifyLoadBalancerAttributesInput); err != nil {
			return &id, fmt.Errorf("ModifyLoadBalancerAttributes failed: %s", err)
		}
	}

	_, tags := mergeTags(spec.Tags, p.namespaceTags)
	return &id, p.Label(id, tags)
}

func (p awsLoadBalancerPlugin) Label(id instance.ID, labels map[string]string) error {
	elbTags := []*elb.Tag{}
	for key, value := range labels {
		elbTags = append(elbTags, &elb.Tag{Key: aws.String(key), Value: aws.String(value)})
	}

	if _, err := p.client.AddTags(&elb.AddTagsInput{LoadBalancerNames: []*string{(*string)(&id)}, Tags: elbTags}); err != nil {
		return fmt.Errorf("AddTags failed: %s", err)
	}
	return nil
}

func (p awsLoadBalancerPlugin) Destroy(id instance.ID) error {
	if _, err := p.client.DeleteLoadBalancer(&elb.DeleteLoadBalancerInput{LoadBalancerName: (*string)(&id)}); err != nil {
		return fmt.Errorf("DeleteLoadBalancer failed: %s", err)
	}
	return nil
}

func (p awsLoadBalancerPlugin) DescribeInstances(labels map[string]string) ([]instance.Description, error) {
	_, tags := mergeTags(labels, p.namespaceTags)

	output, err := p.client.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("DescribeLoadBalancers failed: %s", err)
	}
	if len(output.LoadBalancerDescriptions) == 0 {
		return []instance.Description{}, nil
	}

	loadBalancerNames := []*string{}
	for _, loadBalancerDescription := range output.LoadBalancerDescriptions {
		loadBalancerNames = append(loadBalancerNames, loadBalancerDescription.LoadBalancerName)
	}

	describeTagsOutput, err := p.client.DescribeTags(&elb.DescribeTagsInput{LoadBalancerNames: loadBalancerNames})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("DescribeTags failed: %s", err)
	}

Loop:
	for _, tagDescription := range describeTagsOutput.TagDescriptions {
		retrievedTags := map[string]string{}
		for _, elbTag := range tagDescription.Tags {
			retrievedTags[*elbTag.Key] = *elbTag.Value
		}

		for key, value := range tags {
			if retrievedTagValue, ok := retrievedTags[key]; !ok || retrievedTagValue != value {
				continue Loop
			}
		}

		return []instance.Description{{ID: instance.ID(*tagDescription.LoadBalancerName), Tags: tags}}, nil
	}

	return []instance.Description{}, nil
}
