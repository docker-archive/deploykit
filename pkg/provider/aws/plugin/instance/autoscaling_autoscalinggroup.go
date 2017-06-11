package instance

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsAutoScalingGroupPlugin struct {
	client        autoscalingiface.AutoScalingAPI
	namespaceTags map[string]string
}

// NewAutoScalingGroupPlugin returns a plugin.
func NewAutoScalingGroupPlugin(client autoscalingiface.AutoScalingAPI, namespaceTags map[string]string) instance.Plugin {
	return &awsAutoScalingGroupPlugin{client: client, namespaceTags: namespaceTags}
}

type createAutoScalingGroupRequest struct {
	CreateAutoScalingGroupInput autoscaling.CreateAutoScalingGroupInput
	PutLifecycleHookInputs      []autoscaling.PutLifecycleHookInput
}

func (p awsAutoScalingGroupPlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsAutoScalingGroupPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createAutoScalingGroupRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	name := newUnrestrictedName(spec.Tags, p.namespaceTags)

	request.CreateAutoScalingGroupInput.AutoScalingGroupName = aws.String(name)
	_, err := p.client.CreateAutoScalingGroup(&request.CreateAutoScalingGroupInput)
	if err != nil {
		return nil, fmt.Errorf("CreateAutoScalingGroup failed: %s", err)
	}
	id := instance.ID(name)

	for i, input := range request.PutLifecycleHookInputs {
		input.AutoScalingGroupName = aws.String(name)
		input.LifecycleHookName = aws.String(fmt.Sprintf("%s_hook_%d", newQueueName(spec.Tags, p.namespaceTags), i))
		if _, err := p.client.PutLifecycleHook(&input); err != nil {
			return nil, fmt.Errorf("PutLifecycleHook failed: %s", err)
		}
	}

	return &id, nil
}

func (p awsAutoScalingGroupPlugin) Label(id instance.ID, labels map[string]string) error {
	return nil
}

func (p awsAutoScalingGroupPlugin) Destroy(id instance.ID) error {
	_, err := p.client.DeleteAutoScalingGroup(&autoscaling.DeleteAutoScalingGroupInput{
		AutoScalingGroupName: (*string)(&id),
		ForceDelete:          aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("DeleteAutoScalingGroup failed: %s", err)
	}
	return nil
}

func (p awsAutoScalingGroupPlugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	name := newUnrestrictedName(tags, p.namespaceTags)

	output, err := p.client.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{&name},
	})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("DescribeAutoScalingGroups failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, autoScalingGroup := range output.AutoScalingGroups {
		descriptions = append(descriptions, instance.Description{ID: instance.ID(*autoScalingGroup.AutoScalingGroupName)})
	}
	return descriptions, nil
}
