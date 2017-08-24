package file

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/store"
	"math/rand"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

var log = logutil.New("module", "store/file")

const logV = logutil.V(300)

// Store is an abstraction for writing and reading from entries in a file directory.
type Store struct {
	t   string
	Dir string

	// IDFunc is the function that generates the id.
	IDFunc func(interface{}) string
}

// NewStore creates a store of object with a typeName in the given directory.  Instances of the
// type will be stored as files in this directory and named accordingly.
func NewStore(typeName string, dir string) store.KV {
	log.Debug("Store", "type", typeName, "dir", dir, "V", logV)
	return &Store{
		t:   typeName,
		Dir: dir,
		IDFunc: func(key interface{}) string {
			return fmt.Sprintf("%s-%v", typeName, key)
		},
	}
}

// Close implements io.Closer
func (s *Store) Close() error {
	return nil
}

// Write writes the object and returns an id or error.
func (s *Store) Write(key interface{}, value []byte) error {
	if key == nil {
		key = rand.Int63()
	}
	id := s.IDFunc(key)
	log.Debug("Write", "id", id, "value", string(value), "V", logV)
	return ioutil.WriteFile(filepath.Join(s.Dir, string(id)), value, 0644)
}

// Key returns an id given the key. The id contains type, etc.
func (s *Store) Key(key interface{}) string {
	return s.IDFunc(key)
}

// Read checks for existence
func (s *Store) Read(key interface{}) ([]byte, error) {
	id := s.Key(key)
	fp := filepath.Join(s.Dir, id)
	log.Debug("Read", "id", id, "path", fp, "V", logV)
	return ioutil.ReadFile(fp)
}

// Exists checks for existence
func (s *Store) Exists(key interface{}) (bool, error) {
	id := s.Key(key)
	fp := filepath.Join(s.Dir, id)
	_, err := os.Stat(fp)
	log.Debug("Exists", "id", id, "path", fp, "err", err)
	v := os.IsNotExist(err)
	if v {
		return false, nil
	}
	return !v, err
}

// Delete deletes the object by id
func (s *Store) Delete(key interface{}) error {
	id := s.Key(key)
	fp := filepath.Join(s.Dir, id)
	log.Debug("Delete", "key", key, "file", fp, "V", logV)
	return os.Remove(fp)
}

// Entries returns the entries
func (s *Store) Entries() (<-chan store.Pair, error) {
	entries, err := ioutil.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}

	out := make(chan store.Pair)
	go func() {
		defer close(out)
		for _, entry := range entries {

			if entry.IsDir() {
				continue
			}

			if strings.Index(entry.Name(), s.t) != 0 {
				continue
			}

			fp := filepath.Join(s.Dir, entry.Name())
			log.Debug("reading", "path", fp, "V", logV)
			buff, err := ioutil.ReadFile(fp)
			if err != nil {
				log.Warn("error reading", "path", fp, "err", err)
				continue
			}

			out <- store.Pair{
				Key:   strings.Split(entry.Name(), "-")[1],
				Value: buff,
			}
		}
	}()
	return out, nil
}
