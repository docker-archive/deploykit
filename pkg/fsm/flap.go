package fsm

func newFlaps() *flaps {
	return &flaps{
		history: []Index{},
	}
}

type flaps struct {
	history []Index
}

func (f *flaps) reset() {
	f.history = []Index{}
}

func equals(i, j []Index) bool {
	if len(j) != len(i) {
		return false
	}
	for k := range j {
		if i[k] != j[k] {
			return false
		}
	}
	return true
}

func (f *flaps) record(a, b Index) {
	old := append([]Index{}, f.history...)
	defer func() { log.Debug("record", "before", old, "a", a, "b", b, "after", f.history) }()

	if len(f.history) == 0 {
		f.history = []Index{a, b}
		return
	}
	last := f.history[len(f.history)-2:]
	if equals(last, []Index{b, a}) {
		f.history = append(f.history, b)
	} else {
		f.reset()
	}
}

func (f *flaps) count(a, b Index) int {
	if len(f.history) < 2 {
		return 0
	}
	search := []Index{a, b, a}
	if f.history[len(f.history)-1] == b {
		search = []Index{b, a, b}
	}

	count := 0
	defer func() { log.Debug("search", "search", search, "history", f.history, "count", count) }()

	for i := len(f.history); i > 2; i = i - 2 {
		check := f.history[i-3 : i]
		if equals(check, search) {
			count++
		}
	}
	return count
}
