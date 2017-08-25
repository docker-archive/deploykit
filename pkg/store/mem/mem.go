package mem

import (
	"fmt"
	"strings"
	"sync"

	"github.com/docker/infrakit/pkg/store"
)

// Mem stores the kv pairs with key by a prefix set at construct time.  This is used for dev, learning mostly
type Mem struct {
	prefix string
	store  map[string][]byte
	lock   sync.RWMutex
}

// NewStore returns a store
func NewStore(prefix string) store.KV {
	return &Mem{
		prefix: prefix,
		store:  map[string][]byte{},
	}
}

// Close implements io.Closer
func (s *Mem) Close() error {
	return nil
}

// Write writes the object and returns an id or error.
func (s *Mem) Write(key interface{}, object []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.store[s.Key(key)] = object
	return nil
}

// Key returns an id given the key. The id contains type, etc.
func (s *Mem) Key(key interface{}) string {
	return fmt.Sprintf("%v-%v", s.prefix, key)
}

// Read loads the object
func (s *Mem) Read(key interface{}) ([]byte, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if exists, err := s.Exists(key); err != nil {
		return nil, err
	} else if !exists {
		return nil, fmt.Errorf("not found %v", key)
	}
	v := s.store[s.Key(key)]
	return v, nil
}

// Exists checks for existence
func (s *Mem) Exists(key interface{}) (bool, error) {
	_, exists := s.store[s.Key(key)]
	return exists, nil
}

// Delete deletes the object by id
func (s *Mem) Delete(key interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	exists, err := s.Exists(key)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("not found %v", key)
	}
	delete(s.store, s.Key(key))
	return nil
}

// Entries returns the entries
func (s *Mem) Entries() (<-chan store.Pair, error) {
	out := make(chan store.Pair)
	go func() {
		s.lock.RLock()
		defer s.lock.RUnlock()
		for k, v := range s.store {
			out <- store.Pair{
				Key:   strings.Split(k, "-")[1],
				Value: v,
			}
		}
		close(out)
	}()
	return out, nil
}
