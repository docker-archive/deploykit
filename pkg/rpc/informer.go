package rpc

import (
	"fmt"

	"github.com/docker/infrakit/pkg/plugin"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
)

// NewPluginInformer returns a plugin informer that can give metadata about a plugin
func NewPluginInformer(socketPath string) plugin.Informer {
	return &informer{client: rpc_client.New(socketPath)}
}

type informer struct {
	client rpc_client.Client
}

// GetMeta implements the Informer interface and returns the metadata about the plugin
func (i informer) GetMeta() (plugin.Meta, error) {
	req := plugin.EmptyRequest{}
	resp := plugin.Meta{}
	err := i.client.Call("Plugin.Meta", req, &resp)

	fmt.Println(">>>>>===", err)

	if err != nil {
		return resp, err
	}
	return resp, nil
}
