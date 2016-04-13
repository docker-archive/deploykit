package aws

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"reflect"
	"testing"
)

func TestCreateSync(t *testing.T) {

	request := new(CreateInstanceRequest)
	err := json.Unmarshal([]byte(testCreateSync[0]), request)
	assert.Nil(t, err)

	m := awsFake{EC2API: &ec2.EC2{}}
	m.On("RunInstances", mock.MatchedBy(
		func(input *ec2.RunInstancesInput) bool {
			// check the input
			expectedInput := new(ec2.RunInstancesInput)
			err = json.Unmarshal([]byte(testCreateSync[1]), expectedInput)
			if err != nil {
				return false
			}
			return reflect.DeepEqual(expectedInput, input)
		})).Return(&ec2.Reservation{
		Instances: []*ec2.Instance{
			{InstanceId: aws.String("test-id")},
		},
	}, nil)

	instance, err := createSync(&m, *request)

	m.AssertExpectations(t)
	assert.Nil(t, err)
	assert.NotNil(t, instance)
}
