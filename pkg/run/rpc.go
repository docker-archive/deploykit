package run

import (
	"fmt"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/manager"
	"github.com/docker/infrakit/pkg/plugin"
	event_rpc "github.com/docker/infrakit/pkg/rpc/event"
	flavor_rpc "github.com/docker/infrakit/pkg/rpc/flavor"
	group_rpc "github.com/docker/infrakit/pkg/rpc/group"
	instance_rpc "github.com/docker/infrakit/pkg/rpc/instance"
	manager_rpc "github.com/docker/infrakit/pkg/rpc/manager"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	resource_rpc "github.com/docker/infrakit/pkg/rpc/resource"
	"github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/spi/resource"
)

// ServeRPC starts the RPC endpoint / server given a plugin name for lookup and a list of plugin objects
// that implements the pkg/spi/ interfaces. onStop is a callback invoked when the the endpoint shuts down.
func ServeRPC(name plugin.Name, onStop func(),
	impls []interface{}) (stoppable server.Stoppable, running <-chan struct{}, err error) {

	// Get the server interfaces to be exported.  Do this by checking on the types of the implementations
	// and wrap the implementation with a rpc adaptor
	plugins := []server.VersionedInterface{}

	for i, impl := range impls {

		switch p := impl.(type) {

		case event.Plugin:
			plugins = append(plugins, event_rpc.PluginServer(p))
		case flavor.Plugin:
			plugins = append(plugins, flavor_rpc.PluginServer(p))
		case group.Plugin:
			plugins = append(plugins, group_rpc.PluginServer(p))
		case instance.Plugin:
			plugins = append(plugins, instance_rpc.PluginServer(p))
		case manager.Backend:
			plugins = append(plugins, manager_rpc.PluginServer(p))
		case metadata.Plugin:
			plugins = append(plugins, metadata_rpc.PluginServer(p))
		case resource.Plugin:
			plugins = append(plugins, resource_rpc.PluginServer(p))

		default:
			err = fmt.Errorf("unknown plugin %v, index %v", p, i)
			return
		}

	}

	lookupName, _ := name.GetLookupAndType() // for aws/ec2, start with 'aws' for example.
	stoppable, running = cli.BackgroundPlugin(lookupName, onStop, plugins[0], plugins[1:]...)

	return
}
