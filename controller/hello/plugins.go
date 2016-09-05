package hello

import (
	"encoding/json"
	"fmt"
	"github.com/docker/engine-api/types"
	"github.com/docker/libmachete/controller"
	"golang.org/x/net/context"
)

// Plugin is a specification of the plugin by name (owner/repo)
type Plugin struct {
	Name string `json:"name"`
}

// PluginDiscovery is a composition of plugin with actual discovered socket address
type PluginDiscovery struct {
	Plugin
	Socket string `json:"socket,omitempty"`
}

// PluginCall is a composite of discovered plugin with operation and arg
type PluginCall struct {
	PluginDiscovery
	Op  string           `json:"operation,omitempty"`
	Arg *json.RawMessage `json:"arg,omitempty"`
}

// DiscoverPlugin uses the docker engine api to discover plugin specified
func (h *Server) DiscoverPlugin(p Plugin) (*PluginDiscovery, error) {
	// discover the plugin
	plugins, err := h.docker.PluginList(context.Background())
	if err != nil {
		return nil, err
	}

	var found *types.Plugin
	for _, plugin := range plugins {
		if plugin.Name == p.Name {
			found = plugin
			break
		}
	}

	if found == nil {
		return nil, fmt.Errorf("plugin not found: %s", p.Name)
	}

	if !found.Active {
		return nil, fmt.Errorf("plugin not active: %s, id=%s", found.Name, found.ID)
	}

	pluginSocket := fmt.Sprintf("/run/docker/%s/%s", found.ID, found.Manifest.Interface.Socket)
	return &PluginDiscovery{
		Plugin: p,
		Socket: pluginSocket,
	}, nil
}

// CallPlugin calls the plugin specified by the input.  The socket field is not nil.
// The input is the output of the discover with op and arg filled in.
func (h *Server) CallPlugin(call PluginCall) (interface{}, error) {
	if call.PluginDiscovery.Socket == "" {
		return nil, fmt.Errorf("no-socket")
	}
	// now connect -- this assumes the volume is bind mounted...
	client := controller.NewClient(call.PluginDiscovery.Socket)

	var arg interface{}
	if call.Arg != nil {
		arg = map[string]interface{}{}
		err := json.Unmarshal(*call.Arg, &arg)
		if err != nil {
			return nil, err
		}
	}
	result := map[string]interface{}{}
	err := client.Call(call.Op, arg, &result)
	return result, err
}
