package server

import (
	"io/ioutil"
	"net/http"
	"path"
	"testing"

	rpc_flavor "github.com/docker/infrakit/pkg/rpc/flavor"
	rpc_instance "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/flavor"
	"github.com/docker/infrakit/pkg/template"
	testing_flavor "github.com/docker/infrakit/pkg/testing/flavor"
	testing_instance "github.com/docker/infrakit/pkg/testing/instance"
	"github.com/stretchr/testify/require"
)

func tempSocket() string {
	dir, err := ioutil.TempDir("", "infrakit-test-")
	if err != nil {
		panic(err)
	}

	return path.Join(dir, "instance-impl-test")
}

func TestFetchAPIInfoFromPlugin(t *testing.T) {
	socketPath := tempSocket()

	url := "unix://" + socketPath

	server, err := StartPluginAtPath(socketPath, rpc_instance.PluginServer(&testing_instance.Plugin{}))
	require.NoError(t, err)

	buff, err := template.Fetch(url, template.Options{
		CustomizeFetch: func(req *http.Request) {
			req.URL.Path = "/info/api.json"
			req.URL.Host = "h"
		},
	})
	require.NoError(t, err)

	decoded, err := template.FromJSON(buff)
	require.NoError(t, err)

	result, err := template.QueryObject("Implements[].Name | [0]", decoded)
	require.NoError(t, err)
	require.Equal(t, "Instance", result)

	url = "unix://" + socketPath
	buff, err = template.Fetch(url, template.Options{
		CustomizeFetch: func(req *http.Request) {
			req.URL.Path = "/info/functions.json"
			req.URL.Host = "h"
		},
	})
	require.NoError(t, err)

	server.Stop()
}

type exporter struct {
	flavor.Plugin
}

func (p *exporter) Funcs() []template.Function {
	return []template.Function{
		{
			Name:        "greater",
			Description: []string{"Returns true if a is greater than b"},
			Func: func(a, b int) bool {
				return a > b
			},
		},
		{
			Name:        "equal",
			Description: []string{"Returns true if a is same as b"},
			Func: func(a, b string) bool {
				return a == b
			},
		},
		{
			Name:        "join_token",
			Description: []string{"Returns the join token"},
			Func: func() string {
				return "token"
			},
		},
	}
}

func TestFetchFunctionsFromPlugin(t *testing.T) {
	socketPath := tempSocket()

	url := "unix://" + socketPath

	server, err := StartPluginAtPath(socketPath, rpc_flavor.PluginServer(&exporter{&testing_flavor.Plugin{}}))
	require.NoError(t, err)

	buff, err := template.Fetch(url, template.Options{
		CustomizeFetch: func(req *http.Request) {
			req.URL.Path = "/info/functions.json"
			req.URL.Host = "d"
		},
	})
	require.NoError(t, err)

	decoded, err := template.FromJSON(buff)
	require.NoError(t, err)

	list := decoded.(map[string]interface{})["base"].([]interface{})
	require.Equal(t, 3, len(list))

	result, err := template.QueryObject("[].Usage | [2]", list)
	require.NoError(t, err)
	require.Equal(t, "{{ join_token }}", result)

	server.Stop()
}
