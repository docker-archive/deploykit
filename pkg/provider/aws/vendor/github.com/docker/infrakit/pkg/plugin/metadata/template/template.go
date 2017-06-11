package template

import (
	"fmt"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/rpc/client"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// MetadataFunc returns a template function to support metadata retrieval in templates.
func MetadataFunc(discovery func() discovery.Plugins) func(string) (interface{}, error) {

	plugins := discovery

	return func(path string) (interface{}, error) {

		if plugins == nil {
			return nil, fmt.Errorf("no plugin discovery:%s", path)
		}

		mpath := types.PathFromString(path)
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
			return false, nil // Don't return error.  Just return false for non-existence
		} else if mpath.Len() == 1 {
			return true, nil // This is a test for availability of the plugin
		}

		rpcClient, err := client.New(endpoint.Address, metadata.InterfaceSpec)
		if err != nil {
			return nil, fmt.Errorf("cannot connect to plugin: %s", *first)
		}

		any, err := metadata_rpc.Adapt(rpcClient).Get(mpath.Shift(1))
		if err != nil {
			return nil, err
		}
		var value interface{}
		err = any.Decode(&value)
		if err != nil {
			return any.String(), err
		}
		return value, nil
	}
}
