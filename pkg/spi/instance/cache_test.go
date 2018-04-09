package instance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fake struct {
	Plugin
	calls     int
	id        *ID
	instances []Description
	err       error
}

func (f *fake) DescribeInstances(labels map[string]string, properties bool) ([]Description, error) {
	f.calls++
	return f.instances, f.err
}

func (f *fake) Provision(spec Spec) (*ID, error) {
	return f.id, f.err
}

func TestCacheDescribeInstances(t *testing.T) {

	f := &fake{
		instances: []Description{
			{ID: ID("1")},
			{ID: ID("2")},
		},
	}
	p := CacheDescribeInstances(f, 1*time.Hour, time.Now)

	for i := 0; i < 5; i++ {
		desc, err := p.DescribeInstances(map[string]string{"test": "1"}, false)
		require.NoError(t, err)
		require.Equal(t, 1, f.calls)
		require.Equal(t, f.instances, desc)
	}

	f.instances = f.instances[1:]
	for i := 0; i < 5; i++ {
		desc, err := p.DescribeInstances(map[string]string{"test": "2"}, true)
		require.NoError(t, err)
		require.Equal(t, f.instances, desc)
		require.Equal(t, 2, f.calls)
	}

	f.calls = 0
	f.instances = []Description{{ID: ID("x")}, {ID: ID("y")}}
	p = CacheDescribeInstances(f, 0*time.Second, time.Now)
	for i := 0; i < 5; i++ {
		desc, err := p.DescribeInstances(map[string]string{"test": "1"}, false)
		require.NoError(t, err)
		require.Equal(t, f.instances, desc)
		require.Equal(t, i+1, f.calls)
	}

	f.calls = 0
	f.instances = []Description{}
	p = CacheDescribeInstances(f, 100*time.Millisecond, time.Now)
	desc, err := p.DescribeInstances(map[string]string{"test": "1"}, false)
	require.NoError(t, err)
	require.Equal(t, 1, f.calls)
	require.Equal(t, f.instances, desc)

	// force clear cache
	_, err = p.Provision(Spec{})
	require.NoError(t, err)

	desc, err = p.DescribeInstances(map[string]string{"test": "1"}, false)
	require.NoError(t, err)
	require.Equal(t, 2, f.calls)
	require.Equal(t, f.instances, desc)

	<-time.After(100 * time.Millisecond)
	desc, err = p.DescribeInstances(map[string]string{"test": "1"}, false)
	require.NoError(t, err)
	require.Equal(t, 3, f.calls)
	require.Equal(t, f.instances, desc)
}
