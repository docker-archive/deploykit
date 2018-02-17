package fsm

// Flap is oscillation between two adjacent states.  For example, a->b followed by b->a is
// counted as 1 flap.  Similarly, b->a followed by a->b is another flap.
type Flap struct {
	States [2]Index
	Count  int
	Raise  Signal
}

func (s *Spec) flap(a, b Index) *Flap {
	key := [2]Index{a, b}
	if a > b {
		key = [2]Index{b, a}
	}
	if f, has := s.flaps[key]; has {
		return f
	}
	return nil
}

// CheckFlappingMust is a Must version (will panic if err) of CheckFlapping
func (s *Spec) CheckFlappingMust(checks []Flap) *Spec {
	_, err := s.CheckFlapping(checks)
	if err != nil {
		panic(err)
	}
	return s
}

// CheckFlapping - Limit is the maximum of a->b b->a transitions allowable.  For detecting
// oscillations between two adjacent states (no hops)
func (s *Spec) CheckFlapping(checks []Flap) (*Spec, error) {
	flaps := map[[2]Index]*Flap{}
	for _, check := range checks {

		// check the state
		for _, state := range check.States {
			if _, has := s.states[state]; !has {
				return nil, ErrUnknownState(state)
			}
		}

		key := [2]Index{check.States[0], check.States[1]}
		if check.States[0] > check.States[1] {
			key = [2]Index{check.States[1], check.States[0]}
		}

		copy := check
		flaps[key] = &copy
	}

	s.flaps = flaps

	return s, nil
}

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
