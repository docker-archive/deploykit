package aws

//go:generate mockgen -package mock -destination mock/mock_ec2iface.go github.com/aws/aws-sdk-go/service/ec2/ec2iface EC2API

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
