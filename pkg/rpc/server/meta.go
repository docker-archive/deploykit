package server

import (
	"net/http"

	"github.com/docker/infrakit/pkg/plugin"
)

// Metadata is the service object for the RPC metadata service
type Metadata struct {
	vendor     plugin.Vendor
	reflectors []*reflector
}

// NewMetadata returns an instance of the metadata service
func NewMetadata(receiver interface{}) (*Metadata, error) {
	m := &Metadata{
		reflectors: []*reflector{},
	}
	if v, is := receiver.(plugin.Vendor); is {
		m.vendor = v
	}
	return m, m.Register(receiver)
}

// Register registers an rpc-capable object for introspection and metadata service
func (m *Metadata) Register(receiver interface{}) error {
	r := &reflector{target: receiver}
	if err := r.validate(); err != nil {
		return err
	}
	m.reflectors = append(m.reflectors, r)
	return nil
}

// Meta exposes a simple RPC method that returns information about the plugin's interfaces and
// versions.
func (m *Metadata) Meta(_ *http.Request, req *plugin.EmptyRequest, resp *plugin.Meta) error {
	myImplements := []plugin.Interface{}
	myInterfaces := []plugin.InterfaceDescription{}

	for _, r := range m.reflectors {

		iface := r.Interface()
		myImplements = append(myImplements, iface)

		descriptions := []plugin.MethodDescription{}
		for _, method := range r.pluginMethods() {

			desc := r.toDescription(method)
			descriptions = append(descriptions, desc)

			// sets the example properties which are the custom types for the plugin
			r.setExampleProperties(desc.Request.Params)
		}

		myInterfaces = append(myInterfaces,
			plugin.InterfaceDescription{
				Interface: iface,
				Methods:   descriptions,
			})
	}

	if m.vendor != nil {
		resp.Vendor = m.vendor.Info()
	}

	resp.Implements = myImplements
	resp.Interfaces = myInterfaces

	return nil
}
