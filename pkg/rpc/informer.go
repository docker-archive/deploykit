package rpc

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/docker/infrakit/pkg/plugin"
)

const (
	MetaURL = "/metaz"
)

// NewPluginInformer returns a plugin informer that can give metadata about a plugin
func NewPluginInformer(socketPath string) plugin.Informer {
	dialUnix := func(proto, addr string) (conn net.Conn, err error) {
		return net.Dial("unix", socketPath)
	}
	return &informer{client: &http.Client{Transport: &http.Transport{Dial: dialUnix}}}
}

type informer struct {
	client *http.Client
}

// GetMeta implements the Informer interface and returns the metadata about the plugin
func (i informer) GetMeta() (plugin.Meta, error) {
	meta := plugin.Meta{}
	resp, err := i.client.Get(MetaURL)
	if err != nil {
		return meta, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&meta)
	return meta, err
}
