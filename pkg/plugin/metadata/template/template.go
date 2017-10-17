package template

import (
	"fmt"
	gopath "path"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/docker/infrakit/pkg/rpc"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// MetadataFunc returns a template function to support metadata retrieval in templates.
// The function allows an additional parameter to set the value, provided the metadata plugin
// is not readonly (supports the metadata.Updatable spi).  In the case of an update, the returned value
// is always an empty string, with error (can be nil).  The behavior is the same as var function.
func MetadataFunc(discovery func() discovery.Plugins) func(string, ...interface{}) (interface{}, error) {

	plugins := discovery

	return func(path string, optionalValue ...interface{}) (interface{}, error) {

		switch len(optionalValue) {
		case 0: // read
			return doGetSet(plugins, path, optionalValue...)
		case 1: // set
			return doGetSet(plugins, path, optionalValue...)
		case 2: // a retry time + timeout is specified for a read
			retry, err := duration(optionalValue[0])
			if err != nil {
				return nil, err
			}
			timeout, err := duration(optionalValue[1])
			if err != nil {
				return nil, err
			}
			return doBlockingGet(plugins, path, retry, timeout)
		case 3: // a retry time + timeout is specified for a read + bool to return error
			retry, err := duration(optionalValue[0])
			if err != nil {
				return nil, err
			}
			timeout, err := duration(optionalValue[1])
			if err != nil {
				return nil, err
			}
			errOnTimeout, is := optionalValue[2].(bool)
			if !is {
				return nil, fmt.Errorf("must be boolean %v", optionalValue[2])
			}
			v, err := doBlockingGet(plugins, path, retry, timeout)
			if errOnTimeout {
				return v, err
			}
			return v, nil
		}
		return template.VoidValue, fmt.Errorf("wrong number of args")
	}
}

func duration(v interface{}) (time.Duration, error) {
	switch v := v.(type) {
	case time.Duration:
		return v, nil
	case types.Duration:
		return v.Duration(), nil
	case []byte:
		return time.ParseDuration(string(v))
	case string:
		return time.ParseDuration(string(v))
	case int64:
		return time.Duration(int64(v)), nil
	case uint:
		return time.Duration(int64(v)), nil
	case uint64:
		return time.Duration(int64(v)), nil
	case int:
		return time.Duration(int64(v)), nil
	}
	return 0, fmt.Errorf("cannot convert to duration: %v", v)
}

// blocking get from metadata. block the same go routine / thread until timeout or value is available
func doBlockingGet(plugins func() discovery.Plugins, path string, retry, timeout time.Duration) (interface{}, error) {

	if plugins == nil {
		return nil, fmt.Errorf("no plugin discovery:%s", path)
	}

	mpath := types.PathFromString(path)
	first := mpath.Index(0)
	if first == nil {
		return nil, fmt.Errorf("unknown plugin from path: %s", path)
	}

	lookup, err := plugins().List()
	endpoint, has := lookup[*first]
	if !has {
		return false, nil // Don't return error.  Just return false for non-existence
	} else if mpath.Len() == 1 {
		return true, nil // This is a test for availability of the plugin
	}

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

	subs, has := info[rpc.InterfaceSpec(metadata.InterfaceSpec.Encode())]
	sub := mpath.Shift(1).Index(0)
	if has && sub != nil {
		for _, ss := range subs {
			if *sub == ss {
				pluginName = plugin.Name(gopath.Join(*first, *sub))
				key = key.Shift(1)
			}
		}
	}

	// now we have the plugin name -- try to get the interface
	// note - that there are two different rpc interfaces
	// TODO - consider eliminating and use only one
	metadataPlugin, err := metadata_rpc.FromHandshaker(pluginName, handshaker)
	if err != nil {
		return nil, err
	}

	var value interface{}
	expiry := time.Now().Add(timeout)

	for i := 0; ; i++ {

		any, err := metadataPlugin.Get(key)
		if err == nil && any != nil {
			err = any.Decode(&value)
			if err != nil {
				return any.String(), err // note the type changed to string in error return
			}
			return value, err
		}

		if i > 0 && time.Now().After(expiry) {
			break
		}

		if retry > 0 {
			<-time.After(retry)
		}
	}
	return value, fmt.Errorf("expired waiting")
}

func doGetSet(plugins func() discovery.Plugins, path string, optionalValue ...interface{}) (interface{}, error) {
	if plugins == nil {
		return nil, fmt.Errorf("no plugin discovery:%s", path)
	}

	mpath := types.PathFromString(path)
	first := mpath.Index(0)
	if first == nil {
		return nil, fmt.Errorf("unknown plugin from path: %s", path)
	}

	lookup, err := plugins().List()
	endpoint, has := lookup[*first]
	if !has {
		return false, nil // Don't return error.  Just return false for non-existence
	} else if mpath.Len() == 1 {
		return true, nil // This is a test for availability of the plugin
	}

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

	checks := []rpc.InterfaceSpec{
		rpc.InterfaceSpec(metadata.UpdatableInterfaceSpec.Encode()),
		rpc.InterfaceSpec(metadata.InterfaceSpec.Encode()),
	}

	for _, c := range checks {
		subs, has := info[c]
		sub := mpath.Shift(1).Index(0)
		if has && sub != nil {
			for _, ss := range subs {
				if *sub == ss {
					pluginName = plugin.Name(gopath.Join(*first, *sub))
					key = key.Shift(1)
					break
				}
			}
		}
	}

	metadataPlugin, err := metadata_rpc.NewClient(pluginName, endpoint.Address)
	if err != nil {
		return nil, err
	}

	//key := mpath.Shift(1)
	var value interface{}
	any, err := metadataPlugin.Get(key)
	if err != nil {
		return nil, err
	}

	if any != nil {
		err = any.Decode(&value)
		if err != nil {
			return any.String(), err // note the type changed to string in error return
		}
	}

	// Update case: return value is the version before this successful commit.
	if len(optionalValue) == 1 {

		any, err := types.AnyValue(optionalValue[0])
		if err != nil {
			return value, err
		}

		// update it
		updatablePlugin, is := metadataPlugin.(metadata.Updatable)
		if !is {
			return value, fmt.Errorf("value is read-only")
		}
		_, proposed, cas, err := updatablePlugin.Changes([]metadata.Change{
			{
				Path:  key,
				Value: any,
			},
		})
		err = updatablePlugin.Commit(proposed, cas)
		return template.VoidValue, err
	}

	return value, err
}
