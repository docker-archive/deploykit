package aws

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/provisioners/aws/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"reflect"
	"testing"
	"time"
)

//go:generate mockgen -package mock -destination mock/mock_ec2iface.go github.com/aws/aws-sdk-go/service/ec2/ec2iface EC2API

func noSleep(time.Duration) {
	// no-op - don't sleep in tests
}

func TestCreateInstanceSync(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock.NewMockEC2API(ctrl)

	request := new(CreateInstanceRequest)
	require.Nil(t, json.Unmarshal([]byte(testCreateSync[0]), request))

	reservation := ec2.Reservation{
		Instances: []*ec2.Instance{{InstanceId: aws.String("test-id")}}}
	// Validates command against a known-good value.
	matcher := func(input *ec2.RunInstancesInput) {
		expectedInput := new(ec2.RunInstancesInput)
		require.Nil(t, json.Unmarshal([]byte(testCreateSync[1]), expectedInput))
		if !reflect.DeepEqual(expectedInput, input) {
			t.Error("Expected and actual did not match.", expectedInput, input)
		}
	}
	clientMock.EXPECT().RunInstances(gomock.Any()).Do(matcher).Return(&reservation, nil)

	fmt.Println(">>>>>>", *request)

	instance, err := createInstanceSync(clientMock, *request)

	require.Nil(t, err)
	require.NotNil(t, instance)
}

type WrongRequestType struct {
	api.BaseMachineRequest
}

func (w WrongRequestType) ProvisionWorkflow() []api.TaskName {
	return []api.TaskName{}
}

func TestCreateIncompatibleType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock.NewMockEC2API(ctrl)

	p := &provisioner{client: clientMock, sleepFunction: noSleep, config: defaultConfig()}
	_, err := p.CreateInstance(&WrongRequestType{})
	require.NotNil(t, err)
}

// TODO(wfarner): Inline this function.
func makeDescribeOutput(instanceState string) *ec2.DescribeInstancesOutput {
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			{Instances: []*ec2.Instance{
				{State: &ec2.InstanceState{Name: &instanceState}}},
			},
		},
	}
}

func collectCreateInstanceEvents(
	eventChan <-chan api.CreateInstanceEvent) []api.CreateInstanceEvent {

	var events []api.CreateInstanceEvent
	for event := range eventChan {
		events = append(events, event)
	}
	return events
}

func expectDescribeCall(
	clientMock *mock.MockEC2API,
	instanceID string,
	returnedState string) {

	clientMock.EXPECT().
		DescribeInstances(&ec2.DescribeInstancesInput{InstanceIds: []*string{&instanceID}}).
		Return(makeDescribeOutput(returnedState), nil)
}

func TestCreateInstanceSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock.NewMockEC2API(ctrl)

	instanceID := "test-id"
	reservation := ec2.Reservation{Instances: []*ec2.Instance{{InstanceId: &instanceID}}}
	clientMock.EXPECT().RunInstances(gomock.Any()).Return(&reservation, nil)

	// Simulate the instance not being available yet, the first describe returns nothing.
	expectDescribeCall(clientMock, instanceID, ec2.InstanceStateNamePending)

	// The instance is now running.
	expectDescribeCall(clientMock, instanceID, ec2.InstanceStateNameRunning)

	// The instance is now running.  Second call to get the ip address
	expectDescribeCall(clientMock, instanceID, ec2.InstanceStateNameRunning)

	tagRequest := ec2.CreateTagsInput{
		Resources: []*string{&instanceID},
		Tags: []*ec2.Tag{
			{Key: aws.String("test"), Value: aws.String("test2")},
			{Key: aws.String("Name"), Value: aws.String("test-instance")},
		},
	}
	clientMock.EXPECT().CreateTags(&tagRequest).Return(&ec2.CreateTagsOutput{}, nil)

	provisioner := provisioner{client: clientMock, sleepFunction: noSleep, config: defaultConfig()}
	request := &CreateInstanceRequest{
		BaseMachineRequest: api.BaseMachineRequest{MachineName: "test-instance"},
		Tags:               map[string]string{"test": "test2"},
	}
	eventChan, err := provisioner.CreateInstance(request)

	require.Nil(t, err)
	request.InstanceID = instanceID
	expectedEvents := []api.CreateInstanceEvent{
		{Type: api.CreateInstanceStarted},
		{Type: api.CreateInstanceCompleted, InstanceID: instanceID, Machine: request}}
	require.Equal(t, expectedEvents, collectCreateInstanceEvents(eventChan))
}

func TestCreateInstanceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock.NewMockEC2API(ctrl)

	runError := errors.New("request failed")
	clientMock.EXPECT().RunInstances(gomock.Any()).Return(&ec2.Reservation{}, runError)

	provisioner := provisioner{client: clientMock, sleepFunction: noSleep, config: defaultConfig()}
	eventChan, err := provisioner.CreateInstance(&CreateInstanceRequest{})

	require.Nil(t, err)
	expectedEvents := []api.CreateInstanceEvent{
		{Type: api.CreateInstanceStarted},
		{Type: api.CreateInstanceError, Error: runError}}
	require.Equal(t, expectedEvents, collectCreateInstanceEvents(eventChan))
}

func collectDestroyInstanceEvents(
	eventChan <-chan api.DestroyInstanceEvent) []api.DestroyInstanceEvent {

	var events []api.DestroyInstanceEvent
	for event := range eventChan {
		events = append(events, event)
	}
	return events
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

	// Instance is in terminating state, not yet terminated.
	expectDescribeCall(clientMock, instanceID, ec2.InstanceStateNameStopping)

	expectDescribeCall(clientMock, instanceID, ec2.InstanceStateNameTerminated)

	provisioner := provisioner{client: clientMock, sleepFunction: noSleep, config: defaultConfig()}
	eventChan, err := provisioner.DestroyInstance(instanceID)

	require.Nil(t, err)
	expectedEvents := []api.DestroyInstanceEvent{
		{Type: api.DestroyInstanceStarted},
		{Type: api.DestroyInstanceCompleted}}
	require.Equal(t, expectedEvents, collectDestroyInstanceEvents(eventChan))
}

func TestDestroyInstanceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	clientMock := mock.NewMockEC2API(ctrl)

	runError := errors.New("request failed")
	clientMock.EXPECT().TerminateInstances(gomock.Any()).
		Return(&ec2.TerminateInstancesOutput{}, runError)

	provisioner := provisioner{client: clientMock, sleepFunction: noSleep, config: defaultConfig()}
	eventChan, err := provisioner.DestroyInstance("test-id")

	require.Nil(t, err)
	expectedEvents := []api.DestroyInstanceEvent{
		{Type: api.DestroyInstanceStarted},
		{Type: api.DestroyInstanceError, Error: runError}}
	require.Equal(t, expectedEvents, collectDestroyInstanceEvents(eventChan))
}

const yamlDoc = `name: database
availability_zone: us-west-2a
image_id: ami-5
block_device_name: /dev/sdb
root_size: 64
volume_type: gp2
delete_on_termination: true
security_group_ids: [sg-1, sg-2]
subnet_id: my-subnet-id
instance_type: t2.micro
private_ip_address: 127.0.0.1
associate_public_ip_address: true
private_ip_only: true
ebs_optimized: true
iam_instance_profile: my-iam-profile
tags:
  Name: unit-test-create
  test: aws-create-test
key_name: dev
vpc_id: my-vpc-id
monitoring: true`

func TestYamlSpec(t *testing.T) {
	expected := CreateInstanceRequest{
		BaseMachineRequest:       api.BaseMachineRequest{MachineName: "database"},
		AvailabilityZone:         "us-west-2a",
		ImageID:                  "ami-5",
		BlockDeviceName:          "/dev/sdb",
		RootSize:                 64,
		VolumeType:               "gp2",
		DeleteOnTermination:      true,
		SecurityGroupIds:         []string{"sg-1", "sg-2"},
		SubnetID:                 "my-subnet-id",
		InstanceType:             "t2.micro",
		PrivateIPAddress:         "127.0.0.1",
		AssociatePublicIPAddress: true,
		PrivateIPOnly:            true,
		EbsOptimized:             true,
		IamInstanceProfile:       "my-iam-profile",
		Tags: map[string]string{
			"Name": "unit-test-create",
			"test": "aws-create-test"},
		KeyName:    "dev",
		VpcID:      "my-vpc-id",
		Monitoring: true,
	}
	actual := CreateInstanceRequest{}
	err := yaml.Unmarshal([]byte(yamlDoc), &actual)
	require.Nil(t, err)
	require.Equal(t, expected, actual)
}
