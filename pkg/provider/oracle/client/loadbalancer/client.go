package loadbalancer

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"

	"github.com/docker/infrakit/pkg/provider/oracle/client/api"
)

// API Ref: https://docs.us-phoenix-1.oraclecloud.com/api/#/en/iaas/20160918/

// Client is a client for the load balancing services API.
type Client struct {
	Client        *api.Client
	CompartmentID string
}

// NewClient provides a Client interface for all Compute API calls
func NewClient(c *api.Client, compartmentID string) *Client {
	return &Client{
		Client:        c,
		CompartmentID: compartmentID,
	}
}

// Request builds the API endpoint given a URL and sends it to the API request
func (c *Client) Request(method string, reqURL string, body interface{}) (*http.Response, error) {
	// Parse URL Path
	urlPath, err := url.Parse(reqURL)
	if err != nil {
		return nil, err
	}
	// build URL using proper API version
	urlEndpoint, err := url.Parse(fmt.Sprintf(api.EndpointFormat, c.Client.APIRegion, api.LoadBalancerAPIVersion))
	if err != nil {
		log.Fatalf("Error parsing API Endpoint: %s", err)
	}
	urlEndpoint.Path = path.Join(urlEndpoint.Path, urlPath.Path)
	urlEndpoint.RawQuery = urlPath.RawQuery
	return c.Client.Request(method, urlEndpoint.String(), body)
}
