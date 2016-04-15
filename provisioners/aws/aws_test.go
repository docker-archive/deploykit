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

func TestCreateSync(t *testing.T) {
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

	instance, err := createSync(&clientMock, *request)

	clientMock.AssertExpectations(t)
	require.Nil(t, err)
	require.NotNil(t, instance)
}

func TestCreateIncompatibleType(t *testing.T) {
	p := &provisioner{client: &awsMock{EC2API: &ec2.EC2{}}, sleepFunction: noSleep}
	_, err := p.Create("wrongtype")
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

func collectEvents(eventChan <-chan api.CreateEvent) []api.CreateEvent {
	var events []api.CreateEvent
	for event := range eventChan {
		events = append(events, event)
	}
	return events
}

func TestCreateSuccess(t *testing.T) {
	clientMock := awsMock{EC2API: &ec2.EC2{}}

	instanceID := "test-id"
	reservation := ec2.Reservation{Instances: []*ec2.Instance{{InstanceId: &instanceID}}}
	clientMock.On(
		"RunInstances",
		mock.AnythingOfType("*ec2.RunInstancesInput")).
		Once().
		Return(&reservation, nil)

	// Simulate the instance not being available yet, the first describe returns nothing.
	clientMock.On(
		"DescribeInstances",
		&ec2.DescribeInstancesInput{InstanceIds: []*string{&instanceID}}).
		Once().
		Return(makeDescribeOutput(ec2.InstanceStateNamePending), nil)

	// The instance is now running.
	clientMock.On(
		"DescribeInstances",
		&ec2.DescribeInstancesInput{InstanceIds: []*string{&instanceID}}).
		Once().
		Return(makeDescribeOutput(ec2.InstanceStateNameRunning), nil)

	clientMock.On(
		"CreateTags",
		&ec2.CreateTagsInput{
			Resources: []*string{&instanceID},
			Tags: []*ec2.Tag{
				// Note: The order of this slice is dependent upon the iteration
				// order of the tag map.  Figure out a better way to validate if
				// this proves to be brittle.
				{Key: aws.String("name"), Value: aws.String("test-instance")},
				{Key: aws.String("test"), Value: aws.String("test2")}},
		}).
		Once().
		Return(&ec2.CreateTagsOutput{}, nil)

	provisioner := provisioner{client: &clientMock, sleepFunction: noSleep}
	eventChan, err := provisioner.Create(CreateInstanceRequest{
		Tags: map[string]string{"name": "test-instance", "test": "test2"},
	})

	require.Nil(t, err)
	expectedEvents := []api.CreateEvent{
		{Type: api.CreateStarted},
		{Type: api.CreateCompleted, ResourceID: instanceID}}
	require.Equal(t, expectedEvents, collectEvents(eventChan))

	clientMock.AssertExpectations(t)
}

func TestCreateError(t *testing.T) {
	clientMock := awsMock{EC2API: &ec2.EC2{}}

	runError := errors.New("request failed")
	clientMock.On(
		"RunInstances",
		mock.AnythingOfType("*ec2.RunInstancesInput")).
		Once().
		Return(&ec2.Reservation{}, runError)

	provisioner := provisioner{client: &clientMock, sleepFunction: noSleep}
	eventChan, err := provisioner.Create(CreateInstanceRequest{})

	require.Nil(t, err)
	expectedEvents := []api.CreateEvent{
		{Type: api.CreateStarted},
		{Type: api.CreateError, Error: runError}}
	require.Equal(t, expectedEvents, collectEvents(eventChan))

	clientMock.AssertExpectations(t)
}
