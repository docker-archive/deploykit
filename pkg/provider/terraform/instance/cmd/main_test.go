package main

import (
	"testing"

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
				"infrakit.config_sha": "bootstrap",
				"infrakit.group":      groupID,
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
				"infrakit.config_sha": "bootstrap",
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
