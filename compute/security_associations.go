package compute

// SecurityAssociationsClient is a client for the Security Association functions of the Compute API.
type SecurityAssociationsClient struct {
	ResourceClient
}

// SecurityAssociations obtains a SecurityAssociationsClient which can be used to access to the
// Security Association functions of the Compute API
func (c *AuthenticatedClient) SecurityAssociations() *SecurityAssociationsClient {
	return &SecurityAssociationsClient{
		ResourceClient: ResourceClient{
			AuthenticatedClient: c,
			ResourceDescription: "security association",
			ContainerPath:       "/secassociation/",
			ResourceRootPath:    "/secassociation",
		}}
}

// SecurityAssociationSpec defines a security association to be created.
type SecurityAssociationSpec struct {
	VCable  string `json:"vcable"`
	SecList string `json:"seclist"`
}

// SecurityAssociationInfo describes an existing security association.
type SecurityAssociationInfo struct {
	Name    string `json:"name"`
	VCable  string `json:"vcable"`
	SecList string `json:"seclist"`
	URI     string `json:"uri"`
}

func (c *SecurityAssociationsClient) success(assocInfo *SecurityAssociationInfo) (*SecurityAssociationInfo, error) {
	c.unqualify(&assocInfo.Name, &assocInfo.SecList, &assocInfo.VCable)
	return assocInfo, nil
}

// CreateSecurityAssociation creates a security association between the given VCable and security list.
func (c *SecurityAssociationsClient) CreateSecurityAssociation(vcable, seclist string) (*SecurityAssociationInfo, error) {
	spec := SecurityAssociationSpec{
		VCable:  c.getQualifiedName(vcable),
		SecList: c.getQualifiedName(seclist),
	}

	var assocInfo SecurityAssociationInfo
	if err := c.createResource(&spec, &assocInfo); err != nil {
		return nil, err
	}

	return c.success(&assocInfo)
}

// GetSecurityAssociation retrieves the security association with the given name.
func (c *SecurityAssociationsClient) GetSecurityAssociation(name string) (*SecurityAssociationInfo, error) {
	var assocInfo SecurityAssociationInfo
	if err := c.getResource(name, &assocInfo); err != nil {
		return nil, err
	}

	return c.success(&assocInfo)
}

// DeleteSecurityAssociation deletes the security association with the given name.
func (c *SecurityAssociationsClient) DeleteSecurityAssociation(name string) error {
	return c.deleteResource(name)
}
