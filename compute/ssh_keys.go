package compute

import ()

// SSHKeysClient is a client for the SSH key functions of the Compute API.
type SSHKeysClient struct {
	ResourceClient
}

// SSHKeys obtains an SSHKeysClient which can be used to access to the
// SSH key functions of the Compute API
func (c *AuthenticatedClient) SSHKeys() *SSHKeysClient {
	return &SSHKeysClient{
		ResourceClient: ResourceClient{
			AuthenticatedClient: c,
			ResourceDescription: "SSH key",
			ContainerPath:       "/sshkey/",
			ResourceRootPath:    "/sshkey",
		}}
}

// SSHKeySpec defines an SSH key to be created.
type SSHKeySpec struct {
	Name    string `json:"name"`
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

// SSHKeyInfo describes an existing SSH key.
type SSHKeyInfo struct {
	Name    string `json:"name"`
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
	URI     string `json:"uri"`
}

func (c *SSHKeysClient) success(keyInfo *SSHKeyInfo) (*SSHKeyInfo, error) {
	c.unqualify(&keyInfo.Name)
	return keyInfo, nil
}

// CreateSSHKey creates a new SSH key with the given name, key and enabled flag.
func (c *SSHKeysClient) CreateSSHKey(name, key string, enabled bool) (*SSHKeyInfo, error) {
	spec := SSHKeySpec{
		Name:    c.getQualifiedName(name),
		Key:     key,
		Enabled: enabled,
	}

	var keyInfo SSHKeyInfo
	if err := c.createResource(&spec, &keyInfo); err != nil {
		return nil, err
	}

	return c.success(&keyInfo)
}

// GetSSHKey retrieves the SSH key with the given name.
func (c *SSHKeysClient) GetSSHKey(name string) (*SSHKeyInfo, error) {
	var keyInfo SSHKeyInfo
	if err := c.getResource(name, &keyInfo); err != nil {
		return nil, err
	}

	return c.success(&keyInfo)
}

// UpdateSSHKey updates the key and enabled flag of the SSH key with the given name.
func (c *SSHKeysClient) UpdateSSHKey(name, key string, enabled bool) (*SSHKeyInfo, error) {
	spec := SSHKeySpec{
		Name:    c.getQualifiedName(name),
		Key:     key,
		Enabled: enabled,
	}

	var keyInfo SSHKeyInfo
	if err := c.updateResource(name, &spec, &keyInfo); err != nil {
		return nil, err
	}

	return c.success(&keyInfo)
}

// DeleteSSHKey deletes the SSH key with the given name.
func (c *SSHKeysClient) DeleteSSHKey(name string) error {
	return c.deleteResource(name)
}
