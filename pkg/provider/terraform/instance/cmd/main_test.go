package main

import (
	"testing"

	"github.com/docker/infrakit/pkg/spi/group"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestParseInstanceSpecFromGroupEmptyGroupUrl(t *testing.T) {
	spec, err := parseInstanceSpecFromGroup("", "managers")
	require.NoError(t, err)
	require.Nil(t, spec)
}

func TestParseInstanceSpecFromGroupInvalidJSON(t *testing.T) {
	_, err := parseInstanceSpecFromGroup("str://not-json", "managers")
	require.Error(t, err)
}

func TestParseInstanceSpecFromGroupInvalidGroupSpec(t *testing.T) {
	_, err := parseInstanceSpecFromGroup("str://{{ nosuchfn }}", "managers")
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
	instSpec, err := parseInstanceSpecFromGroup(groupSpecURL, groupID)
	require.NoError(t, err)
	require.Equal(t,
		instance.Spec{
			Properties: types.AnyString(`{"resource": {"aws_instance": {}}}`),
			Tags: map[string]string{
				group.ConfigSHATag: "bootstrap",
				group.GroupTag:     groupID,
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
	instSpec, err := parseInstanceSpecFromGroup(groupSpecURL, groupID)
	require.NoError(t, err)
	require.Equal(t,
		instance.Spec{
			Properties: types.AnyString(`{"resource": {"aws_instance": {}}}`),
			Tags: map[string]string{
				group.ConfigSHATag:    "bootstrap",
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
	instSpec, err := parseInstanceSpecFromGroup(groupSpecURL, "")
	require.NoError(t, err)
	require.Equal(t,
		instance.Spec{
			Properties: types.AnyString(`{"resource": {"aws_instance": {}}}`),
			Tags: map[string]string{
				group.ConfigSHATag: "bootstrap",
			},
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
	_, err := parseInstanceSpecFromGroup(groupSpecURL, "not-managers")
	require.Equal(t,
		"Given spec ID 'managers' does not match given group ID 'not-managers'",
		err.Error())
}
