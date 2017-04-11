package application

import (
	"fmt"
	"net/http"

	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/application"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// PluginServer returns a Application that conforms to the net/rpc rpc call convention.
func PluginServer(p application.Plugin) *Application {
	return &Application{plugin: p}
}

// PluginServerWithTypes which supports multiple types of application plugins. The de-multiplexing
// is done by the server's RPC method implementations.
func PluginServerWithTypes(typed map[string]application.Plugin) *Application {
	return &Application{typedPlugins: typed}
}

// Application the exported type needed to conform to json-rpc call convention
type Application struct {
	plugin       application.Plugin
	typedPlugins map[string]application.Plugin // by type, as qualified in the name of the plugin
}

// VendorInfo returns a metadata object about the plugin, if the plugin implements it.  See spi.Vendor
func (p *Application) VendorInfo() *spi.VendorInfo {
	if p.plugin == nil {
		return nil
	}

	if m, is := p.plugin.(spi.Vendor); is {
		return m.VendorInfo()
	}
	return nil
}

// Funcs implements the template.FunctionExporter method to expose help for plugin's template functions
func (p *Application) Funcs() []template.Function {
	f, is := p.plugin.(template.FunctionExporter)
	if !is {
		return []template.Function{}
	}
	return f.Funcs()
}

// Types implements server.TypedFunctionExporter
func (p *Application) Types() []string {
	if p.typedPlugins == nil {
		return nil
	}
	list := []string{}
	for k := range p.typedPlugins {
		list = append(list, k)
	}
	return list
}

// FuncsByType implements server.TypedFunctionExporter
func (p *Application) FuncsByType(t string) []template.Function {
	if p.typedPlugins == nil {
		return nil
	}
	fp, has := p.typedPlugins[t]
	if !has {
		return nil
	}
	exp, is := fp.(template.FunctionExporter)
	if !is {
		return nil
	}
	return exp.Funcs()
}

// SetExampleProperties sets the rpc request with any example properties/ custom type
func (p *Application) SetExampleProperties(request interface{}) {
	// TODO(chungers) - support typed plugins
	if p.plugin == nil {
		return
	}

	i, is := p.plugin.(spi.InputExample)
	if !is {
		return
	}
	example := i.ExampleProperties()
	if example == nil {
		return
	}

	switch request := request.(type) {
	case *HealthyRequest:
		request.Properties = example
	}
}

// exampleProperties returns an example properties used by the plugin
func (p *Application) exampleProperties() *types.Any {
	if i, is := p.plugin.(spi.InputExample); is {
		return i.ExampleProperties()
	}
	return nil
}

// ImplementedInterface returns the interface implemented by this RPC service.
func (p *Application) ImplementedInterface() spi.InterfaceSpec {
	return application.InterfaceSpec
}

func (p *Application) getPlugin(applicationType string) application.Plugin {
	if applicationType == "" {
		return p.plugin
	}
	if p, has := p.typedPlugins[applicationType]; has {
		return p
	}
	return nil
}

// Validate checks whether the helper can support a configuration.
func (p *Application) Validate(_ *http.Request, req *ValidateRequest, resp *ValidateResponse) error {
	resp.Type = req.Type
	c := p.getPlugin(req.Type)
	if c == nil {
		return fmt.Errorf("no-plugin:%s", req.Type)
	}
	err := c.Validate(req.Properties)
	if err != nil {
		return err
	}
	resp.OK = true
	return nil
}

// Healthy determines whether an instance is healthy.
func (p *Application) Healthy(_ *http.Request, req *HealthyRequest, resp *HealthyResponse) error {
	resp.Type = req.Type
	c := p.getPlugin(req.Type)
	if c == nil {
		return fmt.Errorf("no-plugin:%s", req.Type)
	}
	health, err := c.Healthy(req.Properties)
	if err != nil {
		return err
	}
	resp.Health = health
	return nil
}

// Update specify resource information
func (p *Application) Update(_ *http.Request, req *UpdateRequest, resp *UpdateResponse) error {
	resp.Type = req.Type
	c := p.getPlugin(req.Type)
	if c == nil {
		return fmt.Errorf("no-plugin:%s", req.Type)
	}
	err := c.Update(req.Message)
	if err != nil {
		resp.OK = false
		return err
	}
	resp.OK = true
	return nil

}
