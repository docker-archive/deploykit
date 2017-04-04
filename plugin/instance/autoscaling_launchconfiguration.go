package instance

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsLaunchConfigurationPlugin struct {
	client        autoscalingiface.AutoScalingAPI
	namespaceTags map[string]string
}

// NewLaunchConfigurationPlugin returns a plugin.
func NewLaunchConfigurationPlugin(client autoscalingiface.AutoScalingAPI, namespaceTags map[string]string) instance.Plugin {
	return &awsLaunchConfigurationPlugin{client: client, namespaceTags: namespaceTags}
}

type createLaunchConfigurationRequest struct {
	CreateLaunchConfigurationInput autoscaling.CreateLaunchConfigurationInput
}

func (p awsLaunchConfigurationPlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsLaunchConfigurationPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createLaunchConfigurationRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	if userData := request.CreateLaunchConfigurationInput.UserData; userData != nil {
		if _, err := base64.StdEncoding.DecodeString(*userData); err != nil {
			*userData = base64.StdEncoding.EncodeToString([]byte(*userData))
		}
	}

	name := newUnrestrictedName(spec.Tags, p.namespaceTags)

	request.CreateLaunchConfigurationInput.LaunchConfigurationName = aws.String(name)
	err := retry(30*time.Second, 500*time.Millisecond, func() error {
		_, err := p.client.CreateLaunchConfiguration(&request.CreateLaunchConfigurationInput)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("CreateLaunchConfiguration failed: %s", err)
	}
	id := instance.ID(name)

	return &id, nil
}

func (p awsLaunchConfigurationPlugin) Label(id instance.ID, labels map[string]string) error {
	return nil
}

func (p awsLaunchConfigurationPlugin) Destroy(id instance.ID) error {
	_, err := p.client.DeleteLaunchConfiguration(&autoscaling.DeleteLaunchConfigurationInput{
		LaunchConfigurationName: (*string)(&id),
	})
	if err != nil {
		return fmt.Errorf("DeleteLaunchConfiguration failed: %s", err)
	}
	return nil
}

func (p awsLaunchConfigurationPlugin) DescribeInstances(tags map[string]string, properties bool) ([]instance.Description, error) {
	name := newUnrestrictedName(tags, p.namespaceTags)

	output, err := p.client.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{&name},
	})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("DescribeLaunchConfigurations failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, launchConfiguration := range output.LaunchConfigurations {
		descriptions = append(descriptions, instance.Description{ID: instance.ID(*launchConfiguration.LaunchConfigurationName)})
	}
	return descriptions, nil
}
