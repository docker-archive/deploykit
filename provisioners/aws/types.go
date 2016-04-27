package aws

// CreateInstanceRequest is the struct used to create new instances.
type CreateInstanceRequest struct {
	MachineName              string            `yaml:"name"`
	AvailabilityZone         string            `yaml:"availability_zone"`
	ImageID                  string            `yaml:"image_id"`
	BlockDeviceName          string            `yaml:"block_device_name"`
	RootSize                 int64             `yaml:"root_size"`
	VolumeType               string            `yaml:"volume_type"`
	DeleteOnTermination      bool              `yaml:"delete_on_termination"`
	SecurityGroupIds         []string          `yaml:"security_group_ids,flow"`
	SubnetID                 string            `yaml:"subnet_id"`
	InstanceType             string            `yaml:"instance_type"`
	PrivateIPAddress         string            `yaml:"private_ip_address"`
	AssociatePublicIPAddress bool              `yaml:"associate_public_ip_address"`
	PrivateIPOnly            bool              `yaml:"private_ip_only"`
	EbsOptimized             bool              `yaml:"ebs_optimized"`
	IamInstanceProfile       string            `yaml:"iam_instance_profile"`
	Tags                     map[string]string `yaml:"tags"`
	KeyName                  string            `yaml:"key_name"`
	VpcID                    string            `yaml:"vpc_id"`
	Zone                     string            `yaml:"zone"`
	Monitoring               bool              `yaml:"monitoring"`
}

// Name gets the name to be associated with the machine.
func (req CreateInstanceRequest) Name() string {
	return req.MachineName
}

// Validate checks the data and returns error if not valid
func (req CreateInstanceRequest) Validate() error {
	// TODO finish this.
	return nil
}
