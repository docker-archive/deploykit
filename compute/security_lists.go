package compute

// SecurityListsClient is a client for the Security List functions of the Compute API.
type SecurityListsClient struct {
	ResourceClient
}

// SecurityLists obtains a SecurityListsClient which can be used to access to the
// Security List functions of the Compute API
func (c *AuthenticatedClient) SecurityLists() *SecurityListsClient {
	return &SecurityListsClient{
		ResourceClient: ResourceClient{
			AuthenticatedClient: c,
			ResourceDescription: "security list",
			ContainerPath:       "/seclist/",
			ResourceRootPath:    "/seclist",
		}}
}

// SecurityListSpec defines a security list to be created.
type SecurityListSpec struct {
	Name               string `json:"name"`
	Policy             string `json:"policy"`
	OutboundCIDRPolicy string `json:"outbound_cidr_policy"`
}

// SecurityListInfo describes an existing security list.
type SecurityListInfo struct {
	Account            string `json:"account"`
	Name               string `json:"name"`
	Policy             string `json:"policy"`
	OutboundCIDRPolicy string `json:"outbound_cidr_policy"`
	URI                string `json:"uri"`
}

func (c *SecurityListsClient) success(listInfo *SecurityListInfo) (*SecurityListInfo, error) {
	c.unqualify(&listInfo.Name)
	return listInfo, nil
}

// CreateSecurityList creates a new security list with the given name, policy and outbound CIDR policy.
func (c *SecurityListsClient) CreateSecurityList(name, policy, outboundCIDRPolicy string) (*SecurityListInfo, error) {
	spec := SecurityListSpec{
		Name:               c.getQualifiedName(name),
		Policy:             policy,
		OutboundCIDRPolicy: outboundCIDRPolicy,
	}

	var listInfo SecurityListInfo
	if err := c.createResource(&spec, &listInfo); err != nil {
		return nil, err
	}

	return c.success(&listInfo)
}

// GetSecurityList retrieves the security list with the given name.
func (c *SecurityListsClient) GetSecurityList(name string) (*SecurityListInfo, error) {
	var listInfo SecurityListInfo
	if err := c.getResource(name, &listInfo); err != nil {
		return nil, err
	}

	return c.success(&listInfo)
}

// UpdateSecurityList updates the policy and outbound CIDR pol
func (c *SecurityListsClient) UpdateSecurityList(name, policy, outboundCIDRPolicy string) (*SecurityListInfo, error) {
	spec := SecurityListSpec{
		Name:               c.getQualifiedName(name),
		Policy:             policy,
		OutboundCIDRPolicy: outboundCIDRPolicy,
	}
	var listInfo SecurityListInfo
	if err := c.updateResource(name, &spec, &listInfo); err != nil {
		return nil, err
	}

	return c.success(&listInfo)
}

// DeleteSecurityList deletes the security list with the given name.
func (c *SecurityListsClient) DeleteSecurityList(name string) error {
	return c.deleteResource(name)
}
