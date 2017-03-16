package instance

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

const (
	// VolumeTag is the AWS tag name used to associate unique identifiers (instance.VolumeID) with volumes.
	VolumeTag = "docker-infrakit-volume"

	// AttachmentEBSVolume is the type name used in instance.Attachment
	AttachmentEBSVolume = "ebs"
)

type awsInstancePlugin struct {
	client        ec2iface.EC2API
	namespaceTags map[string]string
}

type properties struct {
	Region   string
	Retries  int
	Instance *types.Any
}

// NewInstancePlugin creates a new plugin that creates instances in AWS EC2.
func NewInstancePlugin(client ec2iface.EC2API, namespaceTags map[string]string) instance.Plugin {
	return &awsInstancePlugin{client: client, namespaceTags: namespaceTags}
}

func (p awsInstancePlugin) tagInstance(
	instance *ec2.Instance,
	systemTags map[string]string,
	userTags map[string]string) error {

	ec2Tags := []*ec2.Tag{}

	keys, allTags := mergeTags(userTags, systemTags, p.namespaceTags)

	for _, k := range keys {
		key := k
		ec2Tags = append(ec2Tags, &ec2.Tag{Key: aws.String(key), Value: aws.String(allTags[key])})
	}

	_, err := p.client.CreateTags(&ec2.CreateTagsInput{Resources: []*string{instance.InstanceId}, Tags: ec2Tags})
	return err
}

// CreateInstanceRequest is the concrete provision request type.
type CreateInstanceRequest struct {
	Tags              map[string]string
	RunInstancesInput ec2.RunInstancesInput
}

// VendorInfo returns a vendor specific name and version
func (p awsInstancePlugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-instance-aws",
			Version: "0.3.0",
		},
		URL: "https://github.com/docker/infrakit.aws",
	}
}

// ExampleProperties returns the properties / config of this plugin
func (p awsInstancePlugin) ExampleProperties() *types.Any {
	example := CreateInstanceRequest{
		Tags: map[string]string{
			"tag1": "value1",
			"tag2": "value2",
		},
		RunInstancesInput: ec2.RunInstancesInput{
			BlockDeviceMappings: []*ec2.BlockDeviceMapping{
				{
					// The device name exposed to the instance (for example, /dev/sdh or xvdh).
					DeviceName: aws.String("/dev/sdh"),
				},
			},
			SecurityGroupIds: []*string{},
			SecurityGroups:   []*string{},
		},
	}

	any, err := types.AnyValue(example)
	if err != nil {
		panic(err)
	}
	return any
}

// Validate performs local checks to determine if the request is valid.
func (p awsInstancePlugin) Validate(req *types.Any) error {
	// TODO(wfarner): Implement
	return nil
}

// Label implements labeling the instances.
func (p awsInstancePlugin) Label(id instance.ID, labels map[string]string) error {

	output, err := p.client.DescribeTags(&ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("resource-id"),
				Values: []*string{aws.String(string(id))},
			},
		},
	})
	if err != nil {
		return err
	}

	allTags := map[string]string{}
	for _, t := range output.Tags {
		allTags[aws.StringValue(t.Key)] = aws.StringValue(t.Value)
	}

	_, merged := mergeTags(allTags, labels)

	for k := range merged {
		// filter out the special aws: key because it's reserved so leave them alone
		if strings.HasPrefix(k, "aws:") {
			delete(merged, k)
		}
	}

	return ec2CreateTags(p.client, id, merged)
}

// mergeTags merges multiple maps of tags, implementing 'last write wins' for colliding keys.
//
// Returns a sorted slice of all keys, and the map of merged tags.  Sorted keys are particularly useful to assist in
// preparing predictable output such as for tests.
func mergeTags(tagMaps ...map[string]string) ([]string, map[string]string) {

	keys := []string{}
	tags := map[string]string{}

	for _, tagMap := range tagMaps {
		for k, v := range tagMap {
			if _, exists := tags[k]; exists {
				log.Warnf("Ovewriting tag value for key %s", k)
			} else {
				keys = append(keys, k)
			}
			tags[k] = v
		}
	}

	sort.Strings(keys)

	return keys, tags
}

