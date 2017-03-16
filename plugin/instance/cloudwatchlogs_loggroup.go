package instance

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsLogGroupPlugin struct {
	client        cloudwatchlogsiface.CloudWatchLogsAPI
	namespaceTags map[string]string
}

// NewLogGroupPlugin returns a plugin.
func NewLogGroupPlugin(client cloudwatchlogsiface.CloudWatchLogsAPI, namespaceTags map[string]string) instance.Plugin {
	return &awsLogGroupPlugin{client: client, namespaceTags: namespaceTags}
}

type createLogGroupRequest struct {
	CreateLogGroupInput     cloudwatchlogs.CreateLogGroupInput
	PutRetentionPolicyInput *cloudwatchlogs.PutRetentionPolicyInput
}

func (p awsLogGroupPlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsLogGroupPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createLogGroupRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	name := newQueueName(spec.Tags, p.namespaceTags)

	request.CreateLogGroupInput.LogGroupName = aws.String(name)
	if _, err := p.client.CreateLogGroup(&request.CreateLogGroupInput); err != nil {
		return nil, fmt.Errorf("CreateLogGroup failed: %s", err)
	}
	id := instance.ID(name)

	if request.PutRetentionPolicyInput != nil {
		request.PutRetentionPolicyInput.LogGroupName = request.CreateLogGroupInput.LogGroupName
		if _, err := p.client.PutRetentionPolicy(request.PutRetentionPolicyInput); err != nil {
			return &id, fmt.Errorf("PutRetentionPolicy failed: %s", err)
		}
	}

	return &id, nil
}

func (p awsLogGroupPlugin) Label(id instance.ID, labels map[string]string) error {
	return nil
}

func (p awsLogGroupPlugin) Destroy(id instance.ID) error {
	if _, err := p.client.DeleteLogGroup(&cloudwatchlogs.DeleteLogGroupInput{LogGroupName: (*string)(&id)}); err != nil {
		return fmt.Errorf("DeleteLogGroup failed: %s", err)
	}
	return nil
}

func (p awsLogGroupPlugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	name := newQueueName(tags, p.namespaceTags)

	output, err := p.client.DescribeLogGroups(&cloudwatchlogs.DescribeLogGroupsInput{LogGroupNamePrefix: &name})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("ListLogGroups failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, logGroup := range output.LogGroups {
		descriptions = append(descriptions, instance.Description{ID: instance.ID(*logGroup.LogGroupName)})
	}
	return descriptions, nil
}
