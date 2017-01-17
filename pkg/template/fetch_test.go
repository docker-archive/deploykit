package template

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"

	rpc "github.com/docker/infrakit/pkg/rpc/instance"
	rpc_server "github.com/docker/infrakit/pkg/rpc/server"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/stretchr/testify/require"
)

type testPlugin struct {
	// Validate performs local validation on a provision request.
	DoValidate func(req json.RawMessage) error

	// Provision creates a new instance based on the spec.
	DoProvision func(spec instance.Spec) (*instance.ID, error)

	// Destroy terminates an existing instance.
	DoDestroy func(instance instance.ID) error

	// DescribeInstances returns descriptions of all instances matching all of the provided tags.
	DoDescribeInstances func(tags map[string]string) ([]instance.Description, error)
}

func (t *testPlugin) Validate(req json.RawMessage) error {
	return t.DoValidate(req)
}
func (t *testPlugin) Provision(spec instance.Spec) (*instance.ID, error) {
	return t.DoProvision(spec)
}
func (t *testPlugin) Destroy(instance instance.ID) error {
	return t.DoDestroy(instance)
}
func (t *testPlugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	return t.DoDescribeInstances(tags)
}

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return path.Join(dir, "instance-impl-test")
}

func TestFetchSocket(t *testing.T) {
	socketPath := tempSocket()
	dir := filepath.Dir(socketPath)
	host := filepath.Base(socketPath)

	url := "unix://" + host + "/info/api.json"

	server, err := rpc_server.StartPluginAtPath(socketPath, rpc.PluginServer(&testPlugin{}))
	require.NoError(t, err)

	buff, err := fetch(url, Options{SocketDir: dir})
	require.NoError(t, err)

	decoded, err := FromJSON(buff)
	require.NoError(t, err)

	result, err := QueryObject("Implements[].Name | [0]", decoded)
	require.NoError(t, err)
	require.Equal(t, "Instance", result)

	server.Stop()
}
