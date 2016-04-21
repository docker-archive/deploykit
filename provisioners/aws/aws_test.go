package aws

import (
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	api "github.com/docker/libmachete"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
	"time"
)

func noSleep(time.Duration) {
	// no-op - don't sleep in tests
}

func TestCreateInstanceSync(t *testing.T) {
	request := new(CreateInstanceRequest)
	require.Nil(t, json.Unmarshal([]byte(testCreateSync[0]), request))

	clientMock := awsMock{EC2API: &ec2.EC2{}}
	reservation := ec2.Reservation{
		Instances: []*ec2.Instance{{InstanceId: aws.String("test-id")}}}
	// Validates command against a known-good value.
	matcher := func(input *ec2.RunInstancesInput) bool {
		expectedInput := new(ec2.RunInstancesInput)
		require.Nil(t, json.Unmarshal([]byte(testCreateSync[1]), expectedInput))
		return reflect.DeepEqual(expectedInput, input)
	}
	clientMock.On("RunInstances", mock.MatchedBy(matcher)).Once().Return(&reservation, nil)

	instance, err := createInstanceSync(&clientMock, *request)

	clientMock.AssertExpectations(t)
	require.Nil(t, err)
	require.NotNil(t, instance)
}

func TestCreateIncompatibleType(t *testing.T) {
	p := &provisioner{client: &awsMock{EC2API: &ec2.EC2{}}, sleepFunction: noSleep}
	_, err := p.CreateInstance("wrongtype")
	require.NotNil(t, err)
}

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

func expectDescribeCall(clientMock *awsMock, instanceID string, returnedState string) {
	clientMock.On(
		"DescribeInstances",
		&ec2.DescribeInstancesInput{InstanceIds: []*string{&instanceID}}).
		Once().
		Return(makeDescribeOutput(returnedState), nil)
}

func TestCreateInstanceSuccess(t *testing.T) {
	clientMock := awsMock{EC2API: &ec2.EC2{}}

	instanceID := "test-id"
	reservation := ec2.Reservation{Instances: []*ec2.Instance{{InstanceId: &instanceID}}}
	clientMock.On(
		"RunInstances",
		mock.AnythingOfType("*ec2.RunInstancesInput")).
		Once().
		Return(&reservation, nil)

	// Simulate the instance not being available yet, the first describe returns nothing.
	expectDescribeCall(&clientMock, instanceID, ec2.InstanceStateNamePending)

	// The instance is now running.
	expectDescribeCall(&clientMock, instanceID, ec2.InstanceStateNameRunning)

	clientMock.On(
		"CreateTags",
		&ec2.CreateTagsInput{
			Resources: []*string{&instanceID},
			Tags: []*ec2.Tag{
				{Key: aws.String("name"), Value: aws.String("test-instance")},
				{Key: aws.String("test"), Value: aws.String("test2")}},
		}).
		Once().
		Return(&ec2.CreateTagsOutput{}, nil)

	provisioner := provisioner{client: &clientMock, sleepFunction: noSleep}
	eventChan, err := provisioner.CreateInstance(CreateInstanceRequest{
		Tags: map[string]string{"name": "test-instance", "test": "test2"},
	})

	require.Nil(t, err)
	expectedEvents := []api.CreateInstanceEvent{
		{Type: api.CreateInstanceStarted},
		{Type: api.CreateInstanceCompleted, InstanceID: instanceID}}
	require.Equal(t, expectedEvents, collectCreateInstanceEvents(eventChan))

	clientMock.AssertExpectations(t)
}

func TestCreateInstanceError(t *testing.T) {
	clientMock := awsMock{EC2API: &ec2.EC2{}}

	runError := errors.New("request failed")
	clientMock.On(
		"RunInstances",
		mock.AnythingOfType("*ec2.RunInstancesInput")).
		Once().
		Return(&ec2.Reservation{}, runError)

	provisioner := provisioner{client: &clientMock, sleepFunction: noSleep}
	eventChan, err := provisioner.CreateInstance(CreateInstanceRequest{})

	require.Nil(t, err)
	expectedEvents := []api.CreateInstanceEvent{
		{Type: api.CreateInstanceStarted},
		{Type: api.CreateInstanceError, Error: runError}}
	require.Equal(t, expectedEvents, collectCreateInstanceEvents(eventChan))

	clientMock.AssertExpectations(t)
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
	clientMock := awsMock{EC2API: &ec2.EC2{}}

	instanceID := "test-id"

	clientMock.On(
		"TerminateInstances",
		&ec2.TerminateInstancesInput{InstanceIds: []*string{&instanceID}}).
		Once().
		Return(&ec2.TerminateInstancesOutput{
			TerminatingInstances: []*ec2.InstanceStateChange{{
				InstanceId: &instanceID,
			}}}, nil)

	// Instance is in terminating state, not yet terminated.
	expectDescribeCall(&clientMock, instanceID, ec2.InstanceStateNameStopping)

	expectDescribeCall(&clientMock, instanceID, ec2.InstanceStateNameTerminated)

	provisioner := provisioner{client: &clientMock, sleepFunction: noSleep}
	eventChan, err := provisioner.DestroyInstance(instanceID)

	require.Nil(t, err)
	expectedEvents := []api.DestroyInstanceEvent{
		{Type: api.DestroyInstanceStarted},
		{Type: api.DestroyInstanceCompleted}}
	require.Equal(t, expectedEvents, collectDestroyInstanceEvents(eventChan))

	clientMock.AssertExpectations(t)
}

func TestDestroyInstanceError(t *testing.T) {
	clientMock := awsMock{EC2API: &ec2.EC2{}}

	runError := errors.New("request failed")
	clientMock.On(
		"TerminateInstances",
		mock.AnythingOfType("*ec2.TerminateInstancesInput")).
		Once().
		Return(&ec2.TerminateInstancesOutput{}, runError)

	provisioner := provisioner{client: &clientMock, sleepFunction: noSleep}
	eventChan, err := provisioner.DestroyInstance("test-id")

	require.Nil(t, err)
	expectedEvents := []api.DestroyInstanceEvent{
		{Type: api.DestroyInstanceStarted},
		{Type: api.DestroyInstanceError, Error: runError}}
	require.Equal(t, expectedEvents, collectDestroyInstanceEvents(eventChan))

	clientMock.AssertExpectations(t)
}
