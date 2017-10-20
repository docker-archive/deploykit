package group

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/stretchr/testify/require"
)

func TestLazyBlockAndCancel(t *testing.T) {

	called := make(chan struct{})
	g := LazyConnect(func() (group.Plugin, error) {
		close(called)
		return nil, fmt.Errorf("boom")
	}, 100*time.Second)

	errs := make(chan error, 1)
	go func() {
		_, err := g.DescribeGroup(group.ID("test"))
		errs <- err
		close(errs)
	}()

	<-called

	CancelWait(g)

	require.Equal(t, "cancelled", (<-errs).Error())
}

func TestLazyNoBlock(t *testing.T) {

	called := make(chan struct{})
	g := LazyConnect(func() (group.Plugin, error) {
		close(called)
		return nil, fmt.Errorf("boom")
	}, 0)

	errs := make(chan error, 1)
	go func() {
		_, err := g.DescribeGroup(group.ID("test"))
		errs <- err
		close(errs)
	}()

	<-called

	require.Equal(t, "boom", (<-errs).Error())
}

type fake chan []interface{}

func (f fake) CommitGroup(grp group.Spec, pretend bool) (string, error) {
	f <- []interface{}{group.Plugin.CommitGroup, grp, pretend}
	return "", nil
}

func (f fake) FreeGroup(id group.ID) error {
	f <- []interface{}{group.Plugin.FreeGroup, id}
	return nil
}

func (f fake) DescribeGroup(id group.ID) (group.Description, error) {
	f <- []interface{}{group.Plugin.DescribeGroup, id}
	return group.Description{}, nil
}

func (f fake) DestroyGroup(id group.ID) error {
	f <- []interface{}{group.Plugin.DestroyGroup, id}
	return nil
}

func (f fake) InspectGroups() ([]group.Spec, error) {
	f <- []interface{}{group.Plugin.InspectGroups}
	return nil, nil
}

func (f fake) DestroyInstances(id group.ID, sub []instance.ID) error {
	f <- []interface{}{group.Plugin.DestroyInstances, id, sub}
	return nil
}

func (f fake) Size(id group.ID) (int, error) {
	f <- []interface{}{group.Plugin.Size, id}
	return 100, nil
}

func (f fake) SetSize(id group.ID, sz int) error {
	f <- []interface{}{group.Plugin.SetSize, id, sz}
	return nil
}

func checkCalls(t *testing.T, ch chan []interface{}, args ...interface{}) {
	found := <-ch
	for i, a := range args {
		if reflect.ValueOf(a) != reflect.ValueOf(found[i]) {
			if a != found[i] {
				t.Fatal("Not equal:", found[i], "vs", a)
			}
		}
	}
}

func TestLazyNoBlockConnect(t *testing.T) {

	called := make(chan struct{})
	called2 := make(chan []interface{}, 2)

	g := LazyConnect(func() (group.Plugin, error) {
		close(called)
		return fake(called2), nil
	}, 0)

	errs := make(chan error, 1)
	go func() {
		_, err := g.DescribeGroup(group.ID("test"))
		errs <- err
		close(errs)

		g.Size(group.ID("test"))
		close(called2)
	}()

	<-called

	require.NoError(t, <-errs)
	checkCalls(t, called2, group.Plugin.DescribeGroup, group.ID("test"))
	checkCalls(t, called2, group.Plugin.Size, group.ID("test"))
}

func TestErrorOnCallsToNilPlugin(t *testing.T) {

	errMessage := "no-plugin"
	proxy := LazyConnect(func() (group.Plugin, error) {
		return nil, errors.New(errMessage)
	}, 0)

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

	proxy := LazyConnect(func() (group.Plugin, error) { return fake, nil }, 0)

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

	proxy := LazyConnect(func() (group.Plugin, error) { return fake, nil }, 0)

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

	proxy := LazyConnect(func() (group.Plugin, error) { return fake, nil }, 0)

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
