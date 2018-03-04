package run

// TODO -- this needs to be versioned because all the interface / spi packages are versioned.

import (
	"fmt"

	"github.com/docker/infrakit/pkg/discovery"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc/client"
	controller_rpc "github.com/docker/infrakit/pkg/rpc/controller"
	event_rpc "github.com/docker/infrakit/pkg/rpc/event"
	flavor_rpc "github.com/docker/infrakit/pkg/rpc/flavor"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	instance_rpc "github.com/docker/infrakit/pkg/rpc/instance"
	loadbalancer_rpc "github.com/docker/infrakit/pkg/rpc/loadbalancer"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	resource_rpc "github.com/docker/infrakit/pkg/rpc/resource"
	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/controller"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/spi/resource"
	"github.com/docker/infrakit/pkg/spi/stack"
)

var log = logutil.New("module", "run")

// PluginCode is the type code for exposing the correct RPC interface for a given object.
// We need type information because some object like Manager implements multiple spi interfaces
// and type information is necessary to know which RPC interface needs to bind to the object.
// This is so that an object that implements both Group and Metadata spi can be bound to separate RPC interfaces.
type PluginCode int

const (
	// Manager is the type code for Manager
	Manager PluginCode = iota
	// Controller is the type code for Controller implementation
	Controller
	//Instance is the type code for Instance SPI implementation
	Instance
	// Flavor is the type code for Flavor SPI implementation
	Flavor
	// Group is the type code for Group SPI implementation
	Group
	// Metadata is the type code for Metadata SPI implementation
	Metadata
	// MetadataUpdatable is the type code for updatable Metadata SPI implementation
	MetadataUpdatable
	// Event is the type code for Event SPI implementation
	Event
	// Resource is the type code for Resource SPI implementation
	Resource
	// L4 is the type code for L4 loadbalancer implementation
	L4
)

// ServeRPC starts the RPC endpoint / server given a plugin name for lookup and a list of plugin objects
// that implements the pkg/spi/ interfaces. onStop is a callback invoked when the the endpoint shuts down.
func ServeRPC(transport plugin.Transport, onStop func(),
	impls map[PluginCode]interface{}) (stoppable server.Stoppable, running <-chan struct{}, err error) {

	// Get the server interfaces to be exported.  Do this by checking on the types of the implementations
	// and wrap the implementation with a rpc adaptor
	plugins := []server.VersionedInterface{}

	for code, p := range impls {

		switch code {

		case Manager:
			log.Debug("manager_rpc.PluginServer", "p", p)
			plugins = append(plugins, manager_rpc.PluginServer(p.(stack.Interface)))
		case Controller:
			switch pp := p.(type) {
			case func() (map[string]controller.Controller, error):
				log.Debug("controller_rpc.ControllerServerWithNamed", "pp", pp)
				plugins = append(plugins, controller_rpc.ServerWithNames(pp))
			case controller.Controller:
				log.Debug("controller_rpc.ControllerServer", "p", p)
				plugins = append(plugins, controller_rpc.Server(p.(controller.Controller)))
			default:
				err = fmt.Errorf("bad plugin %v for code %v", p, code)
				return
			}
		case Instance:
			switch pp := p.(type) {
			case map[string]instance.Plugin:
				log.Debug("instance_rpc.PluginServerWithTypes", "pp", pp)
				plugins = append(plugins, instance_rpc.PluginServerWithTypes(pp))
			case instance.Plugin:
				log.Debug("instance_rpc.PluginServer", "pp", pp)
				plugins = append(plugins, instance_rpc.PluginServer(pp))
			default:
				err = fmt.Errorf("bad plugin %v for code %v", p, code)
				panic(err)
			}
		case Flavor:
			switch pp := p.(type) {
			case map[string]flavor.Plugin:
				log.Debug("flavor_rpc.PluginServerWithTypes", "pp", pp)
				plugins = append(plugins, flavor_rpc.PluginServerWithTypes(pp))
			case flavor.Plugin:
				log.Debug("flavor_rpc.PluginServer", "pp", pp)
				plugins = append(plugins, flavor_rpc.PluginServer(pp))
			default:
				err = fmt.Errorf("bad plugin %v for code %v", p, code)
				panic(err)
			}
		case MetadataUpdatable:
			switch pp := p.(type) {
			case func() (map[string]metadata.Plugin, error):
				log.Debug("metadata_rpc.UpdatablePluginServerWithNames", "p", pp)
				plugins = append(plugins, metadata_rpc.UpdatableServerWithNames(pp))
			case metadata.Updatable:
				log.Debug("metadata_rpc.UpdatablePluginServer", "p", p)
				plugins = append(plugins, metadata_rpc.UpdatableServer(pp))
			default:
				err = fmt.Errorf("bad plugin %v for code %v", p, code)
				panic(err)
			}
		case Metadata:
			switch pp := p.(type) {
			case func() (map[string]metadata.Plugin, error):
				log.Debug("metadata_rpc.PluginServerWithTypes", "pp", pp)
				plugins = append(plugins, metadata_rpc.ServerWithNames(pp))
			case metadata.Plugin:
				log.Debug("metadata_rpc.PluginServer", "p", pp)
				plugins = append(plugins, metadata_rpc.Server(pp))
			default:
				err = fmt.Errorf("bad plugin %v for code %v", p, code)
				panic(err)
			}
		case Event:
			switch pp := p.(type) {
			case func() (map[string]event.Plugin, error):
				log.Debug("event_rpc.PluginServerWithNames", "pp", pp)
				plugins = append(plugins, event_rpc.PluginServerWithNames(pp))
			case event.Plugin:
				log.Debug("event_rpc.PluginServer", "pp", pp)
				plugins = append(plugins, event_rpc.PluginServer(pp))
			default:
				err = fmt.Errorf("bad plugin %v for code %v", p, code)
				panic(err)
			}
		case Group:
			switch pp := p.(type) {
			case func() (map[group.ID]group.Plugin, error):
				log.Debug("group_rpc.PluginServerWithGroups", "pp", pp)
				plugins = append(plugins, group_rpc.PluginServerWithGroups(pp))
			case group.Plugin:
				log.Debug("group_rpc.PluginServer", "p", p)
				plugins = append(plugins, group_rpc.PluginServer(p.(group.Plugin)))
			default:
				err = fmt.Errorf("bad plugin %v for code %v", p, code)
				panic(err)
			}
		case Resource:
			log.Debug("resource_rpc.PluginServer", "p", p)
			plugins = append(plugins, resource_rpc.PluginServer(p.(resource.Plugin)))
		case L4:
			log.Debug("loadbalancer_rpc.PluginServer", "p", p)
			switch pp := p.(type) {

			// This will create a plugin at name/type so that it's fully qualified.
			// Note that L4 will be bound to an ingress which is a Controller, and Controllers can have subtypes
			// so that they map to the domain/host associated to the loadbalancer. For example ingress/test.com
			case func() (map[string]loadbalancer.L4, error):
				log.Debug("loadbalancer_rpc.PluginServerWithNames", "pp", pp)
				plugins = append(plugins, loadbalancer_rpc.PluginServerWithNames(pp))
			case loadbalancer.L4:
				log.Debug("loadbalancer_rpc.PluginServer", "p", p)
				plugins = append(plugins, loadbalancer_rpc.PluginServer(p.(loadbalancer.L4)))
			default:
				err = fmt.Errorf("bad plugin %v for code %v", p, code)
				panic(err)
			}

		default:
			err = fmt.Errorf("unknown plugin %v, code %v", p, code)
			panic(err)

		}

	}

	if transport.Listen == "" {
		stoppable, running = BackgroundPlugin(transport, onStop, plugins[0], plugins[1:]...)
		return
	}
	stoppable, running = BackgroundListener(transport, onStop, plugins[0], plugins[1:]...)
	return
}

