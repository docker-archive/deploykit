package aws

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/docker/libmachete/machete/spi"
	"github.com/docker/libmachete/machete/spi/instance"
	"github.com/docker/libmachete/provisioners/aws/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:generate mockgen -package mock -destination mock/mock_ec2iface.go github.com/aws/aws-sdk-go/service/ec2/ec2iface EC2API

const testCluster = spi.ClusterID("test-cluster")

func TestCreateInstanceSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock.NewMockEC2API(ctrl)

	instanceID := "test-id"
	reservation := ec2.Reservation{Instances: []*ec2.Instance{{InstanceId: &instanceID}}}

	expectedRunInput := ec2.RunInstancesInput{}
	err := json.Unmarshal([]byte(outputJSON), &expectedRunInput)
	require.NoError(t, err)

	clientMock.EXPECT().RunInstances(&expectedRunInput).Return(&reservation, nil)

	tagRequest := ec2.CreateTagsInput{
		Resources: []*string{&instanceID},
		Tags: []*ec2.Tag{
			{Key: aws.String("test"), Value: aws.String("aws-create-test")},
			{Key: aws.String(ClusterTag), Value: aws.String(string(testCluster))},
			{Key: aws.String(GroupTag), Value: aws.String("group")},
		},
	}
	clientMock.EXPECT().CreateTags(&tagRequest).Return(&ec2.CreateTagsOutput{}, nil)

	provisioner := NewInstanceProvisioner(clientMock, testCluster)
	id, err := provisioner.Provision(inputJSON)

	require.NoError(t, err)
	require.Equal(t, instanceID, string(*id))
}

func TestCreateInstanceRequiresGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock.NewMockEC2API(ctrl)

	provisioner := NewInstanceProvisioner(clientMock, testCluster)
	_, err := provisioner.Provision("{}")
	require.Error(t, err)
	require.Equal(t, spi.ErrBadInput, err.Code)
}

func TestCreateInstanceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock.NewMockEC2API(ctrl)

	runError := errors.New("request failed")
	clientMock.EXPECT().RunInstances(gomock.Any()).Return(&ec2.Reservation{}, runError)

	provisioner := NewInstanceProvisioner(clientMock, testCluster)
	id, err := provisioner.Provision(`{"Group": "a"}`)

	require.Error(t, err)
	require.Nil(t, id)
}

func TestDestroyInstanceSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock.NewMockEC2API(ctrl)

	instanceID := "test-id"

	clientMock.EXPECT().
		TerminateInstances(
			&ec2.TerminateInstancesInput{InstanceIds: []*string{&instanceID}}).
		Return(&ec2.TerminateInstancesOutput{
			TerminatingInstances: []*ec2.InstanceStateChange{{
				InstanceId: &instanceID,
			}}}, nil)

	provisioner := NewInstanceProvisioner(clientMock, testCluster)
	require.NoError(t, provisioner.Destroy(instance.ID(instanceID)))
}

func TestDestroyInstanceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock.NewMockEC2API(ctrl)

	instanceID := "test-id"

	runError := errors.New("request failed")
	clientMock.EXPECT().TerminateInstances(&ec2.TerminateInstancesInput{InstanceIds: []*string{&instanceID}}).
		Return(nil, runError)

	provisioner := NewInstanceProvisioner(clientMock, testCluster)
	require.Error(t, provisioner.Destroy(instance.ID(instanceID)))
}

func describeInstancesResponse(instanceIds [][]string, nextToken *string) *ec2.DescribeInstancesOutput {
	reservations := []*ec2.Reservation{}
	for _, ids := range instanceIds {
		instances := []*ec2.Instance{}
		for _, id := range ids {
			instances = append(instances, &ec2.Instance{InstanceId: aws.String(id)})
		}
		reservations = append(reservations, &ec2.Reservation{Instances: instances})
	}

	return &ec2.DescribeInstancesOutput{NextToken: nextToken, Reservations: reservations}
}

func TestDescribeGroupRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	group := instance.GroupID("manager-nodes")

	var nextToken *string
	request := describeGroupRequest(testCluster, group, nextToken)

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
	requireFilter("tag:machete-cluster", string(testCluster))
	requireFilter("tag:machete-group", string(group))

	nextToken = aws.String("page-2")
	request = describeGroupRequest(testCluster, group, nextToken)
	require.Equal(t, nextToken, request.NextToken)
}

func TestListGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock.NewMockEC2API(ctrl)

	group := instance.GroupID("manager-nodes")
	page2Token := "page2"

	// Split instance IDs across multiple reservations and request pages.
	gomock.InOrder(
		clientMock.EXPECT().DescribeInstances(describeGroupRequest(testCluster, group, nil)).
			Return(describeInstancesResponse([][]string{
				{"a", "b", "c"},
				{"d", "e"},
			}, &page2Token), nil),
		clientMock.EXPECT().DescribeInstances(describeGroupRequest(testCluster, group, &page2Token)).
			Return(describeInstancesResponse([][]string{{"f", "g"}}, nil), nil),
	)

	provisioner := NewInstanceProvisioner(clientMock, testCluster)
	ids, err := provisioner.ListGroup(group)

	require.NoError(t, err)
	require.Equal(t, []instance.ID{
		"a",
		"b",
		"c",
		"d",
		"e",
		"f",
		"g",
	}, ids)
}

const (
	inputJSON = `
{
    "group": "group",
    "availability_zone": "us-west-2a",
    "image_id": "ami-30ee0d50",
    "block_device_name": "/dev/sdb",
    "root_size": 64,
    "volume_type": "gp2",
    "delete_on_termination": true,
    "security_group_ids": [
      "sg-973491f0"
    ],
    "subnet_id": "",
    "instance_type": "t2.micro",
    "private_ip_ddress": "",
    "associate_public_ip_address": true,
    "private_ip_only": false,
    "ebs_optimized": false,
    "iam_instance_profile": "",
    "tags": {
      "test": "aws-create-test"
    },
    "key_name": "dev",
    "vpc_id": "vpc-74c22510",
    "monitoring": true
}
`

	outputJSON = `
{
    "AdditionalInfo": null,
    "BlockDeviceMappings": [
      {
        "DeviceName": "/dev/sdb",
        "Ebs": {
          "DeleteOnTermination": true,
          "Encrypted": null,
          "Iops": null,
          "SnapshotId": null,
          "VolumeSize": 64,
          "VolumeType": "gp2"
        },
        "NoDevice": null,
        "VirtualName": null
      }
    ],
    "ClientToken": null,
    "DisableApiTermination": null,
    "DryRun": null,
    "EbsOptimized": false,
    "IamInstanceProfile": {
      "Arn": null,
      "Name": ""
    },
    "ImageId": "ami-30ee0d50",
    "InstanceInitiatedShutdownBehavior": null,
    "InstanceType": "t2.micro",
    "KernelId": null,
    "KeyName": "dev",
    "MaxCount": 1,
    "MinCount": 1,
    "Monitoring": {
      "Enabled": true
    },
    "NetworkInterfaces": [
      {
        "AssociatePublicIpAddress": true,
        "DeleteOnTermination": true,
        "Description": null,
        "DeviceIndex": 0,
        "Groups": [
          "sg-973491f0"
        ],
        "NetworkInterfaceId": null,
        "PrivateIpAddress": null,
        "PrivateIpAddresses": null,
        "SecondaryPrivateIpAddressCount": null,
        "SubnetId": ""
      }
    ],
    "Placement": {
      "Affinity": null,
      "AvailabilityZone": "us-west-2a",
      "GroupName": null,
      "HostId": null,
      "Tenancy": null
    },
    "PrivateIpAddress": null,
    "RamdiskId": null,
    "SecurityGroupIds": null,
    "SecurityGroups": null,
    "SubnetId": null,
    "UserData": null
  }
`
)
