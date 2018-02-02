package callable

import (
	"testing"

	"github.com/docker/infrakit/pkg/callable/backend"
	"github.com/stretchr/testify/require"
)

func TestDefineParameters(t *testing.T) {

	p := &Parameters{}

	testfunc := func(p backend.Parameters) {
		t.Log(p)
	}
	testfunc(p)

	type test struct {
		backend.Parameters
	}

	composed := &test{p}
	require.NotNil(t, composed.Parameters)

	s := p.String("test", "hello", "Hello")
	require.Equal(t, "hello", *s)
	*s = "world"

	_, err := p.GetString("not-found")
	require.Error(t, err)

	found, err := p.GetString("test")
	require.NoError(t, err)
	require.Equal(t, "world", found)

	*s = "war"
	found, err = p.GetString("test")
	require.NoError(t, err)
	require.Equal(t, "war", found)

	slice := p.StringSlice("slice", []string{"hello"}, "Hello")
	require.Equal(t, []string{"hello"}, *slice)
	*slice = append(*slice, "world")
	foundSlice, err := p.GetStringSlice("slice")
	require.NoError(t, err)
	require.Equal(t, []string{"hello", "world"}, foundSlice)

}
