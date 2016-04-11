package aws

// CreateInstanceRequest is the struct used to create new instances.
type CreateInstanceRequest struct {
	AccessKey                string
	SecretKey                string
	SessionToken             string
	Region                   string
	AMI                      string
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
	MachineName              string
	VpcID                    string
	Zone                     string
	Monitoring               bool
}
