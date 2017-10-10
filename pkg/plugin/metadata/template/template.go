package template

import (
	"fmt"

	"github.com/docker/infrakit/pkg/discovery"
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

		metadataPlugin, err := metadata_rpc.NewClient(endpoint.Address)
		if err != nil {
			return nil, fmt.Errorf("cannot connect to plugin: %s", *first)
		}

		key := mpath.Shift(1)
		var value interface{}
		any, err := metadataPlugin.Get(key)
		if err != nil {
			return nil, err
		}

		err = any.Decode(&value)
		if err != nil {
			return any.String(), err // note the type changed to string in error return
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
}
