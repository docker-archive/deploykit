package aws

import (
	api "github.com/docker/libmachete"
	"github.com/docker/libmachete/provisioners/aws"
	. "gopkg.in/check.v1"
	"os"
	"testing"
)

func TestAws(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode.")
		return
	}
	TestingT(t)
}

type TestSuiteAws struct {
	accessKey string
	secretKey string
	region    string
}

var _ = Suite(&TestSuiteAws{})

func (suite *TestSuiteAws) SetUpSuite(c *C) {
	suite.accessKey = os.Getenv("AWS_ACCESS_KEY")
	suite.secretKey = os.Getenv("AWS_SECRET_KEY")

	c.Assert(len(suite.accessKey), Not(Equals), 0)
	c.Assert(len(suite.secretKey), Not(Equals), 0)

	suite.region = "us-west-2"
}

func (suite *TestSuiteAws) TearDownSuite(c *C) {
}

func (suite *TestSuiteAws) TestCreate(c *C) {
	client := aws.CreateClient(suite.region, suite.accessKey, suite.secretKey, "", 10)
	c.Assert(client, Not(IsNil))

	provisioner := aws.NewProvisioner(client)

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
	c.Assert(err, IsNil)

	seenCreateStarted := false
	seenCreateCompleted := false

	for event := range events {
		c.Log("event=", event)

		switch event.Type {
		case api.CreateStarted:
			seenCreateStarted = true
		case api.CreateCompleted:
			seenCreateCompleted = true
		}
	}

	c.Assert(seenCreateStarted, Equals, true)
	c.Assert(seenCreateCompleted, Equals, true)

}
