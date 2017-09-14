package lb

import "github.com/FrenchBen/oracle-sdk-go/api"

// https://docs.us-phoenix-1.oraclecloud.com/api/#/en/iaas/20160918/
// LBClient is a client for the load balancing services API.
type LBClient struct {
	Client        *api.Client
	CompartmentID string
}

// NewLBClient provides a Client interface for all Compute API calls
func NewLBClient(c *api.Client, compartmentID string) *LBClient {
	return &LBClient{
		Client:        c,
		CompartmentID: compartmentID,
	}
}
