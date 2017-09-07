package compute

// SecurityIPListsClient is a client for the Security IP List functions of the Compute API.
type SecurityIPListsClient struct {
	ResourceClient
}

// SecurityIPLists obtains a SecurityIPListsClient which can be used to access to the
// Security IP List functions of the Compute API
func (c *AuthenticatedClient) SecurityIPLists() *SecurityIPListsClient {
	return &SecurityIPListsClient{
		ResourceClient: ResourceClient{
			AuthenticatedClient: c,
			ResourceDescription: "security ip list",
			ContainerPath:       "/seciplist/",
			ResourceRootPath:    "/seciplist",
		}}
}

// SecurityIPListSpec defines a security IP list to be created.
type SecurityIPListSpec struct {
	Name         string   `json:"name"`
	SecIPEntries []string `json:"secipentries"`
}

// SecurityIPListInfo describes an existing security IP list.
type SecurityIPListInfo struct {
	Name         string   `json:"name"`
	SecIPEntries []string `json:"secipentries"`
	URI          string   `json:"uri"`
}

func (c *SecurityIPListsClient) success(listInfo *SecurityIPListInfo) (*SecurityIPListInfo, error) {
	c.unqualify(&listInfo.Name)
	return listInfo, nil
}

// CreateSecurityIPList creates a security IP list with the given name and entries.
func (c *SecurityIPListsClient) CreateSecurityIPList(name string, entries []string) (*SecurityIPListInfo, error) {
	spec := SecurityIPListSpec{
		Name:         c.getQualifiedName(name),
		SecIPEntries: entries,
	}

	var listInfo SecurityIPListInfo
	if err := c.createResource(&spec, &listInfo); err != nil {
		return nil, err
	}

	return c.success(&listInfo)
}

// GetSecurityIPList gets the security IP list with the given name.
func (c *SecurityIPListsClient) GetSecurityIPList(name string) (*SecurityIPListInfo, error) {
	var listInfo SecurityIPListInfo
	if err := c.getResource(name, &listInfo); err != nil {
		return nil, err
	}

	return c.success(&listInfo)
}

// UpdateSecurityIPList modifies the entries in the security IP list with the given name.
func (c *SecurityIPListsClient) UpdateSecurityIPList(name string, entries []string) (*SecurityIPListInfo, error) {
	spec := SecurityIPListSpec{
		Name:         c.getQualifiedName(name),
		SecIPEntries: entries,
	}

	var listInfo SecurityIPListInfo
	if err := c.updateResource(name, &spec, &listInfo); err != nil {
		return nil, err
	}

	return c.success(&listInfo)
}

// DeleteSecurityIPList deletes the security IP list with the given name.
func (c *SecurityIPListsClient) DeleteSecurityIPList(name string) error {
	return c.deleteResource(name)
}
