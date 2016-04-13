package aws

// CreateInstanceRequest is the struct used to create new instances.
type CreateInstanceRequest struct {
	AvailabilityZone         string
	ImageID                  string
	BlockDeviceName          string
	RootSize                 int64
	VolumeType               string
	DeleteOnTermination      bool
	SecurityGroupIds         []string
	SubnetID                 string
	InstanceType             string
	PrivateIPAddress         string
	AssociatePublicIPAddress bool
	PrivateIPOnly            bool
	EbsOptimized             bool
	IamInstanceProfile       string
	Tags                     map[string]string
	KeyName                  string
	VpcID                    string
	Zone                     string
	Monitoring               bool
}

// Validate checks the data and returns error if not valid
func (req CreateInstanceRequest) Validate() error {
	// TODO finish this.
	return nil
}
