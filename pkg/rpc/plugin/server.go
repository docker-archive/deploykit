package plugin

import (
	"github.com/docker/infrakit/pkg/spi"
	"net/http"
)

// Plugin is the service for API metadata.
type Plugin struct {
	Spec spi.InterfaceSpec
}

// Implements responds to a request for the supported plugin interfaces.
func (p Plugin) Implements(_ *http.Request, req *ImplementsRequest, resp *ImplementsResponse) error {
	resp.APIs = []spi.InterfaceSpec{p.Spec}
	return nil
}
