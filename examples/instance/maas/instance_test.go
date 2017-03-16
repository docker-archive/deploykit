package main

import (
	"github.com/docker/infrakit/pkg/spi/instance"
	maas "github.com/juju/gomaasapi"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestProvision_and_Destroy(t *testing.T) {
	testServer := maas.NewTestServer("1.0")
	defer testServer.Close()
	dir, _ := os.Getwd()
	maasPlugin := NewMaasPlugin(dir, "", testServer.URL, "1.0")
	instanceSpec := instance.Spec{
		Tags: map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
		Init: "",
	}
	input := `{"system_id": "test", "hostname":"test"}`
	testServer.NewNode(input)

	id, err := maasPlugin.Provision(instanceSpec)
	require.NoError(t, err)

	list, err := maasPlugin.DescribeInstances(map[string]string{"label1": "value1"})
	require.NoError(t, err)
	require.Equal(t, []instance.Description{
		{
			ID: *id,
			Tags: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
		},
	}, list)
	err = maasPlugin.Label(*id, map[string]string{
		"label1": "value1",
		"label3": "changed",
	})
	require.NoError(t, err)

	list, err = maasPlugin.DescribeInstances(map[string]string{"label1": "value1"})
	require.NoError(t, err)
	require.Equal(t, []instance.Description{
		{
			ID: *id,
			Tags: map[string]string{
				"label1": "value1",
				"label3": "changed",
			},
		},
	}, list)

	list, err = maasPlugin.DescribeInstances(map[string]string{"label3": "changed"})
	require.NoError(t, err)
	require.Equal(t, []instance.Description{
		{
			ID: *id,
			Tags: map[string]string{
				"label1": "value1",
				"label3": "changed",
			},
		},
	}, list)

	err = maasPlugin.Destroy(*id)
	require.NoError(t, err)
}
