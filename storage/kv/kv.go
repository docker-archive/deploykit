package kv

import (
	"github.com/docker/libkv/store"
	"github.com/docker/libmachete/storage"
)

type kvStorage struct {
	kvStore store.Store
}

// NewStore creates a new storage implementation based on a KV store.
func NewStore(kvStore store.Store) storage.Storage {
	return &kvStorage{kvStore: kvStore}
}

func (s *kvStorage) Read(key string) ([]byte, error) {
	pair, err := s.kvStore.Get(key)
	if err != nil {
		return nil, err
	}

	return pair.Value, nil
}

func (s *kvStorage) Write(key string, data []byte) error {
	return s.kvStore.Put(key, data, nil)
}

func (s *kvStorage) Delete(key string) error {
	return s.kvStore.Delete(key)
}
