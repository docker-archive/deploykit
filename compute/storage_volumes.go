package compute

import (
	"fmt"
)

// StorageVolumeClient is a client for the Storage Volume functions of the Compute API.
type StorageVolumeClient struct {
	ResourceClient
}

// StorageVolumes obtains a StorageVolumeClient which can be used to access to the
// Storage Volume functions of the Compute API
func (c *AuthenticatedClient) StorageVolumes() *StorageVolumeClient {
	return &StorageVolumeClient{
		ResourceClient: ResourceClient{
			AuthenticatedClient: c,
			ResourceDescription: "storage volume",
			ContainerPath:       "/storage/volume/",
			ResourceRootPath:    "/storage/volume",
		}}

}

// StorageVolumeSpec represents the body of an API request to create a new Storage Volume.
type StorageVolumeSpec struct {
	Bootable        bool     `json:"bootable,omitempty"`
	Description     string   `json:"description,omitempty"`
	ImageList       string   `json:"imagelist,omitempty"`
	ImageListEntry  int      `json:"imagelist_entry,omitempty"`
	Name            string   `json:"name"`
	Properties      []string `json:"properties,omitempty"`
	Size            string   `json:"size"`
	Snapshot        string   `json:"snapshot,omitempty"`
	SnapshotAccount string   `json:"snapshot_account,omitempty"`
	SnapshotID      string   `json:"snapshot_id,omitempty"`
	Tags            []string `json:"tags,omitempty"`
}

// NewStorageVolumeSpec creates a new StorageVolumeSpec.
func (c *StorageVolumeClient) NewStorageVolumeSpec(size string, properties []string, name string) *StorageVolumeSpec {
	return &StorageVolumeSpec{
		Size:       size,
		Properties: properties,
		Name:       c.getQualifiedName(name),
	}
}

// SetTags sets the tags on a StorageVolumeSpec.
func (s *StorageVolumeSpec) SetTags(tags []string) {
	s.Tags = tags
}

// SetDescription sets the description on a StorageVolumeSpec.
func (s *StorageVolumeSpec) SetDescription(description string) {
	s.Description = description
}

// SetBootableImage sets "bootable" on a StorageVolumeSpec to "true", and specifies the bootable image to use.
func (s *StorageVolumeSpec) SetBootableImage(imagelist string, imagelistentry int) {
	s.Bootable = true
	s.ImageList = imagelist
	s.ImageListEntry = imagelistentry
}

// SetSnapshot sets the snapshot and snapshot account of the StorageVolumeSpec.
func (s *StorageVolumeSpec) SetSnapshot(snapshot, snapshotAccount string) {
	s.Snapshot = snapshot
	s.SnapshotAccount = snapshotAccount
}

// SetSnapshotID sets the snapshot ID of the StorageVolumeSpec.
func (s *StorageVolumeSpec) SetSnapshotID(snapshotID string) {
	s.SnapshotID = snapshotID
}

