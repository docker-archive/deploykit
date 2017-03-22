package fsm

func newFifo(size int) *fifo {
	return &fifo{
		nodes: make([]*event, size),
		size:  size,
	}
}

// fifo is a basic FIFO queue of events based on a circular list that resizes as needed.
type fifo struct {
	nodes []*event
	size  int
	head  int
	tail  int
	count int
}

// Len returns the size of the fifo
func (q *fifo) Len() int {
	return q.count
}

// push adds a node to the queue.
func (q *fifo) push(n *event) {
	if q.head == q.tail && q.count > 0 {
		nodes := make([]*event, len(q.nodes)+q.size)
		copy(nodes, q.nodes[q.head:])
		copy(nodes[len(q.nodes)-q.head:], q.nodes[:q.head])
		q.head = 0
		q.tail = len(q.nodes)
		q.nodes = nodes
	}
	q.nodes[q.tail] = n
	q.tail = (q.tail + 1) % len(q.nodes)
	q.count++
}

// pop removes and returns a node from the queue in first to last order.
func (q *fifo) pop() *event {
	if q.count == 0 {
		return nil
	}
	node := q.nodes[q.head]
	q.head = (q.head + 1) % len(q.nodes)
	q.count--
	return node
}
