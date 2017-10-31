package scope

import (
	"fmt"
	gopath "path"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/types"
)

// DefaultMetadataResolver returns a resolver
func DefaultMetadataResolver(plugins func() discovery.Plugins) func(string) (*MetadataCall, error) {
	return func(path string) (*MetadataCall, error) {
		return metadataPlugin(plugins, types.PathFromString(path))
	}
}

func metadataPlugin(plugins func() discovery.Plugins, mpath types.Path) (*MetadataCall, error) {

	if plugins == nil {
		return nil, fmt.Errorf("no plugin discovery:%s", mpath.String())
	}

	first := mpath.Index(0)
	if first == nil {
		return nil, fmt.Errorf("unknown plugin from path: %s", mpath.String())
	}

	lookup, err := plugins().List()
	endpoint, has := lookup[*first]
	if !has {

		return nil, nil // Don't return error.  Just return nil for non-existence

	} else if mpath.Len() == 1 {

		// This is a test for availability of the plugin
		name := plugin.Name(*first)
		pl, err := metadata_rpc.NewClient(name, endpoint.Address)
		if err != nil {
			return nil, err
		}

		return &MetadataCall{
			Name:   name,
			Plugin: pl,
			Key:    mpath.Shift(1),
		}, nil

	}

	// Longer, full path lookup

	handshaker, err := rpc_client.NewHandshaker(endpoint.Address)
	if err != nil {
		return nil, err
	}
	// we need to get the subtypes
	info, err := handshaker.Types()
	if err != nil {
		return nil, err
	}

	// Need to derive the fully qualified plugin name from a long path
	pluginName := plugin.Name(*first)
	key := mpath.Shift(1)

	// There are two interfaces possible so we need to search for both
	for _, c := range []rpc.InterfaceSpec{
		rpc.InterfaceSpec(metadata.UpdatableInterfaceSpec.Encode()),
		rpc.InterfaceSpec(metadata.InterfaceSpec.Encode()),
	} {
		subs, has := info[c]
		sub := mpath.Shift(1).Index(0)
		if has && sub != nil {
			for _, ss := range subs {
				if *sub == ss {
					pluginName = plugin.Name(gopath.Join(*first, *sub))
					key = key.Shift(1)
				}
			}
		}

	}

	// now we have the plugin name -- try to get the interface
	// note - that there are two different rpc interfaces
	// TODO - consider eliminating and use only one
	pl, err := metadata_rpc.FromHandshaker(pluginName, handshaker)
	if err != nil {
		return nil, err
	}
	return &MetadataCall{Name: pluginName, Key: key, Plugin: pl}, nil
}
