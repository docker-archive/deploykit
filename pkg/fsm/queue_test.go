package fsm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueue(t *testing.T) {

	// Tests the priority queue by deadline

	q := newQueue()

	q.enqueue(&instance{deadline: Time(1)})
	q.enqueue(&instance{deadline: Time(3)})
	q.enqueue(&instance{deadline: Time(2)})

	peek := q.peek()
	require.Equal(t, Time(1), peek.deadline)

	x := &instance{deadline: Time(5)}
	q.enqueue(x)

	q.enqueue(&instance{deadline: Time(5)})

	y := &instance{deadline: Time(4)}
	q.enqueue(y)
	q.enqueue(&instance{deadline: Time(4)})
	q.enqueue(&instance{deadline: Time(20)})

	x.deadline = -1
	q.update(x)
	q.update(&instance{deadline: Time(200), index: 5}) // no effect -- not tracked.

	q.remove(y)
	q.remove(&instance{deadline: Time(200), index: -1}) // no effect -- not tracked.

	sorted := []int{}
	for q.Len() > 0 {
		sorted = append(sorted, int(q.dequeue().deadline))
	}

	require.Equal(t, []int{-1, 1, 2, 3, 4, 5, 20}, sorted)
}

func TestQueue2(t *testing.T) {

	// Tests the priority queue by deadline

	q := newQueue()

	q.enqueue(&instance{deadline: Time(1)})
	require.Equal(t, Time(1), q.peek().deadline)

	q.enqueue(&instance{deadline: Time(3)})
	q.enqueue(&instance{deadline: Time(2)})

	q.dequeue()
	q.dequeue()

	require.Equal(t, Time(3), q.peek().deadline)

	q.dequeue()

	require.Nil(t, q.peek())
}
