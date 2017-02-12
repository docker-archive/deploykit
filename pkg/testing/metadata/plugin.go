package metadata

import (
	"github.com/docker/infrakit/pkg/types"
)

// Plugin implements metadata.Plugin
type Plugin struct {

	// DoList implements List via function
	DoList func(path []string) (child []string, err error)

	// DoGet implements Get via function
	DoGet func(path []string) (value *types.Any, err error)
}

// List lists the child nodes under path
func (t *Plugin) List(path []string) (child []string, err error) {
	return t.DoList(path)
}

// Get gets the value
func (t *Plugin) Get(path []string) (value *types.Any, err error) {
	return t.DoGet(path)
}
