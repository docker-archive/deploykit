package aws

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/docker/libmachete/spi"
	"github.com/docker/libmachete/spi/instance"
	"sort"
	"time"
)

const (
	// VolumeTag is the AWS tag name used to associate unique identifiers (instance.VolumeID) with volumes.
	VolumeTag = "docker-machete-volume"
)

// Provisioner is an instance provisioner for AWS.
type Provisioner struct {
	Client ec2iface.EC2API
}

type properties struct {
	Region   string
	Retries  int
	Instance json.RawMessage
}

// NewPluginFromProperties creates a new AWS plugin based on a JSON configuration.
func NewPluginFromProperties(pluginProperties json.RawMessage) (instance.Plugin, string, error) {
	props := properties{Retries: 5}
	err := json.Unmarshal([]byte(pluginProperties), &props)
	if err != nil {
		return nil, "", err
	}

	providers := []credentials.Provider{
		&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(session.New())},
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{},
	}

	client := session.New(aws.NewConfig().
		WithRegion(props.Region).
		WithCredentials(credentials.NewChainCredentials(providers)).
		WithLogger(getLogger()).
		WithMaxRetries(props.Retries))

	instancePlugin := NewInstancePlugin(ec2.New(client))

	// TODO(wfarner): Provide a way for the plugin to validate an instance request to identify bad configurations
	// more quickly.
	return instancePlugin, string(props.Instance), nil
}

// NewInstancePlugin creates a new plugin that creates instances in AWS EC2.
func NewInstancePlugin(client ec2iface.EC2API) instance.Plugin {
	return &Provisioner{Client: client}
}

func (p Provisioner) tagInstance(
	instance *ec2.Instance,
	systemTags map[string]string,
	userTags map[string]string) error {

	ec2Tags := []*ec2.Tag{}

	// Gather the tag keys in sorted order, to provide predictable tag order.  This is
	// particularly useful for tests.
	var keys []string
	for k := range userTags {
		keys = append(keys, k)
	}
	for k := range systemTags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		key := k
		value, exists := userTags[key]

		// System tags overwrite user tags.
		systemValue, exists := systemTags[key]
		if exists {
			value = systemValue
		}

		ec2Tags = append(ec2Tags, &ec2.Tag{Key: &key, Value: &value})
	}

	_, err := p.Client.CreateTags(&ec2.CreateTagsInput{Resources: []*string{instance.InstanceId}, Tags: ec2Tags})
	return err
}

// CreateInstanceRequest is the concrete provision request type.
type CreateInstanceRequest struct {
	Tags              map[string]string     `json:"tags"`
	RunInstancesInput ec2.RunInstancesInput `json:"run_instances_input"`
}

// Validate performs local checks to determine if the request is valid.
func (p Provisioner) Validate(req json.RawMessage) error {
	// TODO(wfarner): Implement
	return nil
}

