package compute

import (
	"fmt"
)

// ResourceClient is an AuthenticatedClient with some additional information about the resources to be addressed.
type ResourceClient struct {
	*AuthenticatedClient
	ResourceDescription string
	ContainerPath       string
	ResourceRootPath    string
}

func (c *ResourceClient) createResource(requestBody interface{}, responseBody interface{}) error {
	req, err := c.newAuthenticatedPostRequest(c.ContainerPath, requestBody)
	if err != nil {
		return err
	}

	rsp, err := c.requestAndCheckStatus(fmt.Sprintf("create %s", c.ResourceDescription), req)
	if err != nil {
		return err
	}

	return unmarshalResponseBody(rsp, responseBody)
}

func (c *ResourceClient) updateResource(name string, requestBody interface{}, responseBody interface{}) error {
	req, err := c.newAuthenticatedPutRequest(c.getObjectPath(c.ResourceRootPath, name), requestBody)
	if err != nil {
		return err
	}

	rsp, err := c.requestAndCheckStatus(fmt.Sprintf("update %s", c.ResourceDescription), req)
	if err != nil {
		return err
	}

	return unmarshalResponseBody(rsp, responseBody)
}

func (c *ResourceClient) getResource(name string, responseBody interface{}) error {
	req, err := c.newAuthenticatedGetRequest(c.getObjectPath(c.ResourceRootPath, name))
	if err != nil {
		return err
	}

	rsp, err := c.requestAndCheckStatus(fmt.Sprintf("get %s details", c.ResourceDescription), req)
	if err != nil {
		return err
	}

	return unmarshalResponseBody(rsp, responseBody)
}

func (c *ResourceClient) deleteResource(name string) error {
	req, err := c.newAuthenticatedDeleteRequest(c.getObjectPath(c.ResourceRootPath, name))
	if err != nil {
		return err
	}
	_, err = c.requestAndCheckStatus(fmt.Sprintf("delete %s", c.ResourceDescription), req)
	return err
}
