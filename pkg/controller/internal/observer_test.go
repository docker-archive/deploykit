package internal

import (
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/spi/instance"
	testutil_instance "github.com/docker/infrakit/pkg/testing/instance"
	testutil_scope "github.com/docker/infrakit/pkg/testing/scope"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestInstanceObserver(t *testing.T) {

	observer := new(InstanceObserver)
	err := types.Decode([]byte(`
plugin: simulator/compute
select:
  group: workers
  type: large
observeinterval: 2s
KeySelector: \{\{.link\}\}
`), observer)

	require.NoError(t, err)
	require.Equal(t, 2*time.Second, observer.ObserveInterval.Duration())
	require.Equal(t, `\{\{.link\}\}`, observer.KeySelector)
	require.Equal(t, map[string]string{
		"group": "workers",
		"type":  "large",
	}, observer.Select)

	lookup := make(chan string, 10)

	expected := []instance.Description{
		{ID: "id1", Properties: types.AnyValueMust(map[string]string{"link": "link1"})},
		{ID: "id2", Properties: types.AnyValueMust(map[string]string{"link": "link2"})},
		{ID: "id3", Properties: types.AnyValueMust(map[string]string{"link": "link3"})},
		{ID: "id4", Properties: types.AnyValueMust(map[string]string{"link": "link4"})},
	}
	called := make(chan []interface{}, 10)

	testInstancePlugin := &testutil_instance.Plugin{
		DoDescribeInstances: func(tags map[string]string, details bool) ([]instance.Description, error) {
			called <- []interface{}{tags, details}
			return expected, nil
		},
	}

	testScope := testutil_scope.DefaultScope()
	testScope.ResolveInstance = func(n string) (instance.Plugin, error) {
		lookup <- n
		return testInstancePlugin, nil
	}

	err = observer.Init(testScope, 1*time.Second)
	require.NoError(t, err)

	observer.Start()

	observer.Pause(false)
	observer.Pause(true)
	observer.Pause(false)

	require.Equal(t, "simulator/compute", <-lookup)
	require.Equal(t, []interface{}{observer.Select, true}, <-called)

	var seen []instance.Description

	count := 0
	for samples := range observer.Observations() {
		seen = samples
		require.Equal(t, expected, seen)
		count++

		if count == 1 {
			observer.Stop()
		}
	}

	close(lookup)
	close(called)
}

func TestInstanceObserverMultipleSamples(t *testing.T) {

	observer := new(InstanceObserver)
	err := types.Decode([]byte(`
plugin: simulator/compute
select:
  group: workers
  type: large
observeinterval: 2s
KeySelector: \{\{.link\}\}
`), observer)

	require.NoError(t, err)
	require.Equal(t, 2*time.Second, observer.ObserveInterval.Duration())
	require.Equal(t, `\{\{.link\}\}`, observer.KeySelector)
	require.Equal(t, map[string]string{
		"group": "workers",
		"type":  "large",
	}, observer.Select)

	// 2 samples with different values... link2 and link4 disappears
	expected := [][]instance.Description{
		{
			{ID: "id1", Properties: types.AnyValueMust(map[string]string{"link": "link1"})},
			{ID: "id2", Properties: types.AnyValueMust(map[string]string{"link": "link2"})},
			{ID: "id3", Properties: types.AnyValueMust(map[string]string{"link": "link3"})},
			{ID: "id4", Properties: types.AnyValueMust(map[string]string{"link": "link4"})},
		},
		{
			{ID: "id1", Properties: types.AnyValueMust(map[string]string{"link": "link1"})},
			{ID: "id3", Properties: types.AnyValueMust(map[string]string{"link": "link3"})},
		},
	}

	call := 0
	testInstancePlugin := &testutil_instance.Plugin{
		DoDescribeInstances: func(tags map[string]string, details bool) ([]instance.Description, error) {
			defer func() { call++ }()
			if call == 0 {
				return expected[0], nil
			}

			return expected[1], nil
		},
	}

	testScope := testutil_scope.DefaultScope()
	testScope.ResolveInstance = func(n string) (instance.Plugin, error) {
		return testInstancePlugin, nil
	}

	err = observer.Init(testScope, 1*time.Second)
	require.NoError(t, err)

	observer.Start()

	observer.Pause(false)
	observer.Pause(true)

	<-time.After(3 * time.Second)

	observer.Pause(false)

	var seen []instance.Description

	// Take 2 samples and compute the difference in each iteration
	count := 0
loop:
	for samples := range observer.Observations() {

		diff := observer.Difference(seen, samples)
		seen = samples

		switch count {
		case 0:
			require.Equal(t, types.AnyValueMust([]instance.Description{}), types.AnyValueMust(diff))
		case 1:
			require.Equal(t, types.AnyValueMust([]instance.Description{
				{ID: "id2", Properties: types.AnyValueMust(map[string]string{"link": "link2"})},
				{ID: "id4", Properties: types.AnyValueMust(map[string]string{"link": "link4"})},
			}), types.AnyValueMust(diff))
		default:
			observer.Stop()
			break loop
		}
		count++
	}
	require.True(t, count > 1)

	itr := 0
loop2:
	for lost := range observer.Lost() {

		switch itr {
		case 0:
			require.Equal(t, types.AnyValueMust([]instance.Description{}), types.AnyValueMust(lost))
		case 1:
			require.Equal(t, types.AnyValueMust([]instance.Description{
				{ID: "id2", Properties: types.AnyValueMust(map[string]string{"link": "link2"})},
				{ID: "id4", Properties: types.AnyValueMust(map[string]string{"link": "link4"})},
			}), types.AnyValueMust(lost))
		default:
			observer.Stop()
			break loop2
		}
		itr++
	}
	require.True(t, itr > 1)

}
