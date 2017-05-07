package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetURL(t *testing.T) {

	f, err := GetURL("file:///a/b/c/d.tpl", "e.tpl")
	require.NoError(t, err)
	require.Equal(t, "file:///a/b/c/e.tpl", f.String())

	f, err = GetURL("file:///a/b/c/d.tpl", "../e.tpl")
	require.NoError(t, err)
	require.Equal(t, "file:///a/b/e.tpl", f.String())

	f, err = GetURL("file:///a/b/c/d.tpl", "../e/f/g.tpl")
	require.NoError(t, err)
	require.Equal(t, "file:///a/b/e/f/g.tpl", f.String())

	f, err = GetURL("file:///a/b/c/d.tpl", "http://x/y/z.tpl")
	require.NoError(t, err)
	require.Equal(t, "http://x/y/z.tpl", f.String())
}
