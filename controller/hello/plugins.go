package hello

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/types"
	"github.com/docker/libmachete/controller"
	"golang.org/x/net/context"
)

// Plugin is a specification of the plugin by name (owner/repo)
type Plugin struct {
	Name string `json:"name"`
}

// PluginRef is a composition of plugin with actual discovered socket address
type PluginRef struct {
	Plugin
	Socket string `json:"socket,omitempty"`
}

// PluginCall is a composite of discovered plugin with operation and arg
type PluginCall struct {
	PluginRef
	Op  string           `json:"operation,omitempty"`
	Arg *json.RawMessage `json:"arg,omitempty"`
}

// DiscoverPlugin uses the docker engine api to discover plugin specified
func (h *Server) DiscoverPlugin(p Plugin) (*PluginRef, error) {
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
	return &PluginRef{
		Plugin: p,
		Socket: pluginSocket,
	}, nil
}

// InstallPlugin installs the specified plugin and returns the actual reference if it's running successfully.
func (h *Server) InstallPlugin(p Plugin) (*PluginRef, error) {
	// discover if the plugin is running.
	running, err := h.DiscoverPlugin(p)
	if err == nil && running != nil {
		return running, err // already running. no work here.
	}

	err = h.docker.PluginInstall(context.Background(), p.Name, types.PluginInstallOptions{
		AcceptAllPermissions: true,
		// TODO(chungers) -- there's a field RegistryAuth for authenticating with registry for private plugins...
		// for now assume plugins are public.
	})

	if err != nil {
		return nil, err
	}
	return h.DiscoverPlugin(p)
}

// RemovePlugin removes a running plugin by first disable and then remove it
func (h *Server) RemovePlugin(p PluginRef) error {
	running, err := h.DiscoverPlugin(p.Plugin)
	if err != nil {
		return err
	}
	if running == nil {
		return fmt.Errorf("plugin not running", p)
	}

	if running.Socket != p.Socket {
		return fmt.Errorf("mismatch socket / instance", running.Socket, "vs", p.Socket, "plugin=", p)
	}

	ctx := context.Background()

	log.Infoln("Disable plugin", p)
	err = h.docker.PluginDisable(ctx, p.Name)
	if err != nil {
		return err
	}

	log.Infoln("Now removing plugin", p)
	return h.docker.PluginRemove(ctx, p.Name, types.PluginRemoveOptions{})
}

// CallPlugin calls the plugin specified by the input.  The socket field is not nil.
// The input is the output of the discover with op and arg filled in.
func (h *Server) CallPlugin(call PluginCall) (interface{}, error) {
	if call.PluginRef.Socket == "" {
		return nil, fmt.Errorf("no-socket")
	}
	// now connect -- this assumes the volume is bind mounted...
	client := controller.NewClient(call.PluginRef.Socket)

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
