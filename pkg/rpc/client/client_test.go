package client

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAddress(t *testing.T) {

	u, c, err := parseAddress("/foo/bar/baz")
	require.NoError(t, err)
	require.NotNil(t, c.Transport)
	require.Equal(t, "http://h", u.String())

	u, c, err = parseAddress("unix:///foo/bar/baz")
	require.NoError(t, err)
	require.NotNil(t, c.Transport)
	require.Equal(t, "http://h", u.String())

	u, c, err = parseAddress("tcp://host:9090/foo/bar/baz")
	require.NoError(t, err)
	require.NotNil(t, c.Transport)
	require.Equal(t, "http://host:9090/foo/bar/baz", u.String())

	u, c, err = parseAddress("https://host:9090")
	require.NoError(t, err)
	require.NotNil(t, c.Transport)
	require.Equal(t, "https://host:9090", u.String())

}
