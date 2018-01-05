package enrollment

import (
	"sort"

	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// keyFunc is a function that extracts the key from the description
type keyFunc func(instance.Description) (string, error)

func index(list instance.Descriptions, getKey keyFunc) (map[string]instance.Description, mapset.Set, error) {
	// Track errors and return what could be indexed
	var e error
	index := map[string]instance.Description{}
	this := mapset.NewSet()
	for _, n := range list {
		key, err := getKey(n)
		if err != nil {
			log.Error("cannot index entry", "instance.Description", n, "err", err)
			e = err
			continue
		}
		this.Add(key)
		index[key] = n
	}
	return index, this, e
}

// Difference returns a list of specs that is not in the receiver.
func Difference(list instance.Descriptions, listKeyFunc keyFunc,
	other instance.Descriptions, otherKeyFunc keyFunc) (instance.Descriptions, error) {
	this, thisSet, err1 := index(list, listKeyFunc)
	_, thatSet, err2 := index(other, otherKeyFunc)
	// Return an error if either failed to index
	var e error
	if err1 != nil {
		e = err1
	} else if err2 != nil {
		e = err2
	}
	return toDescriptions(listKeyFunc, thisSet.Difference(thatSet), this), e
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
func Delta(list instance.Descriptions, listKeyFunc keyFunc, other instance.Descriptions,
	otherKeyFunc keyFunc) (add instance.Descriptions, remove instance.Descriptions) {

	sort.Sort(instance.Descriptions(list))
	sort.Sort(instance.Descriptions(other))

	this, thisSet, err1 := index(list, listKeyFunc)
	that, thatSet, err2 := index(other, otherKeyFunc)

	// Never remove anything if there are errors since we do not know if the one
	// of the current instances failed to parse
	if err1 == nil && err2 == nil {
		removeSet := thatSet.Difference(thisSet)
		remove = toDescriptions(otherKeyFunc, removeSet, that)
	} else {
		remove = []instance.Description{}
	}

	// Always add what we could parse that is not in the set already
	addSet := thisSet.Difference(thatSet)
	add = toDescriptions(listKeyFunc, addSet, this)

	sort.Sort(instance.Descriptions(add))
	sort.Sort(instance.Descriptions(remove))

	return
}
