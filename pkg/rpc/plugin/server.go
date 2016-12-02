package plugin

import (
	"github.com/docker/infrakit/pkg/spi"
	"net/http"
)

// Plugin is the service for API metadata.
type Plugin struct {
	Spec spi.APISpec
}

// APIs responds to a request for the supported plugin APIs.
func (p Plugin) APIs(_ *http.Request, req *APIsRequest, resp *APIsResponse) error {
	resp.APIs = []spi.APISpec{p.Spec}
	return nil
}
