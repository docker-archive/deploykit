package options

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestParseEnvInvalidJSON(t *testing.T) {
	data := types.AnyString("not-json")
	_, err := ParseEnvs(data)
	require.Error(t, err)
}

func TestParseEnvNil(t *testing.T) {
	envs, err := ParseEnvs(nil)
	require.NoError(t, err)
	require.Equal(t, []string{}, envs)
}

func TestParseEnvEmpytString(t *testing.T) {
	data := types.AnyString("")
	envs, err := ParseEnvs(data)
	require.NoError(t, err)
	require.Equal(t, []string{}, envs)
}

func TestParseEnv(t *testing.T) {
	data := types.AnyString(`["key=val"]`)
	envs, err := ParseEnvs(data)
	require.NoError(t, err)
	require.Equal(t, []string{"key=val"}, envs)
}

func TestParseEnvNotKeyValuePairs(t *testing.T) {
	data := types.AnyString(`["keyval"]`)
	_, err := ParseEnvs(data)
	require.Error(t, err)
	require.Equal(t,
		"Env var is missing '=' character: keyval",
		err.Error())
}

func TestParseEnvFromFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "options-parser-test")
	require.NoError(t, err)
	filename := "myfile"
	filedata, err := json.Marshal(map[string]string{
		"foo":      "bar",
		"some-key": "some-val",
	})
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, filename), filedata, 0644)
	require.NoError(t, err)
	uri := fmt.Sprintf("file://%s/%s:some-key", dir, filename)
	data := types.AnyString(`["key=` + uri + `"]`)
	envs, err := ParseEnvs(data)
	require.NoError(t, err)
	require.Equal(t, []string{"key=some-val"}, envs)
}

func TestParseEnvFromFileNoKey(t *testing.T) {
	dir, err := ioutil.TempDir("", "options-parser-test")
	require.NoError(t, err)
	filename := "myfile"
	filedata, err := json.Marshal(map[string]string{"foo": "bar"})
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, filename), filedata, 0644)
	require.NoError(t, err)
	uri := fmt.Sprintf("file://%s/%s:some-key", dir, filename)
	data := types.AnyString(`["key=` + uri + `"]`)
	_, err = ParseEnvs(data)
	require.Error(t, err)
	require.Equal(t,
		fmt.Sprintf("File '%s' does not contain key 'some-key'", filepath.Join(dir, filename)),
		err.Error())
}

func TestParseEnvFromNoFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "options-parser-test")
	require.NoError(t, err)
	filename := "some-file"
	uri := fmt.Sprintf("file://%s/%s:some-key", dir, filename)
	data := types.AnyString(`["key=` + uri + `"]`)
	_, err = ParseEnvs(data)
	require.Error(t, err)
	prefix := fmt.Sprintf("Failed to read file %s:", filepath.Join(dir, filename))
	require.True(t,
		strings.HasPrefix(err.Error(), prefix),
		fmt.Sprintf("Error '%s' does not have prefix: %s", err.Error(), prefix))
}

func TestParseEnvFromInvalidFileJSON(t *testing.T) {
	dir, err := ioutil.TempDir("", "options-parser-test")
	require.NoError(t, err)
	filename := "myfile"
	filedata, err := json.Marshal([]string{"a", "b", "c"})
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, filename), filedata, 0644)
	require.NoError(t, err)
	uri := fmt.Sprintf("file://%s/%s:some-key", dir, filename)
	data := types.AnyString(`["key=` + uri + `"]`)
	_, err = ParseEnvs(data)
	require.Error(t, err)
	prefix := fmt.Sprintf("Failed to parse file %s:", filepath.Join(dir, filename))
	require.True(t,
		strings.HasPrefix(err.Error(), prefix),
		fmt.Sprintf("Error '%s' does not have prefix: %s", err.Error(), prefix))
}