// StorageVolumeInfo represents information retrieved from the service about a Storage Volume.
type StorageVolumeInfo struct {
	Managed         bool     `json:"managed,omitempty"`
	StatusTimestamp string   `json:"status_timestamp,omitempty"`
	SnapshotAccount string   `json:"snapshot_account,omitempty"`
	MachineImage    string   `json:"machineimage_name,omitempty"`
	SnapshotID      string   `json:"snapshot_id,omitempty"`
	ImageList       string   `json:"imagelist,omitempty"`
	WriteCache      bool     `json:"writecache,omitempty"`
	Size            string   `json:"size"`
	StoragePool     string   `json:"storage_pool,omitempty"`
	Shared          bool     `json:"shared,omitempty"`
	Status          string   `json:"status,omitempty"`
	Description     string   `json:"description,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Quota           string   `json:"quota,omitempty"`
	StatusDetail    string   `json:"status_detail,omitempty"`
	Properties      []string `json:"properties,omitempty"`
	Account         string   `json:"account,omitempty"`
	Name            string   `json:"name"`
	Bootable        bool     `json:"bootable,omitempty"`
	Hypervisor      string   `json:"hypervisor,omitempty"`
	URI             string   `json:"uri,omitempty"`
	ImageListEntry  int      `json:"imagelist_entry,omitempty"`
	Snapshot        string   `json:"snapshot,omitempty"`
}

func (c *StorageVolumeClient) getStorageVolumePath(name string) string {
	return c.getObjectPath("/storage/volume", name) + "/"
}

// CreateStorageVolume uses the given StorageVolumeSpec to create a new Storage Volume.
func (c *StorageVolumeClient) CreateStorageVolume(spec *StorageVolumeSpec) error {
	req, err := c.newAuthenticatedPostRequest("/storage/volume/", spec)
	if err != nil {
		return err
	}

	_, err = c.requestAndCheckStatus("create storage volume", req)
	return err
}

// WaitForStorageVolumeOnline waits until a new Storage Volume is online (i.e. has finished initialising or updating).
func (c *StorageVolumeClient) WaitForStorageVolumeOnline(name string, timeoutSeconds int) (*StorageVolumeInfo, error) {
	var waitResult *StorageVolumeInfo

	err := waitFor(
		fmt.Sprintf("storage volume %s to be online", c.getQualifiedName(name)),
		timeoutSeconds,
		func() (bool, error) {
			result, err := c.GetStorageVolume(name)

			if err != nil {
				return false, err
			}

			if len(result.Result) > 0 {
				waitResult = &result.Result[0]
				if waitResult.Status == "Online" {
					return true, nil
				}
			}

			return false, nil
		})

	return waitResult, err
}

// StorageVolumeResult represents the body of a response to a query for Storage Volume information.
type StorageVolumeResult struct {
	Result []StorageVolumeInfo `json:"result"`
}

var emptyResult = StorageVolumeResult{Result: []StorageVolumeInfo{}}

// GetStorageVolume gets Storage Volume information for the specified storage volume.
func (c *StorageVolumeClient) GetStorageVolume(name string) (*StorageVolumeResult, error) {
	req, err := c.newAuthenticatedGetRequest(c.getStorageVolumePath(name))
	if err != nil {
		return &emptyResult, err
	}

	rsp, err := c.requestAndCheckStatus("get storage volume", req)
	if err != nil {
		return &emptyResult, err
	}

	var result StorageVolumeResult
	err = unmarshalResponseBody(rsp, &result)
	if err != nil {
		return &emptyResult, err
	}

	if len(result.Result) > 0 {
		c.unqualify(&result.Result[0].Name)
	}
	return &result, nil
}

// DeleteStorageVolume deletes the specified storage volume.
func (c *StorageVolumeClient) DeleteStorageVolume(name string) error {
	req, err := c.newAuthenticatedDeleteRequest(c.getStorageVolumePath(name))
	if err != nil {
		return err
	}

	_, err = c.requestAndCheckStatus("delete storage volume", req)
	return err
}

// WaitForStorageVolumeDeleted waits until the specified storage volume has been deleted.
func (c *StorageVolumeClient) WaitForStorageVolumeDeleted(name string, timeoutSeconds int) error {
	return waitFor(
		fmt.Sprintf("storage volume %s to be deleted", c.getQualifiedName(name)),
		timeoutSeconds,
		func() (bool, error) {
			result, err := c.GetStorageVolume(name)
			if err != nil {
				return false, err
			}

			return len(result.Result) == 0, nil
		})
}

// UpdateStorageVolume updates the specified storage volume, optionally modifying size, description and tags.
func (c *StorageVolumeClient) UpdateStorageVolume(name, size, description string, tags []string) error {
	result, err := c.GetStorageVolume(name)
	if err != nil {
		return err
	}

	if len(result.Result) == 0 {
		return fmt.Errorf("No storage volume information found for volume %s", c.getQualifiedName(name))
	}

	volumeInfo := result.Result[0]
	volumeInfo.Size = size
	volumeInfo.Tags = tags
	volumeInfo.Description = description

	req, err := c.newAuthenticatedPutRequest(c.getStorageVolumePath(name), volumeInfo)
	if err != nil {
		return err
	}

	_, err = c.requestAndCheckStatus("update storage volume", req)
	return err
}
