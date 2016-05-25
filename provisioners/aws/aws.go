package aws

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/docker/libmachete"
	"github.com/docker/libmachete/provisioners/api"
	"reflect"
	"sort"
	"time"
)

// Builder is a ProvisionerBuilder for AWS.
type Builder struct {
}

func checkCredential(cred api.Credential) (c *credential, err error) {
	is := false
	if c, is = cred.(*credential); !is {
		err = fmt.Errorf("credential type mismatch: %v", reflect.TypeOf(cred))
		return
	}
	return
}

func getConfig(controls api.ProvisionControls) (*Config, error) {
	config := defaultConfig()

	region, ok := controls.GetString("region")
	if ok {
		config.Region = region
	} else {
		return nil, fmt.Errorf("No region in context")
	}

	if retries, ok, err := controls.GetInt("retries"); ok && err != nil {
		config.Retries = retries
	}

	if maxPoll, ok, err := controls.GetInt("check_instance_max_poll"); ok && err != nil {
		config.CheckInstanceMaxPoll = maxPoll
	}

	if pollInterval, ok, err := controls.GetInt("check_instance_poll_interval"); ok && err != nil {
		config.CheckInstancePollInterval = pollInterval
	}

	return config, nil
}

// ProvisionerWith returns a provision given the runtime context and credential
func ProvisionerWith(controls api.ProvisionControls, cred api.Credential) (api.Provisioner, error) {
	config, err := getConfig(controls)
	if err != nil {
		return nil, err
	}

	creds, err := checkCredential(cred)
	if err != nil {
		return nil, err
	}

	// TODO(wfarner): Consider using pointers for all fields in api.Credential.  Otherwise it is not possible to
	// distinguish between supplied empty values and absent values.  As a result, we must put the static credential
	// value last in the chain.  With proper handling of empty values, it probably belongs at the top of the list
	// as it is more explicit than values in the environment.
	client := CreateClient(config.Region, credentials.NewChainCredentials([]credentials.Provider{
		&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(session.New())},
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{},
		creds,
	}), config.Retries)

	return &provisioner{client: client, sleepFunction: time.Sleep, config: config}, nil
}

type provisioner struct {
	client        ec2iface.EC2API
	sleepFunction func(time.Duration)
	config        *Config
}

// New creates a new AWS provisioner that will use the provided EC2 API implementation and default config.
func New(client ec2iface.EC2API) api.Provisioner {
	return &provisioner{client: client, sleepFunction: time.Sleep, config: defaultConfig()}
}

// CreateClient creates the actual EC2 API client.
func CreateClient(region string, awsCredentials *credentials.Credentials, retryCount int) ec2iface.EC2API {
	return ec2.New(session.New(aws.NewConfig().
		WithRegion(region).
		WithCredentials(awsCredentials).
		WithLogger(getLogger()).
		WithLogLevel(aws.LogDebugWithHTTPBody).
		WithMaxRetries(retryCount)))
}

func getInstanceSync(client ec2iface.EC2API, instanceID string) (*ec2.Instance, error) {
	result, err := client.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{&instanceID},
	})
	if err != nil {
		return nil, err
	}
	if len(result.Reservations) != 1 || len(result.Reservations[0].Instances) != 1 {
		return nil, &ErrUnexpectedResponse{}
	}
	return result.Reservations[0].Instances[0], nil
}

func tagSync(client ec2iface.EC2API, request CreateInstanceRequest, instance *ec2.Instance) error {
	tags := []*ec2.Tag{}

	// Gather the tag keys in sorted order, to provide predictable tag order.  This is
	// particularly useful for tests.
	var keys []string
	for k := range request.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		key := k
		value := request.Tags[key]
		tags = append(tags, &ec2.Tag{
			Key:   &key,
			Value: &value,
		})
	}

	// Add a name tag
	name := "Name"
	nameValue := request.Name()
	tags = append(tags, &ec2.Tag{
		Key:   &name,
		Value: &nameValue,
	})
	_, err := client.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{instance.InstanceId},
		Tags:      tags,
	})
	return err
}

