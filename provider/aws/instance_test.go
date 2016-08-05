package aws

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	mock_ec2 "github.com/docker/libmachete/mock/ec2"
	mock_ssh_util "github.com/docker/libmachete/mock/spi/util/sshutil"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/instance"
	"github.com/docker/libmachete/spi/util/sshutil"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"testing"
)

const (
	testCluster = spi.ClusterID("test-cluster")
	ipAddress   = "256.256.256.256"
)

func TestInstanceLifecycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientMock := mock_ec2.NewMockEC2API(ctrl)
	runnerMock := mock_ssh_util.NewMockCommandRunner(ctrl)
	provisioner := Provisioner{
		Client:        clientMock,
		Cluster:       testCluster,
		CommandRunner: runnerMock,
		KeyStore:      sshutil.FileSystemKeyStore(afero.NewMemMapFs(), "/"),
	}

	// Create an instance.

	instanceID := "test-id"

	keyName := expectKeyPairCreation(t, clientMock)

	clientMock.EXPECT().RunInstances(gomock.Any()).
		Return(&ec2.Reservation{Instances: []*ec2.Instance{{InstanceId: &instanceID}}}, nil)

	tagRequest := ec2.CreateTagsInput{
		Resources: []*string{&instanceID},
		Tags: []*ec2.Tag{
			{Key: aws.String("test"), Value: aws.String("aws-create-test")},
			{Key: aws.String(ClusterTag), Value: aws.String(string(testCluster))},
			{Key: aws.String(GroupTag), Value: aws.String("group")},
		},
	}
	clientMock.EXPECT().CreateTags(&tagRequest).Return(&ec2.CreateTagsOutput{}, nil)

	id, err := provisioner.Provision(inputJSON)

	require.NoError(t, err)
	require.Equal(t, instanceID, string(*id))

	// Execute a shell command

	command := "echo hello"
	commandOutput := "hello"

	expectDescribeInstance(t, clientMock, instanceID, *keyName).AnyTimes()
	runnerMock.EXPECT().Exec(fmt.Sprintf("%s:22", ipAddress), gomock.Any(), command).Do(
		func(addr string, config *ssh.ClientConfig, command string) {
			require.Equal(t, "ubuntu", config.User)
		}).Return(&commandOutput, nil)

	output, err := provisioner.ShellExec(*id, command)
	require.NoError(t, err)
	require.Equal(t, "hello", *output)

	// Destroy the instance.

	expectKeyPairDeletion(t, clientMock, *keyName)
	clientMock.EXPECT().TerminateInstances(&ec2.TerminateInstancesInput{InstanceIds: []*string{&instanceID}}).
		Return(&ec2.TerminateInstancesOutput{
			TerminatingInstances: []*ec2.InstanceStateChange{{InstanceId: &instanceID}}},
			nil)

	require.NoError(t, provisioner.Destroy(instance.ID(instanceID)))
}

func TestCreateInstanceRequiresGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock_ec2.NewMockEC2API(ctrl)

	provisioner := NewInstanceProvisioner(clientMock, testCluster)
	_, err := provisioner.Provision("{}")
	require.Error(t, err)
	require.Equal(t, spi.ErrBadInput, spi.CodeFromError(err))
}

func TestCreateInstanceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock_ec2.NewMockEC2API(ctrl)

	runError := errors.New("request failed")
	expectKeyPairCreation(t, clientMock)
	clientMock.EXPECT().RunInstances(gomock.Any()).Return(&ec2.Reservation{}, runError)

	provisioner := NewInstanceProvisioner(clientMock, testCluster)
	id, err := provisioner.Provision(`{"Group": "a"}`)

	require.Error(t, err)
	require.Nil(t, id)
}

func TestDestroyInstanceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock_ec2.NewMockEC2API(ctrl)

	instanceID := "test-id"

	runError := errors.New("request failed")
	expectDescribeInstance(t, clientMock, instanceID, "keyName")
	expectKeyPairDeletion(t, clientMock, "keyName")
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
			instances = append(instances, &ec2.Instance{
				InstanceId:       aws.String(id),
				PrivateIpAddress: aws.String("127.0.0.1"),
			})
		}
		reservations = append(reservations, &ec2.Reservation{Instances: instances})
	}

	return &ec2.DescribeInstancesOutput{NextToken: nextToken, Reservations: reservations}
}

func TestDescribeInstancesRequest(t *testing.T) {
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
	clientMock := mock_ec2.NewMockEC2API(ctrl)

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
	descriptions, err := provisioner.DescribeInstances(group)

	require.NoError(t, err)
	require.Equal(t, []instance.Description{
		{ID: "a", PrivateIPAddress: "127.0.0.1"},
		{ID: "b", PrivateIPAddress: "127.0.0.1"},
		{ID: "c", PrivateIPAddress: "127.0.0.1"},
		{ID: "d", PrivateIPAddress: "127.0.0.1"},
		{ID: "e", PrivateIPAddress: "127.0.0.1"},
		{ID: "f", PrivateIPAddress: "127.0.0.1"},
		{ID: "g", PrivateIPAddress: "127.0.0.1"},
	}, descriptions)
}

func expectDescribeInstance(t *testing.T, clientMock *mock_ec2.MockEC2API, id string, keyName string) *gomock.Call {
	return clientMock.EXPECT().DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(id)},
	}).Return(&ec2.DescribeInstancesOutput{Reservations: []*ec2.Reservation{
		{Instances: []*ec2.Instance{{PublicIpAddress: aws.String(ipAddress), KeyName: aws.String(keyName)}}},
	}}, nil)
}

func expectKeyPairCreation(t *testing.T, clientMock *mock_ec2.MockEC2API) *string {
	key, err := rsa.GenerateKey(rand.Reader, 512)
	require.NoError(t, err)

	err = key.Validate()
	require.NoError(t, err)

	data := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))

	keyName := ""
	clientMock.EXPECT().CreateKeyPair(gomock.Any()).Do(func(input *ec2.CreateKeyPairInput) {
		require.NotNil(t, input.KeyName)
		require.NotEqual(t, "", *input.KeyName)
		keyName = *input.KeyName
	}).Return(&ec2.CreateKeyPairOutput{KeyMaterial: &data}, nil)
	return &keyName
}

func expectKeyPairDeletion(t *testing.T, clientMock *mock_ec2.MockEC2API, keyName string) {
	clientMock.EXPECT().DeleteKeyPair(&ec2.DeleteKeyPairInput{KeyName: aws.String(keyName)}).Return(nil, nil)
}

const (
	inputJSON = `
{
    "group": "group",
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
        "KeyName": "dev",
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
        }
    }
}
`
)
