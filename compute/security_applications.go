package compute

// SecurityApplicationsClient is a client for the Security Application functions of the Compute API.
type SecurityApplicationsClient struct {
	ResourceClient
}

// SecurityApplications obtains a SecurityApplicationsClient which can be used to access to the
// Security Application functions of the Compute API
func (c *AuthenticatedClient) SecurityApplications() *SecurityApplicationsClient {
	return &SecurityApplicationsClient{
		ResourceClient: ResourceClient{
			AuthenticatedClient: c,
			ResourceDescription: "security application",
			ContainerPath:       "/secapplication/",
			ResourceRootPath:    "/secapplication",
		}}
}

// SecurityApplicationSpec defines a security application to be created.
type SecurityApplicationSpec struct {
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	DPort       string `json:"dport"`
	ICMPType    string `json:"icmptype,omitempty"`
	ICMPCode    string `json:"icmpcode,omitempty"`
	Description string `json:"description"`
}

// SecurityApplicationInfo describes an existing security application.
type SecurityApplicationInfo struct {
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	DPort       string `json:"dport"`
	ICMPType    string `json:"icmptype"`
	ICMPCode    string `json:"icmpcode"`
	Description string `json:"description"`
	URI         string `json:"uri"`
}

func (c *SecurityApplicationsClient) success(result *SecurityApplicationInfo) (*SecurityApplicationInfo, error) {
	c.unqualify(&result.Name)
	return result, nil
}

// CreateSecurityApplication creates a new security application.
func (c *SecurityApplicationsClient) CreateSecurityApplication(name, protocol, dport, icmptype, icmpcode, description string) (*SecurityApplicationInfo, error) {
	spec := SecurityApplicationSpec{
		Name:        c.getQualifiedName(name),
		Protocol:    protocol,
		DPort:       dport,
		ICMPType:    icmptype,
		ICMPCode:    icmpcode,
		Description: description,
	}

	var appInfo SecurityApplicationInfo
	if err := c.createResource(&spec, &appInfo); err != nil {
		return nil, err
	}

	return c.success(&appInfo)
}

// GetSecurityApplication retrieves the security application with the given name.
func (c *SecurityApplicationsClient) GetSecurityApplication(name string) (*SecurityApplicationInfo, error) {
	var appInfo SecurityApplicationInfo
	if err := c.getResource(name, &appInfo); err != nil {
		return nil, err
	}

	return c.success(&appInfo)
}

// DeleteSecurityApplication deletes the security application with the given name.
func (c *SecurityApplicationsClient) DeleteSecurityApplication(name string) error {
	return c.deleteResource(name)
}
