package swarm

import (
	"testing"
	"time"

	gc_types "github.com/docker/infrakit/pkg/controller/gc/types"
	"github.com/docker/infrakit/pkg/fsm"
	"github.com/docker/infrakit/pkg/types"

	"github.com/stretchr/testify/require"
)

func TestSwarmEntities(t *testing.T) {

	gcProperties := new(gc_types.Properties)

	err := types.Decode([]byte(`
observeinterval : 10s
Model : swarm
ModelProperties:
  TickUnit: 1s
  NoData: 20
  RmNodeBufferSize: 20
`), gcProperties)

	require.NoError(t, err)

	require.Equal(t, 10*time.Second, gcProperties.ObserveInterval.Duration())

	m, err := BuildModel(*gcProperties)
	require.NoError(t, err)

	model := m.(*model)
	require.Equal(t, 10*time.Second, model.ObserveInterval.Duration())
	require.Equal(t, fsm.Tick(20), model.NoData)
	require.Equal(t, 1*time.Second, model.TickUnit.Duration())
	require.Equal(t, 10*time.Second, model.tickSize) // must take the slower of the two durations of 10s vs 1s

	m.Start()
	<-time.After(1 * time.Second)
	m.Stop()

	// expect channels to be closed
	{
		_, ok := <-m.GCNode()
		require.False(t, ok)
	}
	{
		_, ok := <-m.GCInstance()
		require.False(t, ok)
	}

}
