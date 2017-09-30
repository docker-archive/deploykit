package oracle

import (
	"github.com/docker/infrakit/pkg/provider/oracle/client/api"
	"github.com/docker/infrakit/pkg/provider/oracle/client/bmc"
	"github.com/docker/infrakit/pkg/provider/oracle/client/core"
	lb "github.com/docker/infrakit/pkg/provider/oracle/client/loadbalancer"
)

// NewClient returns an api client for use with all other services
func NewClient(options *Options) (*api.Client, error) {
	config := &bmc.Config{
		User:        bmc.String(options.UserID),
		Fingerprint: bmc.String(options.Fingerprint),
		KeyFile:     bmc.String(options.KeyFile),
		Tenancy:     bmc.String(options.TenancyID),
		// Pass the region or let the instance metadata dictate it
		Region: bmc.String(options.Region),
	}
	// Create the API Client
	return api.NewClient(config)
}

// CreateOLBClient returns a client for the load balancing service
func CreateOLBClient(apiClient *api.Client, options *Options) *lb.Client {
	// Create LB client
	return lb.NewClient(apiClient, options.ComponentID)
}

// CreateCoreClient returns a client for the core service
func CreateCoreClient(apiClient *api.Client, options *Options) *core.Client {
	// Create Core client
	return core.NewClient(apiClient, options.ComponentID)
}
