package aws

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/stretchr/testify/mock"
)

// LoggingEC2 implements some overrides of the EC2API methods and logs in the input to stdout
type LoggingEC2 struct {
	ec2iface.EC2API
}

// RunInstances implements the EC2API but logs the input to stdout before calling the base.
func (l *LoggingEC2) RunInstances(input *ec2.RunInstancesInput) (*ec2.Reservation, error) {
	buff, _ := json.MarshalIndent(input, "  ", "  ")
	fmt.Println(string(buff))
	return l.EC2API.RunInstances(input)
}

type awsMock struct {
	mock.Mock
	ec2iface.EC2API
}

func (mockClient *awsMock) RunInstances(input *ec2.RunInstancesInput) (*ec2.Reservation, error) {
	args := mockClient.Called(input)
	return args.Get(0).(*ec2.Reservation), args.Error(1)
}

func (mockClient *awsMock) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	args := mockClient.Called(input)
	return args.Get(0).(*ec2.DescribeInstancesOutput), args.Error(1)
}

func (mockClient *awsMock) CreateTags(input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	args := mockClient.Called(input)
	return args.Get(0).(*ec2.CreateTagsOutput), args.Error(1)
}

var (
	testCreateSync = []string{`
{
    "AvailabilityZone": "us-west-2a",
    "ImageID": "ami-30ee0d50",
    "BlockDeviceName": "/dev/sdb",
    "RootSize": 64,
    "VolumeType": "gp2",
    "DeleteOnTermination": true,
    "SecurityGroupIds": [
      "sg-973491f0"
    ],
    "SubnetID": "",
    "InstanceType": "t2.micro",
    "PrivateIPAddress": "",
    "AssociatePublicIPAddress": true,
    "PrivateIPOnly": false,
    "EbsOptimized": false,
    "IamInstanceProfile": "",
    "Tags": {
      "Name": "unit-test-create",
      "test": "aws-create-test"
    },
    "KeyName": "dev",
    "VpcID": "vpc-74c22510",
    "Zone": "a",
    "Monitoring": true
}
`, `
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
`,
	}
)
