package aws

var (
	testCreateSync = []string{`
{
    "availability_zone": "us-west-2a",
    "image_id": "ami-30ee0d50",
    "block_device_name": "/dev/sdb",
    "root_size": 64,
    "volume_type": "gp2",
    "delete_on_termination": true,
    "security_group_ids": [
      "sg-973491f0"
    ],
    "subnet_id": "",
    "instance_type": "t2.micro",
    "private_ip_ddress": "",
    "associate_public_ip_address": true,
    "private_ip_only": false,
    "ebs_optimized": false,
    "iam_instance_profile": "",
    "tags": {
      "Name": "unit-test-create",
      "test": "aws-create-test"
    },
    "key_name": "dev",
    "vpc_id": "vpc-74c22510",
    "monitoring": true
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
