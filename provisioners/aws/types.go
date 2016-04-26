package aws

import "github.com/docker/libmachete/provisioners/api"

// CreateInstanceRequest is the struct used to create new instances.
type CreateInstanceRequest struct {
	// TODO(wfarner): How do we include a Name field and yaml tag? Also note that this struct
	//                does not have an implementation for GetName(), required by MachineRequest.
	//                Need to address this as well.
	api.MachineRequest

	AvailabilityZone         string            `yaml:"availability-zone"`
	ImageID                  string            `yaml:"image-id"`
	BlockDeviceName          string            `yaml:"block-device-name"`
	RootSize                 int64             `yaml:"root-size"`
	VolumeType               string            `yaml:"volume-type"`
	DeleteOnTermination      bool              `yaml:"delete-on-termination"`
	SecurityGroupIds         []string          `yaml:"security-group-ids,flow"`
	SubnetID                 string            `yaml:"subnet-id"`
	InstanceType             string            `yaml:"instance-type"`
	PrivateIPAddress         string            `yaml:"private-ip-address"`
	AssociatePublicIPAddress bool              `yaml:"associate-public-ip-address"`
	PrivateIPOnly            bool              `yaml:"private-ip-only"`
	EbsOptimized             bool              `yaml:"ebs-optimized"`
	IamInstanceProfile       string            `yaml:"iam-instance-profile"`
	Tags                     map[string]string `yaml:"tags"`
	KeyName                  string            `yaml:"key-name"`
	VpcID                    string            `yaml:"vpc-id"`
	Zone                     string            `yaml:"zone"`
	Monitoring               bool              `yaml:"monitoring"`
}

// Validate checks the data and returns error if not valid
func (req CreateInstanceRequest) Validate() error {
	// TODO finish this.
	return nil
}
