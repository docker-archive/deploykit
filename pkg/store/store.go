package store

import (
	"io"
)

// Snapshot provides means to save and load an object.  This is not meant to be
// a generic k-v store.
type Snapshot interface {
	io.Closer

	// Save marshals (encodes) and saves a snapshot of the given object.
	Save(obj interface{}) error

	// Load loads a snapshot and marshals (decodes) into the given reference.
	// If no data is available to unmarshal into the given struct, the fuction returns nil.
	Load(output interface{}) error
}

// Pair is the kv pair
type Pair struct {
	Key   interface{}
	Value []byte
}

// KV stores key-value pairs
type KV interface {
	io.Closer

	// Write writes the object and returns an id or error.
	Write(key interface{}, value []byte) error
	// Key returns an id given the key. The id contains type, etc.
	Key(key interface{}) string
	// Read loads the object
	Read(key interface{}) ([]byte, error)
	// Exists checks for existence
	Exists(key interface{}) (bool, error)
	// Delete deletes the object by id
	Delete(key interface{}) error
	// Entries returns the entries, unsorted
	Entries() (<-chan Pair, error)
}

// Visit visits all entries matching the tags
func Visit(store KV, tags map[string]string,
	getTags func(interface{}) map[string]string,
	decode func([]byte) (interface{}, error),
	visit func(interface{}) (bool, error)) error {

	entries, err := store.Entries()
	if err != nil {
		return err
	}

	for entry := range entries {
		object, err := decode(entry.Value)
		if err != nil {
			return err
		}

		if getTags != nil && len(tags) > 0 {
			found := getTags(object)
			if hasDifferentTags(tags, found) {
				continue
			}
		}

		if continued, err := visit(object); err != nil {
			return err
		} else if !continued {
			return nil
		}
	}
	return nil
}

func hasDifferentTags(expected, actual map[string]string) bool {
	if len(actual) == 0 {
		return true
	}
	for k, v := range expected {
		if a, ok := actual[k]; ok && a != v {
			return true
		}
	}
	return false
}
