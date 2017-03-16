package instance

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
)

type awsRolePlugin struct {
	client        iamiface.IAMAPI
	namespaceTags map[string]string
}

// NewRolePlugin returns a plugin.
func NewRolePlugin(client iamiface.IAMAPI, namespaceTags map[string]string) instance.Plugin {
	return &awsRolePlugin{client: client, namespaceTags: namespaceTags}
}

type createRoleRequest struct {
	CreateRoleInput     iam.CreateRoleInput
	PutRolePolicyInputs []iam.PutRolePolicyInput
}

func (p awsRolePlugin) Validate(req *types.Any) error {
	return nil
}

func (p awsRolePlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	request := createRoleRequest{}
	if err := json.Unmarshal(*spec.Properties, &request); err != nil {
		return nil, fmt.Errorf("Invalid input formatting: %s", err)
	}

	roleName := newIamName(spec.Tags, p.namespaceTags)
	rolePath := newIamPath(spec.Tags, p.namespaceTags)

	request.CreateRoleInput.Path = aws.String(rolePath)
	request.CreateRoleInput.RoleName = aws.String(roleName)
	output, err := p.client.CreateRole(&request.CreateRoleInput)
	if err != nil {
		return nil, fmt.Errorf("CreateRole failed: %s", err)
	}
	id := instance.ID(*output.Role.Arn)

	for i, input := range request.PutRolePolicyInputs {
		input.RoleName = aws.String(roleName)
		input.PolicyName = aws.String(fmt.Sprintf("%s-%d", roleName, i))
		if _, err := p.client.PutRolePolicy(&input); err != nil {
			return &id, fmt.Errorf("PutRolePolicy failed: %s", err)
		}
	}

	return &id, nil
}

func (p awsRolePlugin) Label(id instance.ID, labels map[string]string) error {
	return nil
}

func (p awsRolePlugin) Destroy(id instance.ID) error {
	roleName := arnOrNameToName(string(id))

	output, err := p.client.ListRolePolicies(&iam.ListRolePoliciesInput{RoleName: &roleName})
	if err != nil {
		return fmt.Errorf("ListRolePolicies failed: %s", err)
	}

	for _, policyName := range output.PolicyNames {
		_, err := p.client.DeleteRolePolicy(&iam.DeleteRolePolicyInput{
			PolicyName: policyName,
			RoleName:   &roleName,
		})
		if err != nil {
			return fmt.Errorf("DeleteRolePolicy for %s failed: %s", policyName, err)
		}
	}

	_, err = p.client.DeleteRole(&iam.DeleteRoleInput{
		RoleName: aws.String(arnOrNameToName(string(id))),
	})
	if err != nil {
		return fmt.Errorf("DeleteRole failed: %s", err)
	}
	return nil
}

func (p awsRolePlugin) DescribeInstances(labels map[string]string) ([]instance.Description, error) {
	_, tags := mergeTags(labels, p.namespaceTags)
	path := newIamPath(tags)

	output, err := p.client.ListRoles(&iam.ListRolesInput{PathPrefix: aws.String(path)})
	if err != nil {
		return []instance.Description{}, fmt.Errorf("ListRoles failed: %s", err)
	}

	descriptions := []instance.Description{}
	for _, role := range output.Roles {
		descriptions = append(descriptions, instance.Description{ID: instance.ID(*role.Arn)})
	}
	return descriptions, nil
}
