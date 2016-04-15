package aws

import (
	api "github.com/docker/libmachete"
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

	provisioner := aws.New(aws.CreateClient(region, accessKey, secretKey, "", 10))

	request := aws.CreateInstanceRequest{
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

	events, err := provisioner.Create(request)
	require.Nil(t, err)

	var eventTypes []api.EventType
	for event := range events {
		t.Log("event=", event)
		eventTypes = append(eventTypes, event.Type)
	}

	require.Equal(t, []api.EventType{api.CreateStarted, api.CreateCompleted}, eventTypes)
}