// Provision creates a new instance.
func (p Provisioner) Provision(
	req json.RawMessage,
	volume *instance.VolumeID,
	tags map[string]string) (*instance.ID, error) {

	request := CreateInstanceRequest{}
	err := json.Unmarshal(req, &request)
	if err != nil {
		return nil, spi.NewError(spi.ErrBadInput, fmt.Sprintf("Invalid input formatting: %s", err))
	}

	request.RunInstancesInput.MinCount = aws.Int64(1)
	request.RunInstancesInput.MaxCount = aws.Int64(1)

	if request.RunInstancesInput.UserData != nil {
		request.RunInstancesInput.UserData = aws.String(
			base64.StdEncoding.EncodeToString([]byte(*request.RunInstancesInput.UserData)))
	}

	var awsVolumeID *string
	if volume != nil {
		volumes, err := p.Client.DescribeVolumes(&ec2.DescribeVolumesInput{
			Filters: []*ec2.Filter{
				// TODO(wfarner): Need a way to disambiguate between volumes associated with different
				// clusters.  Currently, volume IDs are private IP addresses, which are not guaranteed
				// unique in separate VPCs.
				{
					Name:   aws.String(fmt.Sprintf("tag:%s", VolumeTag)),
					Values: []*string{aws.String(string(*volume))},
				},
			},
		})
		if err != nil {
			return nil, spi.NewError(spi.ErrUnknown, "Failed while looking up volume")
		}

		switch len(volumes.Volumes) {
		case 0:
			return nil, spi.NewError(spi.ErrBadInput, fmt.Sprintf("Volume %s does not exist", *volume))
		case 1:
			awsVolumeID = volumes.Volumes[0].VolumeId
		default:
			return nil, spi.NewError(spi.ErrBadInput, "Multiple volume matches found")
		}
	}

	reservation, err := p.Client.RunInstances(&request.RunInstancesInput)
	if err != nil {
		return nil, err
	}

	if reservation == nil || len(reservation.Instances) != 1 {
		return nil, spi.NewError(spi.ErrUnknown, "Unexpected AWS API response")
	}
	ec2Instance := reservation.Instances[0]

	id := (*instance.ID)(ec2Instance.InstanceId)

	err = p.tagInstance(ec2Instance, tags, request.Tags)
	if err != nil {
		return id, err
	}

	if awsVolumeID != nil {
		log.Infof("Waiting for instance %s to enter running state before attaching volume", *id)
		for {
			time.Sleep(10 * time.Second)

			inst, err := p.Client.DescribeInstances(&ec2.DescribeInstancesInput{
				InstanceIds: []*string{ec2Instance.InstanceId},
			})
			if err == nil {
				if *inst.Reservations[0].Instances[0].State.Name == ec2.InstanceStateNameRunning {
					break
				}
			} else if awsErr, ok := err.(awserr.Error); ok {
				if awsErr.Code() == "InvalidInstanceID.NotFound" {
					return id, nil
				}
			}

		}

		_, err := p.Client.AttachVolume(&ec2.AttachVolumeInput{
			InstanceId: ec2Instance.InstanceId,
			VolumeId:   awsVolumeID,
			Device:     aws.String("/dev/sdf"),
		})
		if err != nil {
			return id, err
		}
	}

	return id, nil
}

// Destroy terminates an existing instance.
func (p Provisioner) Destroy(id instance.ID) error {
	result, err := p.Client.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: []*string{aws.String(string(id))}})

	if err != nil {
		return err
	}

	if len(result.TerminatingInstances) != 1 {
		// There was no match for the instance ID.
		return spi.NewError(spi.ErrBadInput, "No matching instance")
	}

	return nil
}

func describeGroupRequest(tags map[string]string, nextToken *string) *ec2.DescribeInstancesInput {

	filters := []*ec2.Filter{
		{
			Name: aws.String("instance-state-name"),
			Values: []*string{
				aws.String("pending"),
				aws.String("running"),
			},
		},
	}
	for key, value := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(value)},
		})
	}

	return &ec2.DescribeInstancesInput{NextToken: nextToken, Filters: filters}
}

func (p Provisioner) describeInstances(tags map[string]string, nextToken *string) ([]instance.Description, error) {
	result, err := p.Client.DescribeInstances(describeGroupRequest(tags, nextToken))
	if err != nil {
		return nil, err
	}

	descriptions := []instance.Description{}
	for _, reservation := range result.Reservations {
		for _, ec2Instance := range reservation.Instances {
			tags := map[string]string{}
			if ec2Instance.Tags != nil {
				for _, tag := range ec2Instance.Tags {
					if tag.Key != nil && tag.Value != nil {
						tags[*tag.Key] = *tag.Value
					}
				}
			}

			descriptions = append(descriptions, instance.Description{
				ID:               instance.ID(*ec2Instance.InstanceId),
				PrivateIPAddress: *ec2Instance.PrivateIpAddress,
				Tags:             tags,
			})
		}
	}

	if result.NextToken != nil {
		// There are more pages of results.
		remainingPages, err := p.describeInstances(tags, result.NextToken)
		if err != nil {
			return nil, err
		}

		descriptions = append(descriptions, remainingPages...)
	}

	return descriptions, nil
}

// DescribeInstances implements instance.Provisioner.DescribeInstances.
func (p Provisioner) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	return p.describeInstances(tags, nil)
}

func (p Provisioner) describeInstance(id instance.ID) (*ec2.Instance, error) {
	result, err := p.Client.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(string(id))},
	})
	if err != nil {
		return nil, err
	}
	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, spi.NewError(spi.ErrBadInput, "Instance not found")
	}

	return result.Reservations[0].Instances[0], nil
}
