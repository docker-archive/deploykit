package aws

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	mock_ec2 "github.com/docker/libmachete/mock/ec2"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/instance"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

const (
	testCluster = spi.ClusterID("test-cluster")
)

var (
	tags = map[string]string{"group": "workers"}
)

func TestInstanceLifecycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mock_ec2.NewMockEC2API(ctrl)
	provisioner := Provisioner{
		Client:  clientMock,
		Cluster: testCluster,
	}

	// Create an instance.

	instanceID := "test-id"

	clientMock.EXPECT().RunInstances(gomock.Any()).
		Return(&ec2.Reservation{Instances: []*ec2.Instance{{InstanceId: &instanceID}}}, nil)

	tagRequest := ec2.CreateTagsInput{
		Resources: []*string{&instanceID},
		Tags: []*ec2.Tag{
			{Key: aws.String(ClusterTag), Value: aws.String(string(testCluster))},
			{Key: aws.String("group"), Value: aws.String("workers")},
			{Key: aws.String("test"), Value: aws.String("aws-create-test")},
		},
	}
	clientMock.EXPECT().CreateTags(&tagRequest).Return(&ec2.CreateTagsOutput{}, nil)

	id, err := provisioner.Provision(inputJSON, nil, tags)

	require.NoError(t, err)
	require.Equal(t, instanceID, string(*id))

	// Destroy the instance.

	clientMock.EXPECT().TerminateInstances(&ec2.TerminateInstancesInput{InstanceIds: []*string{&instanceID}}).
		Return(&ec2.TerminateInstancesOutput{
			TerminatingInstances: []*ec2.InstanceStateChange{{InstanceId: &instanceID}}},
			nil)

	require.NoError(t, provisioner.Destroy(instance.ID(instanceID)))
}

func TestCreateInstanceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock_ec2.NewMockEC2API(ctrl)

	runError := errors.New("request failed")
	clientMock.EXPECT().RunInstances(gomock.Any()).Return(&ec2.Reservation{}, runError)

	provisioner := NewInstancePlugin(clientMock, testCluster)
	id, err := provisioner.Provision("{}", nil, tags)

	require.Error(t, err)
	require.Nil(t, id)
}

func TestDestroyInstanceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock_ec2.NewMockEC2API(ctrl)

	instanceID := "test-id"

	runError := errors.New("request failed")
	clientMock.EXPECT().TerminateInstances(&ec2.TerminateInstancesInput{InstanceIds: []*string{&instanceID}}).
		Return(nil, runError)

	provisioner := NewInstancePlugin(clientMock, testCluster)
	require.Error(t, provisioner.Destroy(instance.ID(instanceID)))
}

func describeInstancesResponse(
	instanceIds [][]string,
	tags map[string]string,
	nextToken *string) *ec2.DescribeInstancesOutput {

	ec2Tags := []*ec2.Tag{}
	for key, value := range tags {
		ec2Tags = append(ec2Tags, &ec2.Tag{Key: aws.String(key), Value: aws.String(value)})
	}

	reservations := []*ec2.Reservation{}
	for _, ids := range instanceIds {
		instances := []*ec2.Instance{}
		for _, id := range ids {
			instances = append(instances, &ec2.Instance{
				InstanceId:       aws.String(id),
				PrivateIpAddress: aws.String("127.0.0.1"),
				Tags:             ec2Tags,
			})
		}
		reservations = append(reservations, &ec2.Reservation{Instances: instances})
	}

	return &ec2.DescribeInstancesOutput{NextToken: nextToken, Reservations: reservations}
}

func TestDescribeInstancesRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var nextToken *string
	request := describeGroupRequest(testCluster, tags, nextToken)

	require.Equal(t, nextToken, request.NextToken)

	requireFilter := func(name, value string) {
		for _, filter := range request.Filters {
			if *filter.Name == name {
				for _, filterValue := range filter.Values {
					if *filterValue == value {
						// Match found
						return
					}
				}
			}
		}
		require.Fail(t, fmt.Sprintf("Did not have filter %s/%s", name, value))
	}
	requireFilter("tag:docker-machete", string(testCluster))
	for key, value := range tags {
		requireFilter(fmt.Sprintf("tag:%s", key), value)
	}

	nextToken = aws.String("page-2")
	request = describeGroupRequest(testCluster, tags, nextToken)
	require.Equal(t, nextToken, request.NextToken)
}

func TestListGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock_ec2.NewMockEC2API(ctrl)

	page2Token := "page2"

	// Split instance IDs across multiple reservations and request pages.
	gomock.InOrder(
		clientMock.EXPECT().DescribeInstances(describeGroupRequest(testCluster, tags, nil)).
			Return(describeInstancesResponse([][]string{
				{"a", "b", "c"},
				{"d", "e"},
			}, tags, &page2Token), nil),
		clientMock.EXPECT().DescribeInstances(describeGroupRequest(testCluster, tags, &page2Token)).
			Return(describeInstancesResponse([][]string{{"f", "g"}}, tags, nil), nil),
	)

	provisioner := NewInstancePlugin(clientMock, testCluster)
	descriptions, err := provisioner.DescribeInstances(tags)

	require.NoError(t, err)
	require.Equal(t, []instance.Description{
		{ID: "a", PrivateIPAddress: "127.0.0.1", Tags: tags},
		{ID: "b", PrivateIPAddress: "127.0.0.1", Tags: tags},
		{ID: "c", PrivateIPAddress: "127.0.0.1", Tags: tags},
		{ID: "d", PrivateIPAddress: "127.0.0.1", Tags: tags},
		{ID: "e", PrivateIPAddress: "127.0.0.1", Tags: tags},
		{ID: "f", PrivateIPAddress: "127.0.0.1", Tags: tags},
		{ID: "g", PrivateIPAddress: "127.0.0.1", Tags: tags},
	}, descriptions)
}

const (
	inputJSON = `
{
    "tags": {"test": "aws-create-test"},
    "run_instances_input": {
        "BlockDeviceMappings": [
          {
            "DeviceName": "/dev/sdb",
            "Ebs": {
                "DeleteOnTermination": true,
                "VolumeSize": 64,
                "VolumeType": "gp2"
            }
          }
        ],
        "EbsOptimized": false,
        "ImageId": "ami-30ee0d50",
        "InstanceType": "t2.micro",
        "Monitoring": {
            "Enabled": true
        },
        "NetworkInterfaces": [
          {
            "AssociatePublicIpAddress": true,
            "DeleteOnTermination": true,
            "DeviceIndex": 0,
            "Groups": [
                "sg-973491f0"
            ],
            "SubnetId": "subnet-2"
          }
        ],
        "Placement": {
            "AvailabilityZone": "us-west-2a"
        },
        "UserData": "A string; which must && be base64 encoded"
    }
}
`
)
