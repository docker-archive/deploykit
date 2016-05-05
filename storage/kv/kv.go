package kv

import (
	"github.com/docker/libkv/store"
	"github.com/docker/libmachete/storage"
	"strings"
)

type kvStorage struct {
	kvStore store.Store
}

// NewStore creates a new storage implementation based on a KV store.
func NewStore(kvStore store.Store) storage.Storage {
	return &kvStorage{kvStore: kvStore}
}

// To work around some quirks with libkv, partially because we are using a path-oriented API over
// a store that is possibly not path-aware.
// Some known quirks (with the boltdb backend, at least):
//   - List("") results in a panic
//   - store.ErrKeyNotFound is returned when listing an empty store
//   - List(...) is prefix-based, not path-based, so the backend is ignorant of path separators.
// TODO(wfarner): Verify this behavior with different backends and augment for a general solution
// if necessary.
func prefixPath(path string) string {
	return "/" + path
}

func removePathPrefix(path string) string {
	return strings.TrimLeft(path, "/")
}

func (s *kvStorage) Read(key string) ([]byte, error) {
	pair, err := s.kvStore.Get(prefixPath(key))
	if err != nil {
		return nil, err
	}

	return pair.Value, nil
}

func (s *kvStorage) ReadAll() (map[string][]byte, error) {
	kvPairs, err := s.kvStore.List(prefixPath(""))
	if err != nil {
		// ErrKeyNotFound is returned when there are no keys (for boltdb, at least).
		if err == store.ErrKeyNotFound {
			return map[string][]byte{}, nil
		}

		return nil, err
	}

	content := make(map[string][]byte, len(kvPairs))
	for _, pair := range kvPairs {
		content[removePathPrefix(pair.Key)] = pair.Value
	}

	return content, nil
}

func (s *kvStorage) Write(key string, data []byte) error {
	return s.kvStore.Put(prefixPath(key), data, nil)
}

func (s *kvStorage) Delete(key string) error {
	return s.kvStore.Delete(prefixPath(key))
}

func (s *kvStorage) Close() {
	s.kvStore.Close()
}
