package metadata

import (
	"net/http"
	"sort"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/rpc/internal"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/metadata"
)

// ServerWithNames which supports namespaced plugins
func ServerWithNames(subplugins func() (map[string]metadata.Plugin, error)) *Metadata {

	keyed := internal.ServeKeyed(
		// This is where templates would be nice...
		func() (map[string]interface{}, error) {
			m, err := subplugins()
			if err != nil {
				return nil, err
			}

			out := map[string]interface{}{}
			for k, v := range m {
				out[string(k)] = v
			}
			return out, nil
		},
	)
	return &Metadata{keyed: keyed}
}

// Server returns a Metadata that conforms to the net/rpc rpc call convention.
func Server(p metadata.Plugin) *Metadata {
	return &Metadata{keyed: internal.ServeSingle(p)}
}

// Metadata the exported type needed to conform to json-rpc call convention
type Metadata struct {
	keyed *internal.Keyed
}

// VendorInfo returns a metadata object about the plugin, if the plugin implements it.  See spi.Vendor
func (p *Metadata) VendorInfo() *spi.VendorInfo {
	base, _ := p.keyed.Keyed(plugin.Name("."))
	if m, is := base.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *Metadata) ImplementedInterface() spi.InterfaceSpec {
	return metadata.InterfaceSpec
}

// Objects returns the objects exposed by this kind of RPC service
func (p *Metadata) Objects() []rpc.Object {
	return p.keyed.Objects()
}

// Keys returns a list of child nodes given a path.
func (p *Metadata) Keys(_ *http.Request, req *KeysRequest, resp *KeysResponse) error {

	return p.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		nodes, err := v.(metadata.Plugin).Keys(req.Path)
		if err == nil {
			sort.Strings(nodes)
			resp.Nodes = nodes
		}
		return err
	})
}

// Get retrieves the value at path given.
func (p *Metadata) Get(_ *http.Request, req *GetRequest, resp *GetResponse) error {

	return p.keyed.Do(req, func(v interface{}) error {
		resp.Name = req.Name
		value, err := v.(metadata.Plugin).Get(req.Path)
		if err == nil {
			resp.Value = value
		}
		return err
	})
}
