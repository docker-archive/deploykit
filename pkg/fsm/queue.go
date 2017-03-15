package fsm

import (
	"container/heap"
)

// A priority queue implementing heap.Interface and holds instances prioritized by deadline (if > 0)
type queue []*instance

func newQueue() *queue {
	h := &queue{}
	heap.Init(h)
	return h
}

func (pq *queue) enqueue(instance *instance) {
	heap.Push(pq, instance)
}

func (pq *queue) dequeue() *instance {
	v := heap.Pop(pq)
	return v.(*instance)
}

func (pq *queue) remove(instance *instance) {
	if instance.index > 0 {
		heap.Remove(pq, instance.index)
		instance.index = -1
	}
}

func (pq *queue) update(instance *instance) {
	if instance.index > 0 {
		heap.Fix(pq, instance.index)
	}
}

func (pq queue) Len() int { return len(pq) }

func (pq queue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].deadline < pq[j].deadline
}

func (pq queue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *queue) Push(v interface{}) {
	n := len(*pq)
	instance := v.(*instance)
	instance.index = n
	*pq = append(*pq, instance)
}

func (pq *queue) Pop() interface{} {
	old := *pq
	n := len(old)
	instance := old[n-1]
	instance.index = -1 // for safety
	*pq = old[0 : n-1]
	return instance
}

func (pq *queue) peek() *instance {
	view := *pq
	if len(view) == 0 {
		return nil
	}
	return view[0]
}
