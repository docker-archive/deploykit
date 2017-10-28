package metadata

import (
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/plugin"
	rpc "github.com/docker/infrakit/pkg/rpc/client"
	"github.com/docker/infrakit/pkg/types"
)

var log = logutil.New("module", "rpc/controller")

func list(name plugin.Name, client rpc.Client, method string, path types.Path) ([]string, error) {
	req := KeysRequest{Name: name, Path: path}
	resp := KeysResponse{}
	err := client.Call(method, req, &resp)
	return resp.Nodes, err
}

func get(name plugin.Name, client rpc.Client, method string, path types.Path) (*types.Any, error) {
	req := GetRequest{Name: name, Path: path}
	resp := GetResponse{}
	err := client.Call(method, req, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Value, err
}
