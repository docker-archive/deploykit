package instance

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsSpotInstancePlugin struct {
	client        ec2iface.EC2API
	namespaceTags map[string]string
}

// NewSpotInstancePlugin creates a new plugin that creates spot instances in AWS EC2.
func NewSpotInstancePlugin(client ec2iface.EC2API, namespaceTags map[string]string) instance.Plugin {
	return &awsSpotInstancePlugin{client: client, namespaceTags: namespaceTags}
}

func (p awsSpotInstancePlugin) tagRequest(
	request *ec2.SpotInstanceRequest,
	systemTags map[string]string,
	userTags map[string]string) error {

	ec2Tags := []*ec2.Tag{}
	keys, allTags := mergeTags(userTags, systemTags, p.namespaceTags)
	for _, k := range keys {
		key := k
		ec2Tags = append(ec2Tags, &ec2.Tag{Key: aws.String(key), Value: aws.String(allTags[key])})
	}
	_, err := p.client.CreateTags(&ec2.CreateTagsInput{Resources: []*string{request.SpotInstanceRequestId}, Tags: ec2Tags})
	return err
}

// CreateSpotInstanceRequest is the concrete provision request type.
type CreateSpotInstanceRequest struct {
	Tags                      map[string]string
	RequestSpotInstancesInput ec2.RequestSpotInstancesInput
	AttachVolumeInputs        []ec2.AttachVolumeInput
}

// VendorInfo returns a vendor specific name and version
func (p awsSpotInstancePlugin) VendorInfo() *spi.VendorInfo {
	return &spi.VendorInfo{
		InterfaceSpec: spi.InterfaceSpec{
			Name:    "infrakit-spot-instance-aws",
			Version: "0.3.0",
		},
		URL: "https://github.com/docker/infrakit/pkg/provider/aws",
	}
}

// ExampleProperties returns the properties / config of this plugin
func (p awsSpotInstancePlugin) ExampleProperties() *types.Any {
	example := CreateSpotInstanceRequest{
		Tags: map[string]string{
			"tag1": "value1",
			"tag2": "value2",
		},
		RequestSpotInstancesInput: ec2.RequestSpotInstancesInput{
			LaunchSpecification: &ec2.RequestSpotLaunchSpecification{
				BlockDeviceMappings: []*ec2.BlockDeviceMapping{
					{
						// The device name exposed to the instance (for example, /dev/sdh or xvdh).
						DeviceName: aws.String("/dev/sdh"),
					},
				},
				SecurityGroupIds: []*string{},
				SecurityGroups:   []*string{},
			},
		},
	}

	any, err := types.AnyValue(example)
	if err != nil {
		panic(err)
	}
	return any
}

// Validate performs local checks to determine if the request is valid.
func (p awsSpotInstancePlugin) Validate(req *types.Any) error {
	request := CreateSpotInstanceRequest{}
	err := req.Decode(&request)
	if err != nil {
		return fmt.Errorf("Invalid input formatting: %s", err)
	}
	request.RequestSpotInstancesInput.InstanceCount = aws.Int64(1)
	request.RequestSpotInstancesInput.DryRun = aws.Bool(true)
	_, err = p.client.RequestSpotInstances(&request.RequestSpotInstancesInput)
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case "DryRunOperation":
			return nil
		default:
			return err
		}
	} else {
		return err
	}
}

