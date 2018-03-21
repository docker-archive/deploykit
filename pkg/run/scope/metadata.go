package scope

import (
	"fmt"
	gopath "path"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	rpc_client "github.com/docker/infrakit/pkg/rpc/client"
	metadata_rpc "github.com/docker/infrakit/pkg/rpc/metadata"
	"github.com/docker/infrakit/pkg/spi"
	"github.com/docker/infrakit/pkg/spi/metadata"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
)

// Metadata implements resolution of path to metadata
func (f fullScope) Metadata(path string) (*MetadataCall, error) {
	return DefaultMetadataResolver(f)(path)
}

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
	info, err := handshaker.Hello()
	if err != nil {
		return nil, err
	}

	// Need to derive the fully qualified plugin name from a long path
	pluginName := plugin.Name(*first)
	key := mpath.Shift(1)

	// There are two interfaces possible so we need to search for both
	for _, c := range []spi.InterfaceSpec{
		metadata.UpdatableInterfaceSpec,
		metadata.InterfaceSpec,
	} {
		subs, has := info[c]
		sub := mpath.Shift(1).Index(0)
		if has && sub != nil {
			for _, o := range subs {
				ss := o.Name
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

func clean(args ...interface{}) []interface{} {
	// HACK - for some reason, with playbooks that
	// are evaluated twice, an expression like this
	//
	// {{ $project := metadata `vars/project` }}
	// will appear with an optional args of [ "" ].
	// This causes problems in that it looks like setting
	// the vallue to empty string.
	// So this function needs to catch that case and sanitize
	// the input to the metadata function.

	if len(args) > 1 {
		return args
	}

	out := []interface{}{}
	for i, a := range args {
		switch a := a.(type) {
		case string:
			if a == "" && i == 0 {
				continue
			}
		case *string:
			if a != nil && *a == "" && i == 0 {
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

// MetadataFunc returns a template function to support metadata retrieval in templates.
// The function allows an additional parameter to set the value, provided the metadata plugin
// is not readonly (supports the metadata.Updatable spi).  In the case of an update, the returned value
// is always an empty string, with error (can be nil).  The behavior is the same as var function.
func MetadataFunc(scope Scope) func(string, ...interface{}) (interface{}, error) {

	return func(path string, args ...interface{}) (interface{}, error) {

		optionalValue := clean(args...)

		switch len(optionalValue) {
		case 0: // read
			return doBlockingGet(scope.Metadata, path, 0, 0)
		case 1: // set
			return doSet(scope.Metadata, path, optionalValue[0])
		case 2: // a retry time + timeout is specified for a read
			retry, err := duration(optionalValue[0])
			if err != nil {
				return nil, err
			}
			timeout, err := duration(optionalValue[1])
			if err != nil {
				return nil, err
			}
			return doBlockingGet(scope.Metadata, path, retry, timeout)
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
			v, err := doBlockingGet(scope.Metadata, path, retry, timeout)
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

type errExpired string

func (err errExpired) Error() string {
	return fmt.Sprintf("expired key:%v", string(err))
}

// IsExpired returns true if the error is from wait expired / timeout
func IsExpired(err error) bool {
	_, is := err.(errExpired)
	return is
}

type errReadonly string

func (err errReadonly) Error() string {
	return fmt.Sprintf("readonly:%v", string(err))
}

// IsReadonly returns true if the error is from wait expired / timeout
func IsReadonly(err error) bool {
	_, is := err.(errReadonly)
	return is
}

type metadataResolver func(p string) (*MetadataCall, error)

// blocking get from metadata. block the same go routine / thread until timeout or value is available
func doBlockingGet(resolver metadataResolver, path string, retry, timeout time.Duration) (interface{}, error) {

	call, err := resolver(path)
	if err != nil {
		return nil, err
	}

	if call == nil {
		return nil, nil
	}

	// If the key is nil, the query (path) was for existence of the plugin itself
	if types.NullPath.Equal(call.Key) {
		return true, nil
	}

	var value interface{}
	expiry := time.Now().Add(timeout)

	for i := 0; ; i++ {

		var any *types.Any
		any, err = call.Plugin.Get(call.Key)
		if err == nil && any != nil {
			err = any.Decode(&value)
			if err != nil {
				return any.String(), err // note the type changed to string in error return
			}
			return value, err
		}

		if i > 0 && time.Now().After(expiry) {
			err = errExpired(call.Key.String())
			break
		}

		if retry > 0 {
			<-time.After(retry)
		} else {
			break
		}
	}
	return value, err
}

func doSet(resolver metadataResolver, path string, newValue interface{}) (interface{}, error) {

	call, err := resolver(path)
	if err != nil {
		return nil, err
	}

	if call == nil {
		return nil, nil
	}

	// If the key is nil, the query (path) was for existence of the plugin itself
	if types.NullPath.Equal(call.Key) {
		return true, nil
	}

	var value interface{}
	any, err := call.Plugin.Get(call.Key)
	if err != nil {
		return nil, err
	}

	if any != nil {
		err = any.Decode(&value)
		if err != nil {
			return any.String(), err // note the type changed to string in error return
		}
	}
	any, err = types.AnyValue(newValue)
	if err != nil {
		return value, err
	}

	updatablePlugin, is := call.Plugin.(metadata.Updatable)
	if !is {
		return value, errReadonly(path)
	}
	_, proposed, cas, err := updatablePlugin.Changes([]metadata.Change{
		{
			Path:  call.Key,
			Value: any,
		},
	})
	err = updatablePlugin.Commit(proposed, cas)
	return template.VoidValue, err
}