// Call looks up the the plugin objects by the interface type and executes the work.
func Call(plugins func() discovery.Plugins,
	interfaceSpec spi.InterfaceSpec, name *plugin.Name, work interface{}) error {

	pm, err := plugins().List()
	if err != nil {
		return err
	}

	lookup := ""
	if name != nil {
		lookup, _ = name.GetLookupAndType()
	}

	for n, endpoint := range pm {

		rpcClient, err := client.New(endpoint.Address, interfaceSpec)
		if err == nil {
			// interface type match.  now check for name match
			if lookup != "" && lookup != n {
				continue
			}

			pn := plugin.Name(n)
			if name != nil {
				pn = *name
			}
			switch interfaceSpec {
			case stack.InterfaceSpec:
				do, is := work.(func(stack.Interface) error)
				if !is {
					return fmt.Errorf("wrong function prototype for %v", interfaceSpec)
				}
				v := manager_rpc.Adapt(rpcClient)
				return do(v)
			case controller.InterfaceSpec:
				do, is := work.(func(controller.Controller) error)
				if !is {
					return fmt.Errorf("wrong function prototype for %v", interfaceSpec)
				}
				v := controller_rpc.Adapt(pn, rpcClient)
				return do(v)
			case group.InterfaceSpec:
				do, is := work.(func(group.Plugin) error)
				if !is {
					return fmt.Errorf("wrong function prototype for %v", interfaceSpec)
				}
				v := group_rpc.Adapt(pn, rpcClient)
				return do(v)
			case instance.InterfaceSpec:
				do, is := work.(func(instance.Plugin) error)
				if !is {
					return fmt.Errorf("wrong function prototype for %v", interfaceSpec)
				}
				v := instance_rpc.Adapt(pn, rpcClient)
				return do(v)
			case flavor.InterfaceSpec:
				do, is := work.(func(flavor.Plugin) error)
				if !is {
					return fmt.Errorf("wrong function prototype for %v", interfaceSpec)
				}
				v := flavor_rpc.Adapt(pn, rpcClient)
				return do(v)
			case metadata.InterfaceSpec:
				do, is := work.(func(metadata.Plugin) error)
				if !is {
					return fmt.Errorf("wrong function prototype for %v", interfaceSpec)
				}
				v := metadata_rpc.Adapt(pn, rpcClient)
				return do(v)
			case metadata.UpdatableInterfaceSpec:
				do, is := work.(func(metadata.Updatable) error)
				if !is {
					return fmt.Errorf("wrong function prototype for %v", interfaceSpec)
				}
				v := metadata_rpc.AdaptUpdatable(pn, rpcClient)
				return do(v)
			case loadbalancer.InterfaceSpec:
				do, is := work.(func(loadbalancer.L4) error)
				if !is {
					return fmt.Errorf("wrong function prototype for %v", interfaceSpec)
				}
				v := loadbalancer_rpc.Adapt(pn, rpcClient)
				return do(v)
			default:
			}
		}
	}
	return nil
}
