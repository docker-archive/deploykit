package metadata

import (
	"github.com/docker/infrakit/pkg/types"
)

// Plugin implements metadata.Plugin
type Plugin struct {

	// DoList implements List via function
	DoList func(path types.Path) (child []string, err error)

	// DoGet implements Get via function
	DoGet func(path types.Path) (value *types.Any, err error)
}

// List lists the child nodes under path
func (t *Plugin) List(path types.Path) (child []string, err error) {
	return t.DoList(path)
}

// Get gets the value
func (t *Plugin) Get(path types.Path) (value *types.Any, err error) {
	return t.DoGet(path)
}
