package compute

import (
	"fmt"
	"log"
	"strings"
)

// InstancesClient is a client for the Instance functions of the Compute API.
type InstancesClient struct {
	ResourceClient
}

// Instances obtains an InstancesClient which can be used to access to the
// Instance functions of the Compute API
func (c *AuthenticatedClient) Instances() *InstancesClient {
	return &InstancesClient{
		ResourceClient: ResourceClient{
			AuthenticatedClient: c,
			ResourceDescription: "instance",
			ContainerPath:       "/launchplan/",
			ResourceRootPath:    "/instance",
		}}
}

// LaunchPlanStorageAttachmentSpec defines a storage attachment to be created on launch
type LaunchPlanStorageAttachmentSpec struct {
	Index  int    `json:"index"`
	Volume string `json:"volume"`
}


// InstanceSpec defines an instance to be created.
type InstanceSpec struct {
	Shape      string                            `json:"shape"`
	ImageList  string                            `json:"imagelist"`
	Name       string                            `json:"name"`
	Label      string                            `json:"label"`
	Storage    []LaunchPlanStorageAttachmentSpec `json:"storage_attachments"`
	BootOrder  []int                             `json:"boot_order"`
	SSHKeys    []string                          `json:"sshkeys"`
	Attributes map[string]interface{}            `json:"attributes"`
}

// LaunchPlan defines a launch plan, used to launch instances with the supplied InstanceSpec(s)
type LaunchPlan struct {
	Instances []InstanceSpec `json:"instances"`
}

// InstanceInfo represents the Compute API's view of the state of an instance.
type InstanceInfo struct {
	ID          string                 `json:"id"`
	Shape       string                 `json:"shape"`
	ImageList   string                 `json:"imagelist"`
	Name        string                 `json:"name"`
	Label       string                 `json:"label"`
	BootOrder   []int                  `json:"boot_order"`
	SSHKeys     []string               `json:"sshkeys"`
	State       string                 `json:"state"`
	ErrorReason string                 `json:"error_reason"`
	IPAddress   string                 `json:"ip"`
	VCableID    string                 `json:"vcable_id"`
	Attributes  map[string]interface{} `json:"attributes"`
}

// GetInstanceName returns the InstanceName (name/id pair) of an instance for which we have some InstanceInfo.
func (i *InstanceInfo) GetInstanceName() *InstanceName {
	return &InstanceName{
		Name: i.Name,
		ID:   i.ID,
	}
}

// LaunchPlanCreatedResponse represents the API's response to a request to create a launch plan.
type LaunchPlanCreatedResponse struct {
	Instances []InstanceInfo `json:"instances"`
}

// InstanceName represents the name/id combination which uniquely identifies an instance.
type InstanceName struct {
	Name string
	ID   string
}

// String obtains a string representation of an InstanceName.
func (n *InstanceName) String() string {
	return fmt.Sprintf("%s/%s", n.Name, n.ID)
}

func InstanceNameFromString(instanceNameString string) *InstanceName {
	sections := strings.Split(instanceNameString, "/")
	name := strings.Join(sections[:len(sections) - 1], "/")
	id := sections[len(sections) - 1]
	return &InstanceName{
		Name: name,
		ID: id,
	}
}

// LaunchInstance creates and submits a LaunchPlan to launch a new instance.
func (c *InstancesClient) LaunchInstance(name, label, shape, imageList string, storageAttachments []LaunchPlanStorageAttachmentSpec, bootOrder []int, sshkeys []string, attributes map[string]interface{}) (*InstanceName, error) {
	qualifiedSSHKeys := []string{}
	for _, key := range sshkeys {
		qualifiedSSHKeys = append(qualifiedSSHKeys, c.getQualifiedName(key))
	}

	qualifiedStorageAttachements := []LaunchPlanStorageAttachmentSpec{}
	for _, attachment := range storageAttachments {
		qualifiedStorageAttachements = append(qualifiedStorageAttachements, LaunchPlanStorageAttachmentSpec{
			Index:   attachment.Index,
			Volume:  c.getQualifiedName(attachment.Volume),
		})
	}

	plan := LaunchPlan{Instances: []InstanceSpec{
		InstanceSpec{
			Name:       fmt.Sprintf("%s/%s", c.computeUserName(), name),
			Shape:      shape,
			ImageList:  imageList,
			Storage:    qualifiedStorageAttachements,
			BootOrder:  bootOrder,
			Label:      label,
			SSHKeys:    qualifiedSSHKeys,
			Attributes: attributes,
		},
	}}

	var responseBody LaunchPlanCreatedResponse
	if err := c.createResource(&plan, &responseBody); err != nil {
		return nil, err
	}

	if len(responseBody.Instances) == 0 {
		return nil, fmt.Errorf("No instance information returned: %#v", responseBody)
	}

	return &InstanceName{
		Name: name,
		ID:   responseBody.Instances[0].ID,
	}, nil
}

// WaitForInstanceRunning waits for an instance to be completely initialized and available.
func (c *InstancesClient) WaitForInstanceRunning(name *InstanceName, timeoutSeconds int) (*InstanceInfo, error) {
	var waitResult *InstanceInfo
	err := waitFor("instance to be ready", timeoutSeconds, func() (bool, error) {
		info, err := c.GetInstance(name)
		log.Printf("[DEBUG] Instance state: %#v", info)
		if err != nil {
			return false, err
		}
		if info.State == "error" {
			return false, fmt.Errorf("Error initializing instance: %s", info.ErrorReason)
		}
		if info.State == "running" {
			waitResult = info
			return true, nil
		}
		return false, nil
	})
	return waitResult, err
}

// GetInstance retrieves information about an instance.
func (c *InstancesClient) GetInstance(name *InstanceName) (*InstanceInfo, error) {
	var responseBody InstanceInfo
	if err := c.getResource(name.String(), &responseBody); err != nil {
		return nil, err
	}

	if responseBody.Name == "" {
		return nil, fmt.Errorf("Empty response body when requesting instance %s", name)
	}

	// Overwrite returned name/ID with known name/ID
	responseBody.Name = name.Name
	responseBody.ID = name.ID
	c.unqualify(&responseBody.VCableID)

	// Unqualify SSH Key names
	sshKeyNames := []string{}
	for _, sshKeyRef := range responseBody.SSHKeys {
		sshKeyNames = append(sshKeyNames, c.getUnqualifiedName(sshKeyRef))
	}
	responseBody.SSHKeys = sshKeyNames

	return &responseBody, nil
}

// DeleteInstance deletes an instance.
func (c *InstancesClient) DeleteInstance(name *InstanceName) error {
	return c.deleteResource(name.String())
}

// WaitForInstanceDeleted waits for an instance to be fully deleted.
func (c *InstancesClient) WaitForInstanceDeleted(name *InstanceName, timeoutSeconds int) error {
	return waitFor("instance to be deleted", timeoutSeconds, func() (bool, error) {
		var instanceInfo InstanceInfo
		if err := c.getResource(name.String(), &instanceInfo); err != nil {
			if WasNotFoundError(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
}
