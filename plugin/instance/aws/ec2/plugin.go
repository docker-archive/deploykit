package ec2

import (
	"encoding/base64"
	"encoding/json"
	"errors"
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
	util "github.com/docker/infrakit/plugin/instance/aws"
	"github.com/docker/infrakit/spi/instance"
	"sort"
	"time"
)

const (
	// VolumeTag is the AWS tag name used to associate unique identifiers (instance.VolumeID) with volumes.
	VolumeTag = "docker-infrakit-volume"
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

// CreateInstanceRequest is the concrete provision request type.
type CreateInstanceRequest struct {
	DiscoverDefaults  bool
	Tags              map[string]string
	RunInstancesInput ec2.RunInstancesInput
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
		WithLogger(GetLogger()).
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

// Validate performs local checks to determine if the request is valid.
func (p Provisioner) Validate(req json.RawMessage) error {
	// TODO(wfarner): Implement
	return nil
}

func setDiscoveredProperties(ri *ec2.RunInstancesInput) {
	if ri.SubnetId == nil {
		// No subnet specified... use the same subnet I am in.
		subnetID, err := util.MetadataSubnetID()
		if err == nil {
			ri.SubnetId = &subnetID
		}
	}

	if len(ri.SecurityGroups) == 0 && len(ri.SecurityGroupIds) == 0 {
		// No security groups specified... let's use the same security group as host
		sgs, err := util.MetadataSecurityGroupIDs()
		if err == nil {
			ids := []*string{}
			for _, id := range sgs {
				copy := id
				ids = append(ids, &copy)
			}
			ri.SecurityGroupIds = ids
		}
	}

	if ri.InstanceType == nil {
		t, err := util.GetMetadata(util.MetadataInstanceType)
		if err == nil {
			ri.InstanceType = &t
		}
	}

	if ri.ImageId == nil {
		ami, err := util.GetMetadata(util.MetadataAmiID)
		if err == nil {
			ri.ImageId = &ami
		}
	}
}

// Provision creates a new instance.
func (p Provisioner) Provision(spec instance.Spec) (*instance.ID, error) {

	if spec.Properties == nil {
		return nil, errors.New("Properties must be set")
	}

	request := CreateInstanceRequest{}
	err := json.Unmarshal(*spec.Properties, &request)
	if err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	request.RunInstancesInput.MinCount = aws.Int64(1)
	request.RunInstancesInput.MaxCount = aws.Int64(1)

	// This will default some fields to be identical to the instance running the plugin
	if request.DiscoverDefaults {
		setDiscoveredProperties(&request.RunInstancesInput)
	}

	if spec.LogicalID != nil {
		if len(request.RunInstancesInput.NetworkInterfaces) > 0 {
			request.RunInstancesInput.NetworkInterfaces[0].PrivateIpAddress = (*string)(spec.LogicalID)
		} else {
			request.RunInstancesInput.PrivateIpAddress = (*string)(spec.LogicalID)
		}
	}

	// TODO(chungers) - change this to merge...
	if spec.Init != "" {
		request.RunInstancesInput.UserData = aws.String(spec.Init)
	}

	if request.RunInstancesInput.UserData != nil {
		request.RunInstancesInput.UserData = aws.String(
			base64.StdEncoding.EncodeToString([]byte(*request.RunInstancesInput.UserData)))
	}

	awsVolumeIDs := []*string{}
	if spec.Attachments != nil && len(spec.Attachments) > 0 {
		filterValues := []*string{}
		for _, attachment := range spec.Attachments {
			s := string(attachment)
			filterValues = append(filterValues, &s)
		}

		volumes, err := p.Client.DescribeVolumes(&ec2.DescribeVolumesInput{
			Filters: []*ec2.Filter{
				// TODO(wfarner): Need a way to disambiguate between volumes associated with different
				// clusters.  Currently, volume IDs are private IP addresses, which are not guaranteed
				// unique in separate VPCs.
				{
					Name:   aws.String(fmt.Sprintf("tag:%s", VolumeTag)),
					Values: filterValues,
				},
			},
		})
		if err != nil {
			return nil, errors.New("Failed while looking up volume")
		}

		if len(volumes.Volumes) == len(spec.Attachments) {
			for _, volume := range volumes.Volumes {
				awsVolumeIDs = append(awsVolumeIDs, volume.VolumeId)
			}
		} else {
			return nil, fmt.Errorf(
				"Not all required volumes found to attach.  Wanted %s, found %s",
				spec.Attachments,
				volumes.Volumes)
		}
	}

	reservation, err := p.Client.RunInstances(&request.RunInstancesInput)
	if err != nil {
		return nil, err
	}

	if reservation == nil || len(reservation.Instances) != 1 {
		return nil, errors.New("Unexpected AWS API response")
	}
	ec2Instance := reservation.Instances[0]

	id := (*instance.ID)(ec2Instance.InstanceId)

	err = p.tagInstance(ec2Instance, spec.Tags, request.Tags)
	if err != nil {
		return id, err
	}

	if len(awsVolumeIDs) > 0 {
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

		for _, awsVolumeID := range awsVolumeIDs {
			_, err := p.Client.AttachVolume(&ec2.AttachVolumeInput{
				InstanceId: ec2Instance.InstanceId,
				VolumeId:   awsVolumeID,
				Device:     aws.String("/dev/sdf"),
			})
			if err != nil {
				return id, err
			}
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
		return errors.New("No matching instance")
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
				ID:        instance.ID(*ec2Instance.InstanceId),
				LogicalID: (*instance.LogicalID)(ec2Instance.PrivateIpAddress),
				Tags:      tags,
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
		return nil, errors.New("Instance not found")
	}

	return result.Reservations[0].Instances[0], nil
}
