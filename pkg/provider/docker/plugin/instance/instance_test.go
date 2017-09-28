package instance

import (
	"fmt"
	"testing"

	"github.com/docker/docker/client"
	"github.com/docker/infrakit/pkg/spi/instance"
	testutil "github.com/docker/infrakit/pkg/testing"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

var (
	testNamespace = map[string]string{"cluster": "test", "type": "testing"}
	tags          = map[string]string{"group": "workers"}
)

func TestInstanceLifecycle(t *testing.T) {
	if testutil.SkipTests("docker") {
		t.SkipNow()
	}
	defaultHeaders := map[string]string{"User-Agent": "InfraKit"}
	cli, err := client.NewClient("unix:///var/run/docker.sock", "1.25", nil, defaultHeaders)
	require.NoError(t, err)

	pluginImpl := dockerInstancePlugin{client: *cli, namespaceTags: testNamespace}
	id, err := pluginImpl.Provision(instance.Spec{Properties: inputJSON, Tags: tags, Init: initScript})
	require.NoError(t, err)
	require.NotNil(t, id)

	require.NoError(t, pluginImpl.Destroy(instance.ID(*id), instance.Termination))
}

func TestCreateInstanceError(t *testing.T) {
	if testutil.SkipTests("docker") {
		t.SkipNow()
	}
	defaultHeaders := map[string]string{"User-Agent": "InfraKit"}
	cli, err := client.NewClient("unix:///var/run/docker.sock", "1.25", nil, defaultHeaders)
	require.NoError(t, err)

	pluginImpl := NewInstancePlugin(cli, map[string]string{"cluster": "test"})
	properties := types.AnyString("{}")
	id, err := pluginImpl.Provision(instance.Spec{Properties: properties, Tags: tags})

	require.Error(t, err)
	require.Nil(t, id)
}

func TestDestroyInstanceError(t *testing.T) {
	if testutil.SkipTests("docker") {
		t.SkipNow()
	}
	defaultHeaders := map[string]string{"User-Agent": "InfraKit"}
	cli, err := client.NewClient("unix:///var/run/docker.sock", "1.25", nil, defaultHeaders)
	require.NoError(t, err)

	instanceID := "test-id"

	pluginImpl := NewInstancePlugin(cli, testNamespace)
	require.Error(t, pluginImpl.Destroy(instance.ID(instanceID), instance.Termination))
}

func TestDescribeInstancesRequest(t *testing.T) {
	if testutil.SkipTests("docker") {
		t.SkipNow()
	}
	request := describeGroupRequest(testNamespace, tags)

	require.NotNil(t, request)

	requireFilter := func(name, value string) {
		if request.Filters.Match(name, value) {
			return
		}
		require.Fail(t, fmt.Sprintf("Did not have filter %s=%s", name, value))
	}
	for key, value := range tags {
		requireFilter(key, value)
	}
	for key, value := range testNamespace {
		requireFilter(key, value)
	}
}

func TestListGroup(t *testing.T) {
	if testutil.SkipTests("docker") {
		t.SkipNow()
	}
	defaultHeaders := map[string]string{"User-Agent": "InfraKit"}
	cli, err := client.NewClient("unix:///var/run/docker.sock", "1.25", nil, defaultHeaders)
	require.NoError(t, err)

	pluginImpl := NewInstancePlugin(cli, testNamespace)
	_, err = pluginImpl.DescribeInstances(tags, false)

	require.NoError(t, err)
}

var inputJSON = types.AnyString(`{
    "Tags": {"test": "docker-create-test"},
    "Config": {
        "Image": "alpine:3.5",
        "Cmd": ["nc", "-l", "-p", "8080"],
        "Env": [ "var1=value1", "var2=value2" ]
    },
    "NetworkAttachments": [
        {
            "Name": "deleteme",
            "Driver": "overlay"
        }
    ]
}
`)
var initScript = "#!/bin/sh\ntouch /tmp/touched"
