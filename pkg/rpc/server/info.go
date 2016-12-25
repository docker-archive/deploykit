package server

import (
	"encoding/json"
	"net/http"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi"
)

// PluginInfo is the service object for the RPC metadata service
type PluginInfo struct {
	vendor     spi.Vendor
	reflectors []*reflector
}

// NewPluginInfo returns an instance of the metadata service
func NewPluginInfo(receiver interface{}) (*PluginInfo, error) {
	m := &PluginInfo{
		reflectors: []*reflector{},
	}
	if v, is := receiver.(spi.Vendor); is {
		m.vendor = v
	}
	return m, m.Register(receiver)
}

// Register registers an rpc-capable object for introspection and metadata service
func (m *PluginInfo) Register(receiver interface{}) error {
	r := &reflector{target: receiver}
	if err := r.validate(); err != nil {
		return err
	}
	m.reflectors = append(m.reflectors, r)
	return nil
}

// ServeHTTP implements the http.Handler interface and responds by returning information about the plugin.
func (m *PluginInfo) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	meta := m.getInfo()
	buff, err := json.Marshal(meta)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write([]byte(err.Error()))
		return
	}
	resp.Write(buff)
	return
}

func (m *PluginInfo) getInfo() *plugin.Info {
	meta := &plugin.Info{}
	myImplements := []spi.InterfaceSpec{}
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
				InterfaceSpec: iface,
				Methods:       descriptions,
			})
	}

	if m.vendor != nil {
		meta.Vendor = m.vendor.VendorInfo()
	}

	meta.Implements = myImplements
	meta.Interfaces = myInterfaces
	return meta
}