func (p awsInstancePlugin) findEBSVolumeAttachments(spec instance.Spec) ([]*string, error) {
	found := []*string{}

	// for querying volumes
	filterValues := []*string{}

	for _, attachment := range spec.Attachments {
		if attachment.Type == AttachmentEBSVolume {
			s := attachment.ID
			filterValues = append(filterValues, &s)
		}
	}

	if len(filterValues) == 0 {
		return found, nil // nothing
	}

	volumes, err := p.client.DescribeVolumes(&ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", VolumeTag)),
				Values: filterValues,
			},
		},
	})
	if err != nil {
		return nil, errors.New("Failed while looking up volume")
	}

	for _, volume := range volumes.Volumes {
		if p.hasNamespaceTags(volume.Tags) {
			found = append(found, volume.VolumeId)
		}
	}

	// TODO(chungers) -- not dealing with if only a subset is found.
	if len(found) != len(spec.Attachments) {
		return nil, fmt.Errorf(
			"Not all required volumes found to attach.  Wanted %s, found %s",
			spec.Attachments,
			volumes.Volumes)
	}

	return found, nil
}

func (p awsInstancePlugin) hasNamespaceTags(tags []*ec2.Tag) bool {
	matches := 0
	for k, v := range p.namespaceTags {
		for _, t := range tags {

			if t.Key == nil || t.Value == nil {
				continue
			}

			if *t.Key == k && *t.Value == v {
				matches++
				break
			}
		}
	}
	return matches == len(p.namespaceTags)
}

// Provision creates a new instance.
func (p awsInstancePlugin) Provision(spec instance.Spec) (*instance.ID, error) {

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

	if spec.LogicalID != nil {
		if len(request.RunInstancesInput.NetworkInterfaces) > 0 {
			request.RunInstancesInput.NetworkInterfaces[0].PrivateIpAddress = (*string)(spec.LogicalID)
		} else {
			request.RunInstancesInput.PrivateIpAddress = (*string)(spec.LogicalID)
		}
	}

	if spec.Init != "" {
		request.RunInstancesInput.UserData = aws.String(spec.Init)
	}

	if request.RunInstancesInput.UserData != nil {
		request.RunInstancesInput.UserData = aws.String(
			base64.StdEncoding.EncodeToString([]byte(*request.RunInstancesInput.UserData)))
	}

	reservation, err := p.client.RunInstances(&request.RunInstancesInput)
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

	// work with attachments
	awsVolumeIDs, err := p.findEBSVolumeAttachments(spec)
	if err != nil {
		return id, err
	}

	if len(awsVolumeIDs) > 0 {
		log.Infof("Waiting for instance %s to enter running state before attaching volume", *id)
		for {
			time.Sleep(10 * time.Second)

			inst, err := p.client.DescribeInstances(&ec2.DescribeInstancesInput{
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
			_, err := p.client.AttachVolume(&ec2.AttachVolumeInput{
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
func (p awsInstancePlugin) Destroy(id instance.ID) error {
	result, err := p.client.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: []*string{aws.String(string(id))}})

	if err != nil {
		return err
	}

	if len(result.TerminatingInstances) != 1 {
		return errors.New("No matching instance")
	}

	return nil
}

func describeGroupRequest(namespaceTags, tags map[string]string, nextToken *string) *ec2.DescribeInstancesInput {

	filters := []*ec2.Filter{
		{
			Name: aws.String("instance-state-name"),
			Values: []*string{
				aws.String("pending"),
				aws.String("running"),
			},
		},
	}

	keys, allTags := mergeTags(tags, namespaceTags)

	for _, key := range keys {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(allTags[key])},
		})
	}

	return &ec2.DescribeInstancesInput{NextToken: nextToken, Filters: filters}
}

func (p awsInstancePlugin) describeInstances(tags map[string]string, nextToken *string) ([]instance.Description, error) {

	result, err := p.client.DescribeInstances(describeGroupRequest(p.namespaceTags, tags, nextToken))
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
func (p awsInstancePlugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	return p.describeInstances(tags, nil)
}

func (p awsInstancePlugin) describeInstance(id instance.ID) (*ec2.Instance, error) {
	result, err := p.client.DescribeInstances(&ec2.DescribeInstancesInput{
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
