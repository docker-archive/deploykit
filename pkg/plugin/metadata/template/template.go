package template

import (
	"fmt"

	"github.com/docker/infrakit/pkg/discovery"
	metadata_plugin "github.com/docker/infrakit/pkg/plugin/metadata"
	"github.com/docker/infrakit/pkg/rpc/client"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/metadata"
)

// MetadataFunc returns a template function to support metadata retrieval in templates.
func MetadataFunc(plugins func() discovery.Plugins) func(string) (interface{}, error) {
	return func(path string) (interface{}, error) {

		mpath := metadata_plugin.Path(path)
		first := mpath.Index(0)
		if first == nil {
			return nil, fmt.Errorf("unknown plugin from path: %s", path)
		}

		lookup, err := plugins().List()
		if err != nil {
			return nil, err
		}

		endpoint, has := lookup[*first]
		if !has {
			return nil, fmt.Errorf("plugin: %s not found", *first)
		}

		rpcClient, err := client.New(endpoint.Address, metadata.InterfaceSpec)
		if err != nil {
			return nil, fmt.Errorf("cannot connect to plugin: %s", *first)
		}

		return metadata_rpc.Adapt(rpcClient).Get(mpath.Shift(1))
	}
}
