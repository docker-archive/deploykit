package fsm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFifo(t *testing.T) {

	q := newFifo(10)

	for i := 0; i < 100; i++ {
		q.push(&event{instance: ID(i)})
	}

	require.Equal(t, 100, q.Len())

	ids := []ID{}

	for q.Len() > 0 {
		event := q.pop()
		require.NotNil(t, event)
		ids = append(ids, event.instance)
	}

	require.Equal(t, 100, len(ids))
	for i := 0; i < 100; i++ {
		require.Equal(t, ID(i), ids[i])
	}
}
