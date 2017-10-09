package types

import (
	"github.com/deckarep/golang-set"
	"sort"
)

// Specs is a collection of Specs
type Specs []Spec

// key is used to identify a spec.  Two specs have the same key even if they have
// very different properties.  This is so we can compute the set difference not taking into account of actual
// differences in embedded properties.
type key struct {
	kind    string
	version string
	name    string
	id      string
}

func (s Spec) key() key {
	k := key{
		kind:    s.Kind,
		version: s.Version,
		name:    s.Metadata.Name,
	}
	if s.Metadata.Identity != nil {
		k.id = s.Metadata.Identity.ID
	}
	return k
}

// Fingerprint returns the fingerprint of the spec
func (s Spec) Fingerprint() string {
	return Fingerprint(AnyValueMust(s))
}

func toSpecs(set mapset.Set, index map[key]Spec) Specs {
	out := Specs{}
	for n := range set.Iter() {
		key := n.(key)
		out = append(out, index[key])
	}
	return out
}

func (list Specs) index() (map[key]Spec, mapset.Set) {
	index := map[key]Spec{}
	this := mapset.NewSet()
	for _, spec := range list {
		key := spec.key()
		this.Add(key)
		index[key] = spec
	}
	return index, this
}

// Difference returns a list of specs that is not in the receiver.
func (list Specs) Difference(other Specs) Specs {
	this, thisSet := list.index()
	_, thatSet := other.index()
	return toSpecs(thisSet.Difference(thatSet), this)
}

func (list Specs) fingerprint() string {
	return Fingerprint(AnyValueMust(list))
}

// Len is part of sort.Interface.
func (list Specs) Len() int {
	return len(list)
}

// Swap is part of sort.Interface.
func (list Specs) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (list Specs) Less(i, j int) bool {
	return list[i].Compare(list[j]) < 0
}

// SpecsFromString returns the Specs from input string as YAML or JSON
func SpecsFromString(s string) (Specs, error) {
	return SpecsFromBytes([]byte(s))
}

// SpecsFromBytes parses the input either as YAML or JSON and returns the Specs
func SpecsFromBytes(b []byte) (Specs, error) {
	out := Specs{}
	any, err := AnyYAML(b)
	if err != nil {
		any = AnyBytes(b)
	}
	err = any.Decode(&out)
	return out, err
}

// MustSpecs returns the specs or panic if errors
func MustSpecs(s Specs, err error) Specs {
	if err != nil {
		panic(err)
	}
	return s
}

// Slice returns the slice
func (list Specs) Slice() []Spec {
	return []Spec(list)
}

// Delta computes the changes necessary to make the receiver match the input:
// 1. the add Specs are entries to add to receiver
// 2. the remove Specs are entries to remove from receiver
// 3. changes are a slice of Specs where changes[x][0] is the original, and changes[x][1] is new
func (list Specs) Delta(other Specs) (add Specs, remove Specs, changes [][2]Spec) {

	sort.Sort(list)
	sort.Sort(other)

	this, thisSet := list.index()
	that, thatSet := other.index()

	removeSet := thisSet.Difference(thatSet)
	remove = toSpecs(removeSet, this)

	addSet := thatSet.Difference(thisSet)
	add = toSpecs(addSet, that)

	changeSet := thisSet.Difference(removeSet)

	sort.Sort(add)
	sort.Sort(remove)

	changes = [][2]Spec{}
	for n := range changeSet.Iter() {
		key := n.(key)
		if this[key].Fingerprint() != that[key].Fingerprint() {
			changes = append(changes, [2]Spec{this[key], that[key]})
		}
	}
	return
}

// Changes returns the changes applying other will do to the receiver
func (list Specs) Changes(other Specs) Changes {
	ch := Changes{
		Current: list,
		New:     other,
	}
	ch.Add, ch.Remove, ch.Changes = list.Delta(other)
	return ch
}

// Changes contain the changes needed to make current specs into the new specs
type Changes struct {
	// Current is the current specs
	Current Specs
	// New is the new specs
	New Specs
	// Add is the set of specs to add to current
	Add Specs
	// Remove is the set of specs to remove
	Remove Specs
	// Changes is a set of before/after specs
	Changes [][2]Spec
}
