package client

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path"

	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	"github.com/docker/infrakit/pkg/template"
)

// NewPluginInfoClient returns a plugin informer that can give metadata about a plugin
func NewPluginInfoClient(address string) (*InfoClient, error) {
	u, httpC, err := parseAddress(address)
	if err != nil {
		return nil, err
	}
	return &InfoClient{addr: address, client: httpC, url: u}, nil
}

// InfoClient is the client for retrieving plugin info
type InfoClient struct {
	client *http.Client
	addr   string
	url    *url.URL
}

// GetInfo implements the Info interface and returns the metadata about the plugin
func (i *InfoClient) GetInfo() (plugin.Info, error) {
	meta := plugin.Info{}

	dest := *i.url
	dest.Path = path.Clean(path.Join(i.url.Path, rpc.URLAPI))

	resp, err := i.client.Get(dest.String())
	if err != nil {
		return meta, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&meta)
	return meta, err
}

// GetFunctions returns metadata about the plugin's template functions, if the plugin supports templating.
func (i *InfoClient) GetFunctions() (map[string][]template.Function, error) {
	meta := map[string][]template.Function{}

	dest := *i.url
	dest.Path = path.Clean(path.Join(i.url.Path, rpc.URLFunctions))

	resp, err := i.client.Get(dest.String())
	if err != nil {
		return meta, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&meta)
	return meta, err
}
