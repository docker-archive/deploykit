// Package compute provides an API client for the Oracle Cloud Platform (TM) Compute service.
package compute

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type credentials struct {
	identityDomain string
	userName       string
	password       string
}

func (c *credentials) computeUserName() string {
	return fmt.Sprintf("/Compute-%s/%s", c.identityDomain, c.userName)
}

// Client represents an unauthenticated compute client, with compute credentials and an api client.
type Client struct {
	credentials
	apiClient
}

// NewComputeClient creates a new, unauthenticated compute Client.
func NewComputeClient(identityDomain, userName, password string, apiEndpoint *url.URL) *Client {
	return &Client{
		credentials: credentials{
			identityDomain: identityDomain,
			userName:       userName,
			password:       password,
		},
		apiClient: apiClient{
			apiEndpoint: apiEndpoint,
			httpClient: &http.Client{
				Transport: &http.Transport{
					Proxy:               http.ProxyFromEnvironment,
					TLSHandshakeTimeout: 120 * time.Second},
			},
		},
	}
}

// GetObjectName returns the fully-qualified name of an OPC object, e.g. /identity-domain/user@email/{name}
func (c *Client) getQualifiedName(name string) string {
	if strings.HasPrefix(name, "/oracle") || strings.HasPrefix(name, "/Compute-") {
		return name
	}
	return fmt.Sprintf("%s/%s", c.computeUserName(), name)
}

func (c *Client) getObjectPath(root, name string) string {
	return fmt.Sprintf("%s%s", root, c.getQualifiedName(name))
}

// GetUnqualifiedName returns the unqualified name of an OPC object, e.g. the {name} part of /identity-domain/user@email/{name}
func (c *Client) getUnqualifiedName(name string) string {
	if name == "" {
		return name
	}
	if strings.HasPrefix(name, "/oracle") {
		return name
	}
	nameParts := strings.Split(name, "/")
	return strings.Join(nameParts[3:], "/")
}

func (c *Client) unqualify(names ...*string) {
	for _, name := range names {
		*name = c.getUnqualifiedName(*name)
	}
}
