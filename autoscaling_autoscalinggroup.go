package instance

import (
	"encoding/json"
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"
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

	if spec.LogicalID == nil {
		return nil, errors.New("No AutoScalingGroup name found: missing LogicalID")
	}

	request.CreateAutoScalingGroupInput.AutoScalingGroupName = (*string)(spec.LogicalID)

	_, err := p.client.CreateAutoScalingGroup(&request.CreateAutoScalingGroupInput)
	if err != nil {
		return nil, fmt.Errorf("CreateAutoScalingGroup failed: %s", err)
	}
	for i, input := range request.PutLifecycleHookInputs {
		input.LifecycleHookName = aws.String(fmt.Sprintf("%s_hook_%d", newQueueName(spec.Tags, p.namespaceTags), i))
		if _, err := p.client.PutLifecycleHook(&input); err != nil {
			return nil, fmt.Errorf("PutLifecycleHook failed: %s", err)
		}
	}

	// set the tags
	// TODO: user tag support
	keys, allTags := mergeTags(spec.Tags, p.namespaceTags)
	autoscalingTags := []*autoscaling.Tag{}
	for _, key := range keys {
		autoscalingTags = append(
			autoscalingTags,
			&autoscaling.Tag{
				ResourceId:        request.CreateAutoScalingGroupInput.AutoScalingGroupName,
				ResourceType:      aws.String("auto-scaling-group"),
				Key:               aws.String(key),
				Value:             aws.String(allTags[key]),
				PropagateAtLaunch: aws.Bool(true),
			},
		)
	}
	_, err = p.client.CreateOrUpdateTags(&autoscaling.CreateOrUpdateTagsInput{Tags: autoscalingTags})
	if err != nil {
		return nil, err
	}

	// return the id
	id := (*instance.ID)(request.CreateAutoScalingGroupInput.AutoScalingGroupName)
	return id, nil
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

/**
Return true iff searchTags is a subset of tags
*/
func matchAll(searchTags map[string]string, tags map[string]string) bool {
	for k, v := range searchTags {
		if v != tags[k] {
			return false
		}
	}

	return true
}

/*
Example labels map passed to this function is pretty simple, just 1 key:value. eg infrakit.group:my-group-name
*/
func (p awsAutoScalingGroupPlugin) DescribeInstances(labels map[string]string, properties bool) ([]instance.Description, error) {
	descriptions := []instance.Description{}

	output, err := p.client.DescribeAutoScalingGroups(nil)
	for {
		if err != nil {
			return descriptions, fmt.Errorf("DescribeAutoScalingGroups failed: %s", err)
		}
		for _, group := range output.AutoScalingGroups {
			tags := map[string]string{}
			for _, tag := range group.Tags {
				tags[*tag.Key] = *tag.Value
			}
			if matchAll(labels, tags) {
				var status *types.Any
				if properties {
					if v, err := types.AnyValue(group); err == nil {
						status = v
					} else {
						log.Warningln("cannot encode AutoScalingGroup:", err)
					}
				}
				descriptions = append(
					descriptions,
					instance.Description{
						ID:         instance.ID(*group.AutoScalingGroupName),
						LogicalID:  (*instance.LogicalID)(group.AutoScalingGroupName),
						Tags:       tags,
						Properties: status,
					},
				)
			}
		}
		if output.NextToken == nil {
			break
		} else {
			output, err = p.client.DescribeAutoScalingGroups(
				&autoscaling.DescribeAutoScalingGroupsInput{
					AutoScalingGroupNames: []*string{},
					NextToken:             output.NextToken,
				},
			)
		}
	}
	return descriptions, nil
}
