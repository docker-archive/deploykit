package core

import "github.com/FrenchBen/oracle-sdk-go/api"

// https://docs.us-phoenix-1.oraclecloud.com/api/#/en/iaas/20160918/
// CoreClient is a client for the core services API.
type CoreClient struct {
	Client        *api.Client
	CompartmentID string
}

// NewCoreClient provides a Client interface for all Compute API calls
func NewCoreClient(c *api.Client, compartmentID string) *CoreClient {
	return &CoreClient{
		Client:        c,
		CompartmentID: compartmentID,
	}
}
