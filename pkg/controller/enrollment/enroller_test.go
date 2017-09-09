package enrollment

import (
	"fmt"
	"testing"

	enrollment "github.com/docker/infrakit/pkg/controller/enrollment/types"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	"github.com/stretchr/testify/require"
)

type fakeLeader func() (bool, error)

func (f fakeLeader) IsLeader() (bool, error) {
	return f()
}

type fakePlugins map[string]*plugin.Endpoint

func (f fakePlugins) Find(name plugin.Name) (*plugin.Endpoint, error) {
	lookup, _ := name.GetLookupAndType()
	if v, has := f[lookup]; has {
		return v, nil
	}
	return nil, fmt.Errorf("not found")
}

func (f fakePlugins) List() (map[string]*plugin.Endpoint, error) {
	return (map[string]*plugin.Endpoint)(f), nil
}

func TestEnroller(t *testing.T) {

	enroller := newEnroller(
		func() discovery.Plugins {
			return fakePlugins{
				"test": &plugin.Endpoint{},
			}
		},
		fakeLeader(func() (bool, error) { return false, nil }),
		enrollment.Options{})

	require.False(t, enroller.Running())
}
