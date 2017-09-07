package compute

import (
	"fmt"
	"strings"
)

// IPAssociationsClient is a client for the IP Association functions of the Compute API.
type IPAssociationsClient struct {
	*ResourceClient
}

// IPAssociations obtains a IPAssociationsClient which can be used to access to the
// IP Association functions of the Compute API
func (c *AuthenticatedClient) IPAssociations() *IPAssociationsClient {
	return &IPAssociationsClient{
		ResourceClient: &ResourceClient{
			AuthenticatedClient: c,
			ResourceDescription: "ip association",
			ContainerPath:       "/ip/association/",
			ResourceRootPath:    "/ip/association",
		}}
}

// IPAssociationSpec defines an IP association to be created.
type IPAssociationSpec struct {
	VCable     string `json:"vcable"`
	ParentPool string `json:"parentpool"`
}

// IPAssociationInfo describes an existing IP association.
type IPAssociationInfo struct {
	Name        string `json:"name"`
	VCable      string `json:"vcable"`
	ParentPool  string `json:"parentpool"`
	URI         string `json:"uri"`
	Reservation string `json:"reservation"`
}

// CreateIPAssociation creates a new IP association with the supplied vcable and parentpool.
func (c *IPAssociationsClient) CreateIPAssociation(vcable, parentpool string) (*IPAssociationInfo, error) {
	spec := IPAssociationSpec{
		VCable:     c.getQualifiedName(vcable),
		ParentPool: c.getQualifiedParentPoolName(parentpool),
	}
	var assocInfo IPAssociationInfo
	if err := c.createResource(&spec, &assocInfo); err != nil {
		return nil, err
	}

	return c.success(&assocInfo)
}

func (c *IPAssociationsClient) success(assocInfo *IPAssociationInfo) (*IPAssociationInfo, error) {
	c.unqualify(&assocInfo.Name, &assocInfo.VCable)
	c.unqualifyParentPoolName(&assocInfo.ParentPool)
	return assocInfo, nil
}

// GetIPAssociation retrieves the IP association with the given name.
func (c *IPAssociationsClient) GetIPAssociation(name string) (*IPAssociationInfo, error) {
	var assocInfo IPAssociationInfo
	if err := c.getResource(name, &assocInfo); err != nil {
		return nil, err
	}

	return c.success(&assocInfo)
}

// DeleteIPAssociation deletes the IP association with the given name.
func (c *IPAssociationsClient) DeleteIPAssociation(name string) error {
	return c.deleteResource(name)
}

func (c *IPAssociationsClient) getQualifiedParentPoolName(parentpool string) string {
	parts := strings.Split(parentpool, ":")
	pooltype := parts[0]
	name := parts[1]
	return fmt.Sprintf("%s:%s", pooltype, c.getQualifiedName(name))
}

func (c *IPAssociationsClient) unqualifyParentPoolName(parentpool *string) {
	parts := strings.Split(*parentpool, ":")
	pooltype := parts[0]
	name := parts[1]
	*parentpool = fmt.Sprintf("%s:%s", pooltype, c.getUnqualifiedName(name))
}
