package aws

import "github.com/docker/libmachete/machete/spi/instance"

type createInstanceRequest struct {
	Group                    instance.GroupID  `json:"group"`
	Provisioner              string            `json:"provisioner,omitempty"`
	ProvisionerVersion       string            `json:"version,omitempty"`
	Provision                []string          `json:"provision,omitempty"`
	Teardown                 []string          `json:"teardown,omitempty"`
	AssociatePublicIPAddress bool              `json:"associate_public_ip_address"`
	AvailabilityZone         string            `json:"availability_zone"`
	BlockDeviceName          string            `json:"block_device_name"`
	DeleteOnTermination      bool              `json:"delete_on_termination"`
	EbsOptimized             bool              `json:"ebs_optimized"`
	IamInstanceProfile       string            `json:"iam_instance_profile"`
	ImageID                  string            `json:"image_id"`
	InstanceType             string            `json:"instance_type"`
	KeyName                  string            `json:"key_name"`
	Monitoring               bool              `json:"monitoring"`
	PrivateIPOnly            bool              `json:"private_ip_only"`
	RootSize                 int64             `json:"root_size"`
	SecurityGroupIds         []string          `json:"security_group_ids"`
	SubnetID                 string            `json:"subnet_id"`
	Tags                     map[string]string `json:"tags"`
	VolumeType               string            `json:"volume_type"`
	VpcID                    string            `json:"vpc_id"`
}
