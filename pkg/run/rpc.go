package run

import (
	"fmt"

	"github.com/docker/infrakit/pkg/controller"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
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
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/loadbalancer"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/spi/resource"
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
			plugins = append(plugins, manager_rpc.PluginServer(p.(manager.Manager)))
		case Controller:
			switch pp := p.(type) {
			case func() (map[string]controller.Controller, error):
				log.Debug("controller_rpc.ControllerServerWithTypes", "pp", pp)
				plugins = append(plugins, controller_rpc.ServerWithTypes(pp))
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
				return
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
				return
			}
		case MetadataUpdatable:
			log.Debug("metadata_rpc.UpdatablePluginServer", "p", p)
			plugins = append(plugins, metadata_rpc.UpdatablePluginServer(p.(metadata.Updatable)))
		case Metadata:
			switch pp := p.(type) {
			case map[string]metadata.Plugin:
				log.Debug("metadata_rpc.PluginServerWithTypes", "pp", pp)
				plugins = append(plugins, metadata_rpc.PluginServerWithTypes(pp))
			case metadata.Plugin:
				log.Debug("metadata_rpc.PluginServer", "p", pp)
				plugins = append(plugins, metadata_rpc.PluginServer(pp))
			default:
				err = fmt.Errorf("bad plugin %v for code %v", p, code)
				return
			}
		case Event:
			switch pp := p.(type) {
			case map[string]event.Plugin:
				log.Debug("event_rpc.PluginServerWithTypes", "pp", pp)
				plugins = append(plugins, event_rpc.PluginServerWithTypes(pp))
			case event.Plugin:
				log.Debug("event_rpc.PluginServer", "pp", pp)
				plugins = append(plugins, event_rpc.PluginServer(pp))
			default:
				err = fmt.Errorf("bad plugin %v for code %v", p, code)
				return
			}
		case Group:
			switch pp := p.(type) {
			case func() (map[string]group.Plugin, error):
				log.Debug("group_rpc.PluginServerWithTypes", "pp", pp)
				plugins = append(plugins, group_rpc.PluginServerWithTypes(pp))
			case group.Plugin:
				log.Debug("group_rpc.PluginServer", "p", p)
				plugins = append(plugins, group_rpc.PluginServer(p.(group.Plugin)))
			default:
				err = fmt.Errorf("bad plugin %v for code %v", p, code)
				return
			}
		case Resource:
			log.Debug("resource_rpc.PluginServer", "p", p)
			plugins = append(plugins, resource_rpc.PluginServer(p.(resource.Plugin)))
		case L4:
			log.Debug("loadbalancer_rpc.PluginServer", "p", p)
			// This will create a plugin at name/type so that it's fully qualified.
			// Note that L4 will be bound to an ingress which is a Controller, and Controllers can have subtypes
			// so that they map to the domain/host associated to the loadbalancer. For example ingress/test.com
			plugins = append(plugins, loadbalancer_rpc.PluginServer(p.(loadbalancer.L4)).WithType(p.(loadbalancer.L4).Name()))

		default:
			err = fmt.Errorf("unknown plugin %v, code %v", p, code)
			return
		}

	}

	if transport.Listen == "" {
		stoppable, running = BackgroundPlugin(transport, onStop, plugins[0], plugins[1:]...)
		return
	}
	stoppable, running = BackgroundListener(transport, onStop, plugins[0], plugins[1:]...)
	return
}
