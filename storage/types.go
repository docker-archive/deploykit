package storage

import (
	"fmt"
	"sort"
)

// Key is the unique identifier for stored values.
type Key struct {
	Path []string
}

// RequirePathLength panics if the path length does not match the expected value.
func (k Key) RequirePathLength(requiredPathLength uint) {
	if len(k.Path) != int(requiredPathLength) {
		panic(fmt.Sprintf("Key %v must have exactly %d parts", k, requiredPathLength))
	}
}

// RootKey is an unscoped key, useful for listing the root level of a store.
var RootKey = Key{Path: []string{}}

// SortKeys sorts keys lexicographically.
func SortKeys(keys []Key) {
	sort.Sort(&keySorter{keys})
}

type keySorter struct {
	keys []Key
}

func (k *keySorter) Len() int {
	return len(k.keys)
}

func (k *keySorter) Swap(i, j int) {
	k.keys[i], k.keys[j] = k.keys[j], k.keys[i]
}

func (k *keySorter) Less(i, j int) bool {
	first := k.keys[i].Path
	second := k.keys[j].Path

	// walk elements until non-equal elements, or a length difference is encountered
	for index := range first {
		if len(second) == index+1 {
			// second is shorter than first, and all other entries are equal
			return false
		}
		if first[index] != second[index] {
			return first[index] > second[index]
		}
	}

	if len(second) > len(first) {
		// second is longer than first, and all other entries are equal
		return true
	}

	// items are equal
	return false
}

// KvStore defines functions that can be used to manage entities by unique keys.
type KvStore interface {
	Save(key Key, data []byte) error

	ListRecursive(key Key) ([]Key, error)

	Get(key Key) ([]byte, error)

	Delete(key Key) error
}
