package aws

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/provisioners/aws"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func requireEnvVar(t *testing.T, varName string) string {
	value := os.Getenv(varName)
	require.NotEmpty(t, value, varName+" environment variable must be set")
	return value
}

func TestCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode.")
		return
	}

	accessKey := requireEnvVar(t, "AWS_ACCESS_KEY")
	secretKey := requireEnvVar(t, "AWS_SECRET_KEY")

	region := os.Getenv("AWS_REGION")
	if region == "" {
		// NOTE: When the region is misconfigured, strange errors can occur within the
		// AWS client.  For example, as of this writing, a region of "us-west2" results
		// in client retries and output such as:
		// 2016/04/14 18:02:46 DEBUG: Response ec2/RunInstances Details:
		// ---[ RESPONSE ]--------------------------------------
		// HTTP/0.0 0 status code 0
		// Content-Length: 0
		// -----------------------------------------------------
		// 2016/04/14 18:02:50 Request body type has been overwritten. May cause race conditions
		//
		// It would be nice if our tooling could handle this type of bad input with
		// greater robustness.
		region = "us-west-2"
	}

	awsCredentials := credentials.NewStaticCredentials(accessKey, secretKey, "")
	provisioner := aws.New(aws.CreateClient(region, awsCredentials, 10))

	request := &aws.CreateInstanceRequest{
		AvailabilityZone:         "us-west-2a",
		ImageID:                  "ami-30ee0d50",
		BlockDeviceName:          "/dev/sdb",
		RootSize:                 64,
		VolumeType:               "gp2",
		DeleteOnTermination:      true,
		SecurityGroupIds:         []string{"sg-973491f0"},
		InstanceType:             "t2.micro",
		AssociatePublicIPAddress: true,
		PrivateIPOnly:            false,
		EbsOptimized:             false,
		Tags: map[string]string{
			"Name": "unit-test-create",
			"test": "aws-create-test",
		},
		KeyName:    "dev",
		VpcID:      "vpc-74c22510",
		Zone:       "a",
		Monitoring: true,
	}

	createEvents, err := provisioner.CreateInstance(request)
	require.Nil(t, err)

	var createEventTypes []api.CreateInstanceEventType
	var instanceID string
	for event := range createEvents {
		t.Log("event=", event)
		createEventTypes = append(createEventTypes, event.Type)

		if event.InstanceID != "" {
			instanceID = event.InstanceID
		}
	}

	expectedCreateEvents := []api.CreateInstanceEventType{
		api.CreateInstanceStarted,
		api.CreateInstanceCompleted}
	require.Equal(t, expectedCreateEvents, createEventTypes)

	require.NotEmpty(t, instanceID)
	destroyEvents, err := provisioner.DestroyInstance(instanceID)
	require.Nil(t, err)

	var destroyEventTypes []api.DestroyInstanceEventType
	for event := range destroyEvents {
		t.Log("event=", event)
		destroyEventTypes = append(destroyEventTypes, event.Type)
	}

	expectedDestroyEvents := []api.DestroyInstanceEventType{
		api.DestroyInstanceStarted,
		api.DestroyInstanceCompleted}
	require.Equal(t, expectedDestroyEvents, destroyEventTypes)
}
