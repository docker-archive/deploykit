package enrollment

import (
	"sort"

	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// keyFunc is a function that extracts the key from the description
type keyFunc func(instance.Description) string

// Descriptions is a slice of descriptions
type Descriptions []instance.Description

func (list Descriptions) index(getKey keyFunc) (map[string]instance.Description, mapset.Set) {
	index := map[string]instance.Description{}
	this := mapset.NewSet()
	for _, n := range list {
		key := getKey(n)
		this.Add(key)
		index[key] = n
	}
	return index, this
}

// Difference returns a list of specs that is not in the receiver.
func Difference(list Descriptions, listKeyFunc keyFunc,
	other Descriptions, otherKeyFunc keyFunc) Descriptions {
	this, thisSet := list.index(listKeyFunc)
	_, thatSet := other.index(otherKeyFunc)
	return toDescriptions(listKeyFunc, thisSet.Difference(thatSet), this)
}

func toDescriptions(keyFunc keyFunc, set mapset.Set, index map[string]instance.Description) Descriptions {
	out := Descriptions{}
	for n := range set.Iter() {
		out = append(out, index[n.(string)])
	}
	return out
}

// Delta computes the changes necessary to make the receiver match the input:
// 1. the add Descriptions are entries to add to receiver
// 2. the remove Descriptions are entries to remove from receiver
// 3. changes are a slice of Descriptions where changes[x][0] is the original, and changes[x][1] is new
func Delta(list Descriptions, listKeyFunc keyFunc, other Descriptions,
	otherKeyFunc keyFunc) (add Descriptions, remove Descriptions, changes [][2]instance.Description) {

	sort.Sort(instance.Descriptions(list))
	sort.Sort(instance.Descriptions(other))

	this, thisSet := list.index(listKeyFunc)
	that, thatSet := other.index(otherKeyFunc)

	removeSet := thisSet.Difference(thatSet)
	remove = toDescriptions(listKeyFunc, removeSet, this)

	addSet := thatSet.Difference(thisSet)
	add = toDescriptions(otherKeyFunc, addSet, that)

	changeSet := thisSet.Difference(removeSet)

	sort.Sort(instance.Descriptions(add))
	sort.Sort(instance.Descriptions(remove))

	changes = [][2]instance.Description{}
	for n := range changeSet.Iter() {
		key := n.(string)
		if this[key].Fingerprint() != that[key].Fingerprint() {
			changes = append(changes, [2]instance.Description{this[key], that[key]})
		}
	}
	return
}