func createInstanceSync(
	client ec2iface.EC2API,
	request CreateInstanceRequest) (*ec2.Instance, error) {

	reservation, err := client.RunInstances(&ec2.RunInstancesInput{
		ImageId:  &request.ImageID,
		MinCount: aws.Int64(1),
		MaxCount: aws.Int64(1),
		Placement: &ec2.Placement{
			AvailabilityZone: &request.AvailabilityZone,
		},
		KeyName:      &request.KeyName,
		InstanceType: &request.InstanceType,
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{{
			DeviceIndex:              aws.Int64(0), // eth0
			Groups:                   makePointerSlice(request.SecurityGroupIds),
			SubnetId:                 &request.SubnetID,
			AssociatePublicIpAddress: &request.AssociatePublicIPAddress,
			DeleteOnTermination:      &request.DeleteOnTermination,
		}},
		Monitoring: &ec2.RunInstancesMonitoringEnabled{
			Enabled: &request.Monitoring,
		},
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Name: &request.IamInstanceProfile,
		},
		EbsOptimized: &request.EbsOptimized,
		BlockDeviceMappings: []*ec2.BlockDeviceMapping{
			{
				DeviceName: &request.BlockDeviceName,
				Ebs: &ec2.EbsBlockDevice{
					VolumeSize:          &request.RootSize,
					VolumeType:          &request.VolumeType,
					DeleteOnTermination: &request.DeleteOnTermination,
				},
			},
		},
	})

	if err != nil {
		return nil, err
	}

	if reservation == nil || len(reservation.Instances) != 1 {
		return nil, &ErrUnexpectedResponse{}
	}
	return reservation.Instances[0], nil
}

func (p *provisioner) blockUntilInstanceInState(instanceID string, instanceState string) error {
	return WaitUntil(
		p.sleepFunction,
		p.config.CheckInstanceMaxPoll,
		time.Duration(p.config.CheckInstancePollInterval)*time.Second,
		func() (bool, error) {
			inst, err := getInstanceSync(p.client, instanceID)
			return inst != nil && *inst.State.Name == instanceState, err
		})
}

func getProvisionTaskMap() *libmachete.TaskMap {
	return libmachete.NewTaskMap(
		libmachete.TaskSSHKeyGen.Override("AWS - upload generated SSH key", GenerateAndUploadSSHKey),
		libmachete.TaskCreateInstance,
		libmachete.TaskUserData,
		libmachete.TaskInstallDockerEngine,
	)
}

func getTeardownTaskMap() *libmachete.TaskMap {
	return libmachete.NewTaskMap(
		libmachete.TaskDestroyInstance,
		libmachete.TaskSSHKeyRemove.Override("AWS - remove ssh key", RemoveLocalAndUploadedSSHKey),
	)
}

// NewMachineRequest returns a canonical machine request suitable for this provisioner.
// This includes the standard workflow steps as well as the platform attributes.
func NewMachineRequest() api.MachineRequest {
	req := new(CreateInstanceRequest)
	req.Provisioner = ProvisionerName
	req.ProvisionerVersion = ProvisionerVersion
	req.Provision = getProvisionTaskMap().Names()
	req.Teardown = getTeardownTaskMap().Names()
	return req
}

// Name returns the name of the provisioner
func (p *provisioner) Name() string {
	return ProvisionerName
}

func (p *provisioner) NewRequestInstance() api.MachineRequest {
	return NewMachineRequest()
}

func ensureRequestType(req api.MachineRequest) (r *CreateInstanceRequest, err error) {
	is := false
	if r, is = req.(*CreateInstanceRequest); !is {
		err = fmt.Errorf("request type mismatch: %v", reflect.TypeOf(req))
		return
	}
	return
}

// GetInstanceID returns the infrastructure identifier given the state of the machine.
func (p *provisioner) GetInstanceID(req api.MachineRequest) (string, error) {
	ci, err := ensureRequestType(req)
	if err != nil {
		return "", err
	}
	return ci.InstanceID, nil
}

