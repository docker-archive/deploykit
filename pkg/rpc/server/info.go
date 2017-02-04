package server

import (
	"encoding/json"
	"net/http"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/template"
)

// TypedFunctionExporter is an interface implemented by plugins that supports multiple types in a single RPC endpoint.
// Each typed plugin can export some functions and this interface provides metadata about them.
type TypedFunctionExporter interface {

	// Types returns a list of types in this plugin
	Types() []string

	// FuncsByType returns the template functions exported by each typed plugin
	FuncsByType(string) []template.Function
}

// PluginInfo is the service object for the RPC metadata service
type PluginInfo struct {
	vendor     spi.Vendor
	reflectors []*reflector
	receiver   interface{}
}

// NewPluginInfo returns an instance of the metadata service
func NewPluginInfo(receiver interface{}) (*PluginInfo, error) {
	m := &PluginInfo{
		reflectors: []*reflector{},
		receiver:   receiver,
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

// ShowAPI responds by returning information about the plugin.
func (m *PluginInfo) ShowAPI(resp http.ResponseWriter, req *http.Request) {
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

type functionInfo struct {
	Name        string
	Description string
	Function    string
	Usage       string
}

// ShowTemplateFunctions responds by returning information about template functions the plugin expopses.
func (m *PluginInfo) ShowTemplateFunctions(resp http.ResponseWriter, req *http.Request) {
	result := map[string][]functionInfo{}
	base := []functionInfo{}
	exporter, is := m.receiver.(template.FunctionExporter)
	if is {
		for _, f := range exporter.Funcs() {
			base = append(base, functionInfo{
				Name:        f.Name,
				Description: f.Description,
				Function:    printFunc(f.Name, f.Func),
				Usage:       printUsage(f.Name, f.Func),
			})
		}
		if len(base) > 0 {
			result["base"] = base
		}
	}

	texporter, is := m.receiver.(TypedFunctionExporter)
	if is {
		for _, t := range texporter.Types() {
			typed := []functionInfo{}

			for _, f := range texporter.FuncsByType(t) {
				typed = append(typed, functionInfo{
					Name:        f.Name,
					Description: f.Description,
					Function:    printFunc(f.Name, f.Func),
					Usage:       printUsage(f.Name, f.Func),
				})
			}

			result[t] = typed
		}
	}

	buff, err := json.Marshal(result)
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