// Label implements labeling the instances.
func (p awsSpotInstancePlugin) Label(id instance.ID, labels map[string]string) error {

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

func (p awsSpotInstancePlugin) findEBSVolumeAttachments(spec instance.Spec) ([]*string, error) {
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

	// TODO(yujioshima) -- not dealing with if only a subset is found.
	if len(found) != len(spec.Attachments) {
		return nil, fmt.Errorf(
			"Not all required volumes found to attach.  Wanted %s, found %s",
			spec.Attachments,
			volumes.Volumes)
	}

	return found, nil
}

func (p awsSpotInstancePlugin) hasNamespaceTags(tags []*ec2.Tag) bool {
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
func (p awsSpotInstancePlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	if spec.Properties == nil {
		return nil, errors.New("Properties must be set")
	}
	request := CreateSpotInstanceRequest{}
	err := json.Unmarshal(*spec.Properties, &request)
	if err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}
	request.RequestSpotInstancesInput.InstanceCount = aws.Int64(1)

	if spec.LogicalID != nil {
		if len(request.RequestSpotInstancesInput.LaunchSpecification.NetworkInterfaces) > 0 {
			request.RequestSpotInstancesInput.LaunchSpecification.NetworkInterfaces[0].PrivateIpAddress = (*string)(spec.LogicalID)
		} else {
			request.RequestSpotInstancesInput.LaunchSpecification.SetNetworkInterfaces([]*ec2.InstanceNetworkInterfaceSpecification{
				{
					PrivateIpAddress: (*string)(spec.LogicalID),
				},
			})
		}
	}
	if spec.Init != "" {
		request.RequestSpotInstancesInput.LaunchSpecification.UserData = aws.String(spec.Init)
	}
	if request.RequestSpotInstancesInput.LaunchSpecification.UserData != nil {
		request.RequestSpotInstancesInput.LaunchSpecification.UserData = aws.String(
			base64.StdEncoding.EncodeToString([]byte(*request.RequestSpotInstancesInput.LaunchSpecification.UserData)))
	}
	retrequest, err := p.client.RequestSpotInstances(&request.RequestSpotInstancesInput)
	if err != nil {
		return nil, err
	}
	spotRequest := retrequest.SpotInstanceRequests[0]
	if spotRequest == nil || *spotRequest.State != "open" {
		return nil, errors.New("Unexpected AWS API response")
	}
	id := (*instance.ID)(spotRequest.SpotInstanceRequestId)
	err = p.tagRequest(spotRequest, spec.Tags, request.Tags)
	if err != nil {
		return id, err
	}

	// wait until request evaliated
	discinput := &ec2.DescribeSpotInstanceRequestsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("state"),
				Values: []*string{
					aws.String("open"),
					aws.String("active"),
				},
			},
		},
	}
	findTarget := false
	for {
		time.Sleep(5 * time.Second)
		requests, _ := p.client.DescribeSpotInstanceRequests(discinput)
		for _, r := range requests.SpotInstanceRequests {
			if *r.SpotInstanceRequestId == *spotRequest.SpotInstanceRequestId {
				findTarget = true
				break
			}
		}
		if findTarget {
			break
		}
	}

	// work with attachments
	awsVolumeIDs, err := p.findEBSVolumeAttachments(spec)
	if err != nil {
		return id, err
	}

	if len(awsVolumeIDs) > 0 {
		log.Info("Waiting for instance to enter running state before attaching volume", "instance", *id)
		err = p.client.WaitUntilSpotInstanceRequestFulfilled(&ec2.DescribeSpotInstanceRequestsInput{
			SpotInstanceRequestIds: []*string{
				spotRequest.SpotInstanceRequestId,
			},
		})
		if err != nil {
			return id, err
		}
		req, err := p.client.DescribeSpotInstanceRequests(&ec2.DescribeSpotInstanceRequestsInput{
			SpotInstanceRequestIds: []*string{
				aws.String(string(*id)),
			},
		})
		if err != nil {
			return id, err
		}

		for {
			time.Sleep(10 * time.Second)

			inst, err := p.client.DescribeInstances(&ec2.DescribeInstancesInput{
				InstanceIds: []*string{req.SpotInstanceRequests[0].InstanceId},
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
				InstanceId: req.SpotInstanceRequests[0].InstanceId,
				VolumeId:   awsVolumeID,
				Device:     aws.String("/dev/sdf"),
			})
			if err != nil {
				return id, err
			}
		}

		for _, attachVolumeInput := range request.AttachVolumeInputs {
			attachVolumeInput.InstanceId = req.SpotInstanceRequests[0].InstanceId
			err := retry(30*time.Second, 500*time.Millisecond, func() error {
				_, err := p.client.AttachVolume(&attachVolumeInput)
				return err
			})
			if err != nil {
				return id, fmt.Errorf("AttachVolume failed: %s", err)
			}
		}
	}
	return id, nil
}

