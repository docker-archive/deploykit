package types

import (
	"testing"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestParseOptionsEnvsInvalidJSON(t *testing.T) {
	o := Options{Envs: *types.AnyString("not-json")}
	_, err := o.ParseOptionsEnvs()
	require.Error(t, err)
}

func TestParseOptionsEnvsNil(t *testing.T) {
	o := Options{Envs: nil}
	envs, err := o.ParseOptionsEnvs()
	require.NoError(t, err)
	require.Equal(t, []string{}, envs)
}

func TestParseOptionsEnvsEmpytString(t *testing.T) {
	o := Options{Envs: *types.AnyString("")}
	envs, err := o.ParseOptionsEnvs()
	require.NoError(t, err)
	require.Equal(t, []string{}, envs)
}

func TestParseOptionsEnvs(t *testing.T) {
	o := Options{Envs: *types.AnyString(`["k1=v1", "k2=v2"]`)}
	envs, err := o.ParseOptionsEnvs()
	require.NoError(t, err)
	require.Equal(t, []string{"k1=v1", "k2=v2"}, envs)
}

func TestParseOptionsEnvsNotKeyValuePairs(t *testing.T) {
	o := Options{Envs: *types.AnyString(`["k1=v1", "keyval"]`)}
	envs, err := o.ParseOptionsEnvs()
	require.Error(t, err)
	require.Equal(t,
		"Env var is missing '=' character: keyval",
		err.Error())
	require.Equal(t, []string{}, envs)
}

func plugins() discovery.Plugins {
	d, err := local.NewPluginDiscovery()
	if err != nil {
		panic(err)
	}
	return d
}

func TestParseInstanceSpecFromGroupEmptyGroupUrl(t *testing.T) {
	o := Options{ImportGroupID: "managers"}
	spec, err := o.ParseInstanceSpecFromGroup(scope.DefaultScope(plugins))
	require.NoError(t, err)
	require.Nil(t, spec)
}

func TestParseInstanceSpecFromGroupInvalidJSON(t *testing.T) {
	o := Options{
		ImportGroupID:      "managers",
		ImportGroupSpecURL: "str://not-json",
	}
	_, err := o.ParseInstanceSpecFromGroup(scope.DefaultScope(plugins))
	require.Error(t, err)
}

func TestParseInstanceSpecFromGroupInvalidGroupSpec(t *testing.T) {
	o := Options{
		ImportGroupID:      "managers",
		ImportGroupSpecURL: "str://{{ nosuchfn }}",
	}
	_, err := o.ParseInstanceSpecFromGroup(scope.DefaultScope(plugins))
	require.Error(t, err)
}

func TestParseInstanceSpecFromGroup(t *testing.T) {
	groupSpecURL := `str://
{
  "ID": "managers",
  "Properties": {
    "instance": {
      "Properties": {"resource": {"aws_instance": {}}}
    }
  }
}`
	groupID := "managers"
	o := Options{
		ImportGroupID:      groupID,
		ImportGroupSpecURL: groupSpecURL,
	}
	instSpec, err := o.ParseInstanceSpecFromGroup(scope.DefaultScope(plugins))
	require.NoError(t, err)
	require.Equal(t,
		instance.Spec{
			Properties: types.AnyString(`{"resource": {"aws_instance": {}}}`),
			Tags: map[string]string{
				group.GroupTag: groupID,
			},
		},
		*instSpec)
}

func TestParseInstanceSpecFromGroupLogicalID(t *testing.T) {
	groupSpecURL := `str://
{
  "ID": "managers",
  "Properties": {
    "Allocation": {
      "LogicalIDs": ["mgr1", "mgr2", "mgr3"]
    },
    "instance": {
      "Properties": {"resource": {"aws_instance": {}}}
    }
  }
}`
	groupID := "managers"
	o := Options{
		ImportGroupID:      groupID,
		ImportGroupSpecURL: groupSpecURL,
	}
	instSpec, err := o.ParseInstanceSpecFromGroup(scope.DefaultScope(plugins))
	require.NoError(t, err)
	require.Equal(t,
		instance.Spec{
			Properties: types.AnyString(`{"resource": {"aws_instance": {}}}`),
			Tags: map[string]string{
				group.GroupTag:        groupID,
				instance.LogicalIDTag: "mgr1",
			},
		},
		*instSpec)
}

func TestParseInstanceSpecFromGroupNoGroupIDSpecified(t *testing.T) {
	groupSpecURL := `str://
{
  "ID": "managers",
  "Properties": {
    "instance": {
      "Properties": {"resource": {"aws_instance": {}}}
    }
  }
}`
	o := Options{
		ImportGroupID:      "",
		ImportGroupSpecURL: groupSpecURL,
	}
	instSpec, err := o.ParseInstanceSpecFromGroup(scope.DefaultScope(plugins))
	require.NoError(t, err)
	require.Equal(t,
		instance.Spec{
			Properties: types.AnyString(`{"resource": {"aws_instance": {}}}`),
			Tags:       map[string]string{},
		},
		*instSpec)
}

func TestParseInstanceSpecFromGroupNonMatchingGroupID(t *testing.T) {
	groupSpecURL := `str://
{
  "ID": "managers",
  "Properties": {
    "instance": {
      "Properties": {"resource": {"aws_instance": {}}}
    }
  }
}`
	o := Options{
		ImportGroupID:      "not-managers",
		ImportGroupSpecURL: groupSpecURL,
	}
	_, err := o.ParseInstanceSpecFromGroup(scope.DefaultScope(plugins))
	require.Equal(t,
		"Given spec ID 'managers' does not match given group ID 'not-managers'",
		err.Error())
}
