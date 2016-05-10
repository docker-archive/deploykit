package aws

import "github.com/docker/libmachete/provisioners/api"

// CreateInstanceRequest is the struct used to create new instances.
type CreateInstanceRequest struct {
	api.BaseMachineRequest   `yaml:",inline"`
	AvailabilityZone         string            `yaml:"availability_zone" json:"availability_zone"`
	ImageID                  string            `yaml:"image_id" json:"image_id"`
	BlockDeviceName          string            `yaml:"block_device_name" json:"block_device_name"`
	RootSize                 int64             `yaml:"root_size" json:"root_size"`
	VolumeType               string            `yaml:"volume_type" json:"volume_type"`
	DeleteOnTermination      bool              `yaml:"delete_on_termination" json:"delete_on_termination"`
	SecurityGroupIds         []string          `yaml:"security_group_ids,flow" json:"security_group_ids"`
	SubnetID                 string            `yaml:"subnet_id" json:"subnet_id"`
	InstanceType             string            `yaml:"instance_type" json:"instance_type"`
	PrivateIPAddress         string            `yaml:"private_ip_address" json:"private_ip_address"`
	AssociatePublicIPAddress bool              `yaml:"associate_public_ip_address" json:"associate_public_ip_address"`
	PrivateIPOnly            bool              `yaml:"private_ip_only" json:"private_ip_only"`
	EbsOptimized             bool              `yaml:"ebs_optimized" json:"ebs_optimized"`
	IamInstanceProfile       string            `yaml:"iam_instance_profile" json:"iam_instance_profile"`
	Tags                     map[string]string `yaml:"tags" json:"tags"`
	KeyName                  string            `yaml:"key_name" json:"key_name"`
	VpcID                    string            `yaml:"vpc_id" json:"vpc_id"`
	Monitoring               bool              `yaml:"monitoring" json:"monitoring"`
}

// Validate checks the data and returns error if not valid
func (req CreateInstanceRequest) Validate() error {
	// TODO finish this.
	return nil
}
