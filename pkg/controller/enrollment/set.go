package enrollment

import (
	"sort"

	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// keyFunc is a function that extracts the key from the description
type keyFunc func(instance.Description) (string, error)

func index(list instance.Descriptions, getKey keyFunc) (map[string]instance.Description, mapset.Set) {
	index := map[string]instance.Description{}
	this := mapset.NewSet()
	for _, n := range list {
		key, err := getKey(n)
		if err != nil {
			log.Error("cannot index entry", "instance.Description", n, "err", err)
			continue
		}
		this.Add(key)
		index[key] = n
	}
	return index, this
}

// Difference returns a list of specs that is not in the receiver.
func Difference(list instance.Descriptions, listKeyFunc keyFunc,
	other instance.Descriptions, otherKeyFunc keyFunc) instance.Descriptions {
	this, thisSet := index(list, listKeyFunc)
	_, thatSet := index(other, otherKeyFunc)
	return toDescriptions(listKeyFunc, thisSet.Difference(thatSet), this)
}

func toDescriptions(keyFunc keyFunc, set mapset.Set, index map[string]instance.Description) instance.Descriptions {
	out := instance.Descriptions{}
	for n := range set.Iter() {
		out = append(out, index[n.(string)])
	}
	sort.Sort(out)
	return out
}

// Delta computes the changes necessary to make the list match other:
// 1. the add Descriptions are entries to add to other
// 2. the remove Descriptions are entries to remove from other
// 3. changes are a slice of Descriptions where changes[x][0] is the original, and changes[x][1] is new
func Delta(list instance.Descriptions, listKeyFunc keyFunc, other instance.Descriptions,
	otherKeyFunc keyFunc) (add instance.Descriptions, remove instance.Descriptions, changes [][2]instance.Description) {

	sort.Sort(instance.Descriptions(list))
	sort.Sort(instance.Descriptions(other))

	this, thisSet := index(list, listKeyFunc)
	that, thatSet := index(other, otherKeyFunc)

	removeSet := thatSet.Difference(thisSet)
	remove = toDescriptions(otherKeyFunc, removeSet, that)

	addSet := thisSet.Difference(thatSet)
	add = toDescriptions(listKeyFunc, addSet, this)

	changeSet := thatSet.Difference(removeSet)

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
