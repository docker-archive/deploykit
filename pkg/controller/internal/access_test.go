package internal

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/spi/instance"
	testutil_instance "github.com/docker/infrakit/pkg/testing/instance"
	testutil_scope "github.com/docker/infrakit/pkg/testing/scope"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestInstanceAccessMarshal(t *testing.T) {

	dict := map[string]*InstanceAccess{}

	access1 := new(InstanceAccess)
	err := types.Decode([]byte(`
plugin: simulator/compute
select:
  group: workers
  type: large
observeinterval: 2s
KeySelector: \{\{.link\}\}
tags:
  label: workers
init: apt-get install -y foo
properties:
  vcpu: 2
  mem: 16
  network: net1
`), access1)

	require.NoError(t, err)

	dict["test1"] = access1

	any, err := types.AnyValue(dict)
	require.NoError(t, err)

	dict2 := map[string]*InstanceAccess{}
	err = any.Decode(&dict2)

	require.NoError(t, err)
	require.Equal(t, access1.Spec.Init, dict2["test1"].Spec.Init)
}

func TestInstanceAccess(t *testing.T) {

	access := new(InstanceAccess)
	err := types.Decode([]byte(`
plugin: simulator/compute
select:
  group: workers
  type: large
observeinterval: 2s
KeySelector: \{\{.link\}\}
tags:
  label: workers
init: apt-get install -y foo
properties:
  vcpu: 2
  mem: 16
  network: net1
`), access)

	require.NoError(t, err)

	provisionSpec := instance.Spec{
		Tags: map[string]string{"label": "workers"},
		Init: "apt-get install -y foo",
		Properties: types.AnyValueMust(map[string]interface{}{
			"vcpu":    2,
			"mem":     16,
			"network": "net1",
		}),
	}

	require.Equal(t, types.AnyValueMust(provisionSpec), types.AnyValueMust(access.Spec))

	require.Equal(t, 2*time.Second, access.ObserveInterval.Duration())
	require.Equal(t, `\{\{.link\}\}`, access.KeySelector)
	require.Equal(t, map[string]string{
		"group": "workers",
		"type":  "large",
	}, access.Select)

	lookup := make(chan string, 10)

	expected := []instance.Description{
		{ID: "id1", Properties: types.AnyValueMust(map[string]string{"link": "link1"})},
		{ID: "id2", Properties: types.AnyValueMust(map[string]string{"link": "link2"})},
		{ID: "id3", Properties: types.AnyValueMust(map[string]string{"link": "link3"})},
		{ID: "id4", Properties: types.AnyValueMust(map[string]string{"link": "link4"})},
	}

	made := 0
	described := make(chan []interface{}, 1)
	provisioned := make(chan []interface{}, 1)
	testInstancePlugin := &testutil_instance.Plugin{
		DoDescribeInstances: func(tags map[string]string, details bool) ([]instance.Description, error) {
			described <- []interface{}{tags, details}
			return expected, nil
		},
		DoProvision: func(spec instance.Spec) (*instance.ID, error) {
			provisioned <- []interface{}{spec}
			made++
			id := instance.ID(fmt.Sprintf("%v", made))
			return &id, nil
		},
	}

	testScope := testutil_scope.DefaultScope()
	testScope.ResolveInstance = func(n string) (instance.Plugin, error) {
		lookup <- n
		return testInstancePlugin, nil
	}

	err = access.Init(testScope, 1*time.Second)
	require.NoError(t, err)

	require.NotNil(t, access.Plugin)

	access.Start()

	access.Pause(false)
	access.Pause(true)
	access.Pause(false)

	require.Equal(t, "simulator/compute", <-lookup)
	require.Equal(t, []interface{}{access.Select, true}, <-described)

	var seen []instance.Description

	count := 0
	for samples := range access.Observations() {
		seen = samples
		require.Equal(t, expected, seen)
		count++

		if count == 1 {
			access.Stop()
		}
	}

	// provision
	id, err := access.Provision(access.Spec)
	require.NoError(t, err)
	require.Equal(t, "1", string(*id))

	require.Equal(t, types.AnyValueMust(provisionSpec), types.AnyValueMust((<-provisioned)[0]))

	close(lookup)
	close(described)
	close(provisioned)
}