// Destroy terminates an existing instance.
func (p awsSpotInstancePlugin) Destroy(id instance.ID, ctx instance.Context) error {
	input := &ec2.DescribeSpotInstanceRequestsInput{
		SpotInstanceRequestIds: []*string{
			aws.String(string(id)),
		},
	}
	result, err := p.client.DescribeSpotInstanceRequests(input)
	if err != nil {
		return err
	}
	if result.SpotInstanceRequests[0].InstanceId != nil {
		input := &ec2.TerminateInstancesInput{
			InstanceIds: []*string{
				result.SpotInstanceRequests[0].InstanceId,
			},
		}
		_, err := p.client.TerminateInstances(input)
		if err != nil {
			return err
		}
	}
	cancelreq := &ec2.CancelSpotInstanceRequestsInput{
		SpotInstanceRequestIds: []*string{
			aws.String(string(id)),
		},
	}
	_, err = p.client.CancelSpotInstanceRequests(cancelreq)
	if err != nil {
		return err
	}
	return nil
}

func (p awsSpotInstancePlugin) describeGroupRequest(namespaceTags, tags map[string]string) *ec2.DescribeSpotInstanceRequestsInput {
	filters := []*ec2.Filter{
		{
			Name: aws.String("state"),
			Values: []*string{
				aws.String("open"),
				aws.String("active"),
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
	return &ec2.DescribeSpotInstanceRequestsInput{Filters: filters}
}
func (p awsSpotInstancePlugin) getEC2Instance(id *string) (*ec2.Instance, error) {
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			id,
		},
	}
	result, err := p.client.DescribeInstances(input)
	if err != nil {
		return nil, err
	}
	for _, request := range result.Reservations {
		for _, ec2Instance := range request.Instances {
			copy := *ec2Instance
			return &copy, nil
		}
	}
	return nil, nil
}
func (p awsSpotInstancePlugin) describeRequests(tags map[string]string, properties bool) ([]instance.Description, error) {

	result, err := p.client.DescribeSpotInstanceRequests(p.describeGroupRequest(p.namespaceTags, tags))
	if err != nil {
		return nil, err
	}
	descriptions := []instance.Description{}
	for _, request := range result.SpotInstanceRequests {
		tags := map[string]string{}
		if request.Tags != nil {
			for _, tag := range request.Tags {
				if tag.Key != nil && tag.Value != nil {
					tags[*tag.Key] = *tag.Value
				}
			}
		}
		var lID *string
		var ec2Instance *ec2.Instance

		if request.InstanceId != nil {
			ec2Instance, err = p.getEC2Instance(request.InstanceId)
			if err != nil {
				return nil, err
			}
			if ec2Instance != nil {
				lID = ec2Instance.PrivateIpAddress
			}
		}

		var state *types.Any
		if properties {

			type desc struct {
				Request  *ec2.SpotInstanceRequest
				Instance *ec2.Instance
			}

			if v, err := types.AnyValue(desc{
				Request:  request,
				Instance: ec2Instance,
			}); err == nil {
				state = v
			} else {
				log.Warn("cannot encode ec2Instance", "err", err)
			}
		}

		descriptions = append(descriptions, instance.Description{
			ID:         instance.ID(*request.SpotInstanceRequestId),
			LogicalID:  (*instance.LogicalID)(lID),
			Tags:       tags,
			Properties: state,
		})
	}
	return descriptions, nil
}

// DescribeInstances implements instance.Provisioner.DescribeInstances.
func (p awsSpotInstancePlugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	return p.describeRequests(tags, properties)
}
