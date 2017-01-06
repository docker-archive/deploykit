package manager

import (
	"errors"
	"testing"

	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/stretchr/testify/require"
)

func TestErrorOnCallsToNilPlugin(t *testing.T) {

	errMessage := "no-plugin"
	proxy := NewProxy(func() (group.Plugin, error) {
		return nil, errors.New(errMessage)
	})

	err := proxy.FreeGroup(group.ID("test"))
	require.Error(t, err)
	require.Equal(t, errMessage, err.Error())
}

type fakeGroupPlugin struct {
	group.Plugin
	commitGroup func(grp group.Spec, pretend bool) (string, error)
	freeGroup   func(id group.ID) error
}

func (f *fakeGroupPlugin) CommitGroup(grp group.Spec, pretend bool) (string, error) {
	return f.commitGroup(grp, pretend)
}
func (f *fakeGroupPlugin) FreeGroup(id group.ID) error {
	return f.freeGroup(id)
}

func TestDelayPluginLookupCallingMethod(t *testing.T) {

	called := false
	fake := &fakeGroupPlugin{
		commitGroup: func(grp group.Spec, pretend bool) (string, error) {
			called = true
			require.Equal(t, group.Spec{ID: "foo"}, grp)
			require.Equal(t, true, pretend)
			return "some-response", nil
		},
	}

	proxy := NewProxy(func() (group.Plugin, error) { return fake, nil })

	require.False(t, called)

	actualStr, actualErr := proxy.CommitGroup(group.Spec{ID: "foo"}, true)
	require.True(t, called)
	require.NoError(t, actualErr)
	require.Equal(t, "some-response", actualStr)
}

func TestDelayPluginLookupCallingMethodReturnsError(t *testing.T) {

	called := false
	fake := &fakeGroupPlugin{
		freeGroup: func(id group.ID) error {
			called = true
			require.Equal(t, group.ID("foo"), id)
			return errors.New("can't-free")
		},
	}

	proxy := NewProxy(func() (group.Plugin, error) { return fake, nil })

	require.False(t, called)

	actualErr := proxy.FreeGroup(group.ID("foo"))
	require.True(t, called)
	require.Error(t, actualErr)
	require.Equal(t, "can't-free", actualErr.Error())
}

func TestDelayPluginLookupCallingMultipleMethods(t *testing.T) {

	called := false
	fake := &fakeGroupPlugin{
		commitGroup: func(grp group.Spec, pretend bool) (string, error) {
			called = true
			require.Equal(t, group.Spec{ID: "foo"}, grp)
			require.Equal(t, true, pretend)
			return "some-response", nil
		},
		freeGroup: func(id group.ID) error {
			called = true
			require.Equal(t, group.ID("foo"), id)
			return errors.New("can't-free")
		},
	}

	proxy := NewProxy(func() (group.Plugin, error) { return fake, nil })

	require.False(t, called)

	actualStr, actualErr := proxy.CommitGroup(group.Spec{ID: "foo"}, true)
	require.True(t, called)
	require.NoError(t, actualErr)
	require.Equal(t, "some-response", actualStr)

	called = false
	actualErr = proxy.FreeGroup(group.ID("foo"))
	require.True(t, called)
	require.Error(t, actualErr)
	require.Equal(t, "can't-free", actualErr.Error())
}
