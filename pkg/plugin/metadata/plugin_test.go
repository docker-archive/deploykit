package metadata

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

func first(a, b interface{}) interface{} {
	return a
}

func TestPluginUnserializedReadWrites(t *testing.T) {

	m := map[string]interface{}{}

	require.True(t, types.Put(types.PathFromString("us-west-1/metrics/instances/count"), 2000, m))
	require.True(t, types.Put(types.PathFromString("us-west-2/metrics/instances/count"), 1000, m))

	p := NewPluginFromData(m)

	require.Equal(t, []string{"us-west-1", "us-west-2"}, first(p.Keys(types.PathFromString("/"))))
	require.Nil(t, first(p.Get(types.PathFromString("us-west-1/metrics/instances/foo"))))
	require.Equal(t, "2000", first(p.Get(types.PathFromString("us-west-1/metrics/instances/count"))).(*types.Any).String())

}

func TestPluginSerializedReadWrites(t *testing.T) {

	c := make(chan func(map[string]interface{}))
	p := NewPluginFromChannel(c)

	var wait sync.WaitGroup

	start := make(chan struct{})
	for i := range []int{0, 1, 2, 3} {
		k := fmt.Sprintf("namespace/%d/value", i)
		v := i * 100
		go func() {
			<-start
			c <- func(m map[string]interface{}) {
				types.Put(types.PathFromString(k), v, m)
				wait.Add(1)
			}
		}()
	}

	close(start) // start!

	time.Sleep(10 * time.Millisecond)

	results := []string{}
	for i := range []int{0, 1, 2, 3} {

		k := fmt.Sprintf("namespace/%d/value", i)
		val, err := p.Get(types.PathFromString(k))
		require.NoError(t, err)

		if val != nil {
			results = append(results, val.String())
		}

		wait.Done()
	}

	close(c)

	wait.Wait()

	require.Equal(t, []string{"0", "100", "200", "300"}, results)
}