// GetIPAddress - this prefers private IP if it's set; make this behavior configurable as a
// machine template or context?
func (p *provisioner) GetIPAddress(req api.MachineRequest) (string, error) {
	ci, err := ensureRequestType(req)
	if err != nil {
		return "", err
	}
	if ci.PrivateIPAddress != "" {
		return ci.PrivateIPAddress, nil // TODO - make this configurable based on context??
	}
	return ci.PublicIPAddress, nil
}

func (p *provisioner) GetProvisionTasks(tasks []api.TaskName) ([]api.Task, error) {
	return getProvisionTaskMap().Filter(tasks)
}

func (p *provisioner) GetTeardownTasks(tasks []api.TaskName) ([]api.Task, error) {
	return getTeardownTaskMap().Filter(tasks)
}

// Validate checks the data and returns error if not valid
func validate(req *CreateInstanceRequest) error {
	// TODO finish this.
	return nil
}

func (p *provisioner) CreateInstance(
	req api.MachineRequest) (<-chan api.CreateInstanceEvent, error) {

	request, is := req.(*CreateInstanceRequest)
	if !is {
		return nil, &ErrInvalidRequest{}
	}

	if err := validate(request); err != nil {
		return nil, err
	}

	events := make(chan api.CreateInstanceEvent)
	go func() {
		defer close(events)

		events <- api.CreateInstanceEvent{Type: api.CreateInstanceStarted}

		instance, err := createInstanceSync(p.client, *request)
		if err != nil {
			events <- api.CreateInstanceEvent{
				Error: err,
				Type:  api.CreateInstanceError,
			}
			return
		}

		err = p.blockUntilInstanceInState(*instance.InstanceId, ec2.InstanceStateNameRunning)
		if err != nil {
			events <- api.CreateInstanceEvent{
				Error: err,
				Type:  api.CreateInstanceError,
			}
			return
		}

		err = tagSync(p.client, *request, instance)
		if err != nil {
			events <- api.CreateInstanceEvent{
				Error: err,
				Type:  api.CreateInstanceError,
			}
			return
		}

		provisioned, err := getInstanceSync(p.client, *instance.InstanceId)
		if err != nil {
			events <- api.CreateInstanceEvent{
				Error: err,
				Type:  api.CreateInstanceError,
			}
			return
		}

		// TODO(chungers) -- need to figure out reasonable way to separate request from state
		provisionedState := *request // copy
		if provisioned.PrivateIpAddress != nil {
			provisionedState.PrivateIPAddress = *provisioned.PrivateIpAddress
		}
		if provisioned.PublicIpAddress != nil {
			provisionedState.PublicIPAddress = *provisioned.PublicIpAddress
		}
		provisionedState.InstanceID = *instance.InstanceId

		log.Infoln("ProvisionedState=", provisionedState)

		events <- api.CreateInstanceEvent{
			Type:       api.CreateInstanceCompleted,
			InstanceID: *instance.InstanceId,
			Machine:    &provisionedState,
		}
	}()

	return events, nil
}

func destroyInstanceSync(client ec2iface.EC2API, instanceID string) error {
	result, err := client.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: []*string{&instanceID}})

	if err != nil {
		return err
	}

	if len(result.TerminatingInstances) == 1 {
		return nil
	}

	// There was no match for the instance ID.
	return &ErrInvalidRequest{}
}

func (p *provisioner) DestroyInstance(instanceID string) (<-chan api.DestroyInstanceEvent, error) {
	events := make(chan api.DestroyInstanceEvent)

	go func() {
		defer close(events)

		events <- api.DestroyInstanceEvent{Type: api.DestroyInstanceStarted}

		err := destroyInstanceSync(p.client, instanceID)
		if err != nil {
			events <- api.DestroyInstanceEvent{
				Type:  api.DestroyInstanceError,
				Error: err}
			return
		}

		err = p.blockUntilInstanceInState(instanceID, ec2.InstanceStateNameTerminated)
		if err != nil {
			events <- api.DestroyInstanceEvent{
				Error: err,
				Type:  api.DestroyInstanceError,
			}
			return
		}

		events <- api.DestroyInstanceEvent{Type: api.DestroyInstanceCompleted}
	}()

	return events, nil
}
