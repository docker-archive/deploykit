package terraform

import (
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestParseOptionsEnvsInvalidJSON(t *testing.T) {
	data := types.AnyString("not-json")
	_, err := parseOptionsEnvs(data)
	require.Error(t, err)
}

func TestParseOptionsEnvsNil(t *testing.T) {
	envs, err := parseOptionsEnvs(nil)
	require.NoError(t, err)
	require.Equal(t, []string{}, envs)
}

func TestParseOptionsEnvsEmpytString(t *testing.T) {
	data := types.AnyString("")
	envs, err := parseOptionsEnvs(data)
	require.NoError(t, err)
	require.Equal(t, []string{}, envs)
}

func TestParseOptionsEnvs(t *testing.T) {
	data := types.AnyString(`["k1=v1", "k2=v2"]`)
	envs, err := parseOptionsEnvs(data)
	require.NoError(t, err)
	require.Equal(t, []string{"k1=v1", "k2=v2"}, envs)
}

func TestParseOptionsEnvsNotKeyValuePairs(t *testing.T) {
	data := types.AnyString(`["k1=v1", "keyval"]`)
	envs, err := parseOptionsEnvs(data)
	require.Error(t, err)
	require.Equal(t,
		"Env var is missing '=' character: keyval",
		err.Error())
	require.Equal(t, []string{}, envs)
}
