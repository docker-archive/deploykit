package instance

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsInstanceProfilePlugin struct {
	client        iamiface.IAMAPI
	namespaceTags map[string]string
}

// NewInstanceProfilePlugin returns a plugin.
func NewInstanceProfilePlugin(client iamiface.IAMAPI, namespaceTags map[string]string) instance.Plugin {
	return &awsInstanceProfilePlugin{client: client, namespaceTags: namespaceTags}
}

type createInstanceProfileRequest struct {
	CreateInstanceProfileInput    iam.CreateInstanceProfileInput
	AddRoleToInstanceProfileInput *iam.AddRoleToInstanceProfileInput
}

func (p awsInstanceProfilePlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsInstanceProfilePlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	_, tags := mergeTags(spec.Tags, p.namespaceTags)
	path := newIamPath(tags)

	request := createInstanceProfileRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}
	request.CreateInstanceProfileInput.InstanceProfileName = aws.String(strings.Replace(strings.Trim(path, "/"), "/", ".", -1))
	request.CreateInstanceProfileInput.Path = aws.String(path)

	if _, err := p.client.CreateInstanceProfile(&request.CreateInstanceProfileInput); err != nil {
		if awsErr, ok := err.(awserr.Error); !(ok && awsErr.Code() == "EntityAlreadyExists") {
			return nil, fmt.Errorf("CreateInstanceProfile failed: %s", err)
		}
	}
	id := instance.ID(*request.CreateInstanceProfileInput.InstanceProfileName)

	if request.AddRoleToInstanceProfileInput != nil {
		// If AddRoleToInstanceProfileInput.RoleName is an ARN, set it to just the name.
		if roleName := request.AddRoleToInstanceProfileInput.RoleName; roleName != nil {
			*roleName = arnOrNameToName(*roleName)
		}

		request.AddRoleToInstanceProfileInput.InstanceProfileName = request.CreateInstanceProfileInput.InstanceProfileName
		if _, err := p.client.AddRoleToInstanceProfile(request.AddRoleToInstanceProfileInput); err != nil {
			return nil, fmt.Errorf("AddRoleToInstanceProfile failed: %s", err)
		}
	}

	return &id, nil
}

func (p awsInstanceProfilePlugin) Label(id instance.ID, labels map[string]string) error {
	return nil
}

func (p awsInstanceProfilePlugin) Destroy(id instance.ID) error {
	output, err := p.client.GetInstanceProfile(&iam.GetInstanceProfileInput{InstanceProfileName: (*string)(&id)})
	if err != nil {
		return fmt.Errorf("GetInstanceProfile failed: %s", err)
	}

	for _, r := range output.InstanceProfile.Roles {
		_, err := p.client.RemoveRoleFromInstanceProfile(&iam.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: (*string)(&id),
			RoleName:            r.RoleName,
		})
		if err != nil {
			return fmt.Errorf("RemoveRoleFromInstanceProfile failed: %s", err)
		}
	}

	_, err = p.client.DeleteInstanceProfile(&iam.DeleteInstanceProfileInput{InstanceProfileName: (*string)(&id)})
	if err != nil {
		return fmt.Errorf("DeleteInstanceProfile failed: %s", err)
	}
	return nil
}

func (p awsInstanceProfilePlugin) DescribeInstances(labels map[string]string, properties bool) ([]instance.Description, error) {
	_, tags := mergeTags(labels, p.namespaceTags)
	path := newIamPath(tags)

	output, err := p.client.ListInstanceProfiles(&iam.ListInstanceProfilesInput{PathPrefix: &path})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("ListInstanceProfiles failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, instanceProfile := range output.InstanceProfiles {
		descriptions = append(descriptions, instance.Description{ID: instance.ID(*instanceProfile.InstanceProfileName)})
	}
	return descriptions, nil
}
