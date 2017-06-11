package instance

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsVolumePlugin struct {
	client        ec2iface.EC2API
	namespaceTags map[string]string
}

// NewVolumePlugin returns a plugin.
func NewVolumePlugin(client ec2iface.EC2API, namespaceTags map[string]string) instance.Plugin {
	return &awsVolumePlugin{client: client, namespaceTags: namespaceTags}
}

type createVolumeRequest struct {
	CreateVolumeInput ec2.CreateVolumeInput
	Tags              map[string]string
}

func (p awsVolumePlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsVolumePlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createVolumeRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	output, err := p.client.CreateVolume(&request.CreateVolumeInput)
	if err != nil {
		return nil, fmt.Errorf("CreateVolume failed: %s", err)
	}
	id := instance.ID(*output.VolumeId)

	return &id, ec2CreateTags(p.client, id, request.Tags, spec.Tags, p.namespaceTags)
}

func (p awsVolumePlugin) Label(id instance.ID, labels map[string]string) error {
	return ec2CreateTags(p.client, id, labels)
}

func (p awsVolumePlugin) Destroy(id instance.ID) error {
	if _, err := p.client.DeleteVolume(&ec2.DeleteVolumeInput{VolumeId: (*string)(&id)}); err != nil {
		return fmt.Errorf("DeleteVolume failed: %s", err)
	}
	return nil
}

func (p awsVolumePlugin) DescribeInstances(labels map[string]string, properties bool) ([]instance.Description, error) {
	_, tags := mergeTags(labels, p.namespaceTags)

	filters := []*ec2.Filter{}
	for key, value := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(value)},
		})
	}

	output, err := p.client.DescribeVolumes(&ec2.DescribeVolumesInput{Filters: filters})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("DescribeVolumes failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, volume := range output.Volumes {
		tags := map[string]string{}
		for _, tag := range volume.Tags {
			if tag.Key != nil && tag.Value != nil {
				tags[*tag.Key] = *tag.Value
			}
		}
		descriptions = append(descriptions, instance.Description{ID: instance.ID(*volume.VolumeId), Tags: tags})
	}
	return descriptions, nil
}
