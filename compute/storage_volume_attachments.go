package compute

import (
	"fmt"
	"log"
)

// StorageAttachmentsClient is a client for the Storage Attachment functions of the Compute API.
type StorageAttachmentsClient struct {
	ResourceClient
}

// StorageAttachments obtains a StorageAttachmentsClient which can be used to access to the
// Storage Attachment functions of the Compute API
func (c *AuthenticatedClient) StorageAttachments() *StorageAttachmentsClient {
	return &StorageAttachmentsClient{
		ResourceClient: ResourceClient{
			AuthenticatedClient: c,
			ResourceDescription: "storage volume attachment",
			ContainerPath:       "/storage/attachment/",
			ResourceRootPath:    "/storage/attachment",
		}}
}

// StorageAttachmentSpec defines a storage attachment to be created.
type StorageAttachmentSpec struct {
	Index             int    `json:"index"`
	InstanceName      string `json:"instance_name"`
	StorageVolumeName string `json:"storage_volume_name"`
}

// StorageAttachmentInfo describes an existing storage attachment.
type StorageAttachmentInfo struct {
	Name              string `json:"name"`
	Index             int    `json:"index"`
	InstanceName      string `json:"instance_name"`
	StorageVolumeName string `json:"storage_volume_name"`
	State             string `json:"state"`
}

// StorageAttachmentList is a collection of storage attachments attached to a specific instance.
type StorageAttachmentList struct {
	Result []StorageAttachmentInfo `json:"result"`
}

func (c *StorageAttachmentsClient) success(attachmentInfo *StorageAttachmentInfo) (*StorageAttachmentInfo, error) {
	c.unqualify(&attachmentInfo.Name, &attachmentInfo.InstanceName, &attachmentInfo.StorageVolumeName)
	return attachmentInfo, nil
}

// CreateStorageAttachment creates a storage attachment attaching the given volume to the given instance at the given index.
func (c *StorageAttachmentsClient) CreateStorageAttachment(index int, instanceName *InstanceName, storageVolumeName string) (*StorageAttachmentInfo, error) {
	spec := StorageAttachmentSpec{
		Index:             index,
		InstanceName:      c.getQualifiedName(instanceName.String()),
		StorageVolumeName: c.getQualifiedName(storageVolumeName),
	}

	var attachmentInfo StorageAttachmentInfo
	if err := c.createResource(&spec, &attachmentInfo); err != nil {
		return nil, err
	}

	return c.success(&attachmentInfo)
}

// GetStorageAttachment retrieves the storage attachment with the given name.
func (c *StorageAttachmentsClient) GetStorageAttachment(name string) (*StorageAttachmentInfo, error) {
	var attachmentInfo StorageAttachmentInfo
	if err := c.getResource(name, &attachmentInfo); err != nil {
		return nil, err
	}

	return c.success(&attachmentInfo)
}

// WaitForStorageAttachmentCreated waits for the storage attachment with the given name to be fully attached, or times out.
func (c *StorageAttachmentsClient) WaitForStorageAttachmentCreated(name string, timeoutSeconds int) error {
	return waitFor("storage attachment to be attached", timeoutSeconds, func() (bool, error) {
		info, err := c.GetStorageAttachment(name)
		if err != nil {
			return false, err
		}
		if info.State == "attached" {
			return true, nil
		}
		return false, nil
	})
}


// WaitForStorageAttachmentDeleted waits for the storage attachment with the given name to be fully deleted, or times out.
func (c *StorageAttachmentsClient) WaitForStorageAttachmentDeleted(name string, timeoutSeconds int) error {
	return waitFor("storage attachment to be deleted", timeoutSeconds, func() (bool, error) {
		_, err := c.GetStorageAttachment(name)
		if err != nil {
			if WasNotFoundError(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
}

// GetStorageAttachmentsForInstance retrieves all of the storage attachments for the given instance.
func (c *StorageAttachmentsClient) GetStorageAttachmentsForInstance(name *InstanceName) (*[]StorageAttachmentInfo, error) {
	return c.getStorageAttachments(
		fmt.Sprintf("instance_name=%s", c.getQualifiedName(name.String())),
		"instance",
	)
}

// GetStorageAttachmentsForInstance retrieves all of the storage attachments for the given volume.
func (c *StorageAttachmentsClient) GetStorageAttachmentsForVolume(name string) (*[]StorageAttachmentInfo, error) {
	return c.getStorageAttachments(
		fmt.Sprintf("storage_volume_name=%s", c.getQualifiedName(name)),
		"volume",
	)
}

func (c *StorageAttachmentsClient) getStorageAttachments(query string, description string) (*[]StorageAttachmentInfo, error) {
	queryPath := fmt.Sprintf("/storage/attachment%s/?state=attached&%s",
		c.computeUserName(),
		query)
	log.Printf("[DEBUG] Querying for storage attachments: %s", queryPath)
	req, err := c.newAuthenticatedGetRequest(queryPath)

	if err != nil {
		return nil, err
	}

	rsp, err := c.requestAndCheckStatus(fmt.Sprintf("get storage attachments for %s", description), req)
	if err != nil {
		return nil, err
	}

	var attachmentList StorageAttachmentList
	if err = unmarshalResponseBody(rsp, &attachmentList); err != nil {
		return nil, err
	}

	attachments := make([]StorageAttachmentInfo, len(attachmentList.Result))
	for index, attachment := range attachmentList.Result {
		c.unqualify(&attachment.Name, &attachment.InstanceName, &attachment.StorageVolumeName)
		attachments[index] = attachment
	}
	return &attachments, nil
}

// DeleteStorageAttachment deletes the storage attachment with the given name.
func (c *StorageAttachmentsClient) DeleteStorageAttachment(name string) error {
	return c.deleteResource(name)
}
