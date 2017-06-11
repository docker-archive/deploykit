package instance

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsQueuePlugin struct {
	client        sqsiface.SQSAPI
	namespaceTags map[string]string
}

// NewQueuePlugin returns a plugin.
func NewQueuePlugin(client sqsiface.SQSAPI, namespaceTags map[string]string) instance.Plugin {
	return &awsQueuePlugin{client: client, namespaceTags: namespaceTags}
}

type createQueueRequest struct {
	CreateQueueInput sqs.CreateQueueInput
}

func (p awsQueuePlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsQueuePlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createQueueRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	name := newQueueName(spec.Tags, p.namespaceTags)

	request.CreateQueueInput.QueueName = aws.String(name)
	output, err := p.client.CreateQueue(&request.CreateQueueInput)
	if err != nil {
		return nil, fmt.Errorf("CreateQueue failed: %s", err)
	}

	getQueueAttributesOutput, err := p.client.GetQueueAttributes(&sqs.GetQueueAttributesInput{
		AttributeNames: []*string{aws.String("QueueArn")},
		QueueUrl:       output.QueueUrl,
	})
	if err != nil {
		return nil, fmt.Errorf("GetQueueAttributes failed: %s", err)
	}

	var id instance.ID
	for name, value := range getQueueAttributesOutput.Attributes {
		if name == "QueueArn" {
			id = instance.ID(*value)
			break
		}
	}
	if id == instance.ID("") {
		return nil, fmt.Errorf("QueueArn not found for %s", name)
	}

	return &id, nil
}

func (p awsQueuePlugin) Label(id instance.ID, labels map[string]string) error {
	return nil
}

func (p awsQueuePlugin) Destroy(id instance.ID) error {
	output, err := p.client.GetQueueUrl(&sqs.GetQueueUrlInput{QueueName: aws.String(arnOrNameToName(string(id)))})
	if err != nil {
		return fmt.Errorf("GetQueueUrl failed: %s", err)
	}

	if _, err = p.client.DeleteQueue(&sqs.DeleteQueueInput{QueueUrl: output.QueueUrl}); err != nil {
		return fmt.Errorf("DeleteQueue failed: %s", err)
	}
	return nil
}

func (p awsQueuePlugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	name := newQueueName(tags, p.namespaceTags)

	output, err := p.client.ListQueues(&sqs.ListQueuesInput{QueueNamePrefix: aws.String(name)})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("ListQueues failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, queueURL := range output.QueueUrls {
		getQueueAttributesOutput, err := p.client.GetQueueAttributes(&sqs.GetQueueAttributesInput{
			AttributeNames: []*string{aws.String("QueueArn")},
			QueueUrl:       queueURL,
		})
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "AWS.SimpleQueueService.NonExistentQueue" {
				// A deleted queue may wind up here.
				continue
			}
			return []instance.Description{}, fmt.Errorf("GetQueueAttributes failed: %s", err)
		}

		var id instance.ID
		for name, value := range getQueueAttributesOutput.Attributes {
			if name == "QueueArn" {
				id = instance.ID(*value)
				break
			}
		}
		if id == instance.ID("") {
			return []instance.Description{}, fmt.Errorf("QueueArn not found for %s", queueURL)
		}

		descriptions = append(descriptions, instance.Description{ID: id})
	}

	return descriptions, nil
}
