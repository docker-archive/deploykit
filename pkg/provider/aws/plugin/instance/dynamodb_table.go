package instance

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsTablePlugin struct {
	client        dynamodbiface.DynamoDBAPI
	namespaceTags map[string]string
}

// NewTablePlugin returns a plugin.
func NewTablePlugin(client dynamodbiface.DynamoDBAPI, namespaceTags map[string]string) instance.Plugin {
	return &awsTablePlugin{client: client, namespaceTags: namespaceTags}
}

type createTableRequest struct {
	CreateTableInput dynamodb.CreateTableInput
}

func (p awsTablePlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsTablePlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createTableRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	name := newTableName(spec.Tags, p.namespaceTags)

	request.CreateTableInput.TableName = aws.String(name)
	_, err := p.client.CreateTable(&request.CreateTableInput)
	if err != nil {
		return nil, fmt.Errorf("CreateTable failed: %s", err)
	}
	id := instance.ID(name)

	return &id, nil
}

func (p awsTablePlugin) Label(id instance.ID, labels map[string]string) error {
	return nil
}

func (p awsTablePlugin) Destroy(id instance.ID) error {
	if _, err := p.client.DeleteTable(&dynamodb.DeleteTableInput{TableName: (*string)(&id)}); err != nil {
		return fmt.Errorf("DeleteTable failed: %s", err)
	}
	return nil
}

func (p awsTablePlugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	name := newTableName(tags, p.namespaceTags)

	if _, err := p.client.DescribeTable(&dynamodb.DescribeTableInput{TableName: aws.String(name)}); err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "ResourceNotFoundException" {
			return nil, nil
		}
		return nil, fmt.Errorf("ListTables failed: %s", err)
	}

	return []instance.Description{{ID: instance.ID(name), Tags: tags}}, nil
}
