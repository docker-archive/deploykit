package server

import (
	"net/http"
	"reflect"

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
	myImplements := []plugin.TypeVersion{}
	myInterfaces := []plugin.Interface{}

	for _, r := range m.reflectors {

		v, err := r.TypeVersion()
		if err != nil {
			return err
		}
		myImplements = append(myImplements, v)

		descriptions := []plugin.MethodDescription{}
		for _, method := range r.pluginMethods() {

			desc := r.toDescription(method)
			descriptions = append(descriptions, desc)

			// Set example value from the vendor plugin: this is the custom typed
			// that is carried as opaque json blob value for `Properties`
			// Note that only for Instance.Provision do we need to recurse into
			// the request struct to set the example value.  For flavor and group,
			// their custom configs are all at the top level and the Properties contained
			// in the Spec fields are not associated to the custom config of the flavor
			// or the group plugin themselves (it's for instance plugin).
			example := r.exampleProperties()
			for _, param := range desc.Params {

				// Only special case of Instance.Provision and Group.CommitGroup
				// requires to recursively set the Properties because
				// there are RawMessages inside Spec.
				// TODO(chungers) -- this logic really belongs in the RPC plugin
				// packages.
				recursive := desc.Method == "Instance.Provision" ||
					desc.Method == "Group.CommitGroup"
				setFieldValue("Properties", reflect.ValueOf(param), example, recursive)
			}
		}

		myInterfaces = append(myInterfaces, plugin.Interface{Name: v, Methods: descriptions})
	}

	if m.vendor != nil {
		resp.Vendor = m.vendor.Info()
	}

	resp.Implements = myImplements
	resp.Interfaces = myInterfaces

	return nil
}
