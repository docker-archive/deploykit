package lb

import "github.com/FrenchBen/oracle-sdk-go/api"

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
