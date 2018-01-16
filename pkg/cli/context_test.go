package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	_ "github.com/docker/infrakit/pkg/cli/backend/http"
	_ "github.com/docker/infrakit/pkg/cli/backend/instance"
	_ "github.com/docker/infrakit/pkg/cli/backend/print"
	_ "github.com/docker/infrakit/pkg/cli/backend/sh"
)

func TestMissing(t *testing.T) {
	require.True(t, Missing("string", ""))
	require.True(t, Missing("int", 0))
	require.True(t, Missing("float", 0.))
	require.True(t, Missing("bool", none))
	require.False(t, Missing("bool", false))
	require.False(t, Missing("bool", true))
}

func plugins() discovery.Plugins {
	d, err := local.NewPluginDiscovery()
	if err != nil {
		panic(err)
	}
	return d
}

func TestContext(t *testing.T) {

	// A template file containing flags and prompts will be parsed and used to configure
	// the cobra command

	script := `
{{/* The directive here tells infrakit to run this script with sh:  =% print %=  */}}

{{/* The function 'flag' will create a flag in the CLI; the function 'prompt' will ask user for input */}}

{{ $doCommit := flag "commit" "bool" "true to commit" false }}
{{ $clusterName := flag "cluster-name" "string" "the name of the cluster" "swarm" }}
{{ $clusterSize := flag "size" "int" "the size of the cluster" 20 }}
{{ $floatValue := flag "param" "float" "some float param" 25.5 }}
{{ $listValue := listflag "tags" "string" "some string tags (Comma-separated)" "test" }}

{{ $user := prompt "Please enter your user name" "string" }}

{{/* An example here where we expose a flag and if not set, ask the user */}}
{{ $instanceType := flag "instance-type" "string" "VM instance type" | prompt "Please specify vm instance type:" "string"}}

{{/* generate a json so we can parse back and check result */}}
{
  "clusterName" : "{{$clusterName}}",
  "clusterSize" : {{$clusterSize}},
  "username" : "{{$user}}",
  "doCommit" : {{$doCommit}},
  "instanceType" : "{{$instanceType}}",
  "param" : {{$floatValue}},
  "tags" : "{{$listValue}}"
}
`

	c := &Context{
		cmd: &cobra.Command{
			Use:   "test",
			Short: "test",
		},
		scope: scope.DefaultScope(plugins),
		src:   "str://" + script,
		input: bytes.NewBufferString("username\n"),
	}

	c.exec = false
	err := c.BuildFlags()
	require.NoError(t, err)

	for _, n := range []string{"commit", "cluster-name", "size", "instance-type", "param"} {
		require.NotNil(t, c.cmd.Flag(n))
	}

	err = c.cmd.Flags().Parse(strings.Split("--param 75.0 --cluster-name swarm1 --tags dev,infrakit --commit true --size 20 --instance-type large", " "))
	require.NoError(t, err)

	err = c.Execute(c.cmd, nil)
	require.NoError(t, err)

	m := map[string]interface{}{}
	err = types.AnyString(c.script).Decode(&m)
	require.NoError(t, err)

	// compare by the encoded json value
	require.Equal(t, types.AnyValueMust(map[string]interface{}{
		"clusterName":  "swarm1",
		"clusterSize":  20,
		"param":        75.0,
		"username":     "username",
		"doCommit":     true,
		"instanceType": "large",
		"tags":         "[dev infrakit]",
	}).String(), types.AnyValueMust(m).String())
}

func TestContextRunShell(t *testing.T) {

	script := `#!/bin/bash
{{/* The directive here tells infrakit to run this script with sh:  =% sh "-s" "--"  %=  */}}
{{ $lines := flag "lines" "int" "the number of lines" 5 }}

for i in $(seq {{$lines}}); do
echo line $i
done
`

	c := &Context{
		scope: scope.DefaultScope(plugins),
		cmd: &cobra.Command{
			Use:   "test",
			Short: "test",
		},
		src: "str://" + script,
	}

	c.exec = false
	err := c.BuildFlags()
	require.NoError(t, err)

	err = c.cmd.Flags().Parse(strings.Split("--lines 3", " "))
	require.NoError(t, err)

	err = c.Execute(c.cmd, nil)
	require.NoError(t, err)

}
