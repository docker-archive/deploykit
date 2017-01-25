package client

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
)

// NewPluginInfoClient returns a plugin informer that can give metadata about a plugin
func NewPluginInfoClient(socketPath string) *InfoClient {
	dialUnix := func(proto, addr string) (conn net.Conn, err error) {
		return net.Dial("unix", socketPath)
	}
	return &InfoClient{client: &http.Client{Transport: &http.Transport{Dial: dialUnix}}}
}

// InfoClient is the client for retrieving plugin info
type InfoClient struct {
	client *http.Client
}

// GetInfo implements the Info interface and returns the metadata about the plugin
func (i *InfoClient) GetInfo() (plugin.Info, error) {
	meta := plugin.Info{}
	resp, err := i.client.Get("http://d" + rpc.InfoURL)
	if err != nil {
		return meta, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&meta)
	return meta, err
}
