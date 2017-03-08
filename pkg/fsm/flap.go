package fsm

func newFlaps() *flaps {
	return &flaps{
		index: make(map[[2]Index]int),
	}
}

type flaps struct {
	index map[[2]Index]int
}

func (f *flaps) record(a, b Index) {
	key := [2]Index{b, a} // next to complete the flap = b -> a
	f.index[key] = f.index[key] + 1
}

func (f *flaps) reset(a, b Index) {
	for _, key := range [][2]Index{{a, b}, {b, a}} {
		delete(f.index, key)
	}
}

func (f *flaps) count(a, b Index) int {
	total := 0
	for _, key := range [][2]Index{{a, b}, {b, a}} {
		total = f.index[key] + total
	}
	return total / 2
}
