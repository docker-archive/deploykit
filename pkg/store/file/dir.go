package file

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/afero"
	"math/rand"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

var log = logutil.New("module", "store/file")

const logV = logutil.V(300)

// Store is an abstraction for writing and reading from entries in a file directory.
type Store struct {
	t    string
	Dir  string
	yaml bool
	fs   afero.Fs

	// IDFunc is the function that generates the id.
	IDFunc func(interface{}) string
}

// NewStore creates a store of object with a typeName in the given directory.  Instances of the
// type will be stored as files in this directory and named accordingly.
func NewStore(typeName string, dir string, yaml bool) *Store {
	log.Debug("Store", "type", typeName, "dir", dir, "yaml", yaml, "V", logV)
	return &Store{
		t:    typeName,
		Dir:  dir,
		yaml: yaml,
		fs:   afero.NewOsFs(),
	}
}

// Init initializes the store using defaults.
func (s *Store) Init() *Store {
	if s.IDFunc == nil {
		s.IDFunc = func(key interface{}) string {
			return fmt.Sprintf("%s-%v", s.t, key)
		}
	}
	return s
}

// Write writes the object and returns an id or error.
func (s *Store) Write(key interface{}, object interface{}) error {
	if key == nil {
		key = rand.Int63()
	}
	id := s.IDFunc(key)

	var buff []byte
	var err error
	if s.yaml {
		buff, err = types.AnyValueMust(object).MarshalYAML()
		if err != nil {
			return err
		}
	} else {
		buff, err = types.AnyValueMust(object).MarshalJSON()
		if err != nil {
			return err
		}
	}
	log.Debug("Write", "id", id, "object", object, "err", err, "V", logV)
	if err != nil {
		return err
	}
	return afero.WriteFile(s.fs, filepath.Join(s.Dir, string(id)), buff, 0644)
}

// Key returns an id given the key. The id contains type, etc.
func (s *Store) Key(key interface{}) string {
	return s.IDFunc(key)
}

// Read checks for existence
func (s *Store) Read(key interface{}, decode func([]byte) (interface{}, error)) (interface{}, error) {
	id := s.Key(key)
	fp := filepath.Join(s.Dir, id)
	f, err := s.fs.Open(fp)
	log.Debug("Read", "id", id, "path", fp, "err", err)

	buff, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return decode(buff)
}

// Exists checks for existence
func (s *Store) Exists(key interface{}) (bool, error) {
	id := s.Key(key)
	fp := filepath.Join(s.Dir, id)
	_, err := s.fs.Stat(fp)
	log.Debug("Exists", "id", id, "path", fp, "err", err)
	return !os.IsNotExist(err), err
}

// Delete deletes the object by id
func (s *Store) Delete(key interface{}) error {
	id := s.Key(key)
	fp := filepath.Join(s.Dir, id)
	log.Debug("Delete", "key", key, "file", fp, "V", logV)
	return s.fs.Remove(fp)
}

// All iterates through all qualifying objects. tags is the function that gets the tags inside the object
func (s *Store) All(tags map[string]string,
	getTags func(interface{}) map[string]string,
	decode func([]byte) (interface{}, error),
	visit func(interface{}) (bool, error)) error {

	log.Debug("Visit", "tags", tags, "visit", visit, "V", logV)
	entries, err := afero.ReadDir(s.fs, s.Dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {

		if entry.IsDir() {
			continue
		}

		if strings.Index(entry.Name(), s.t) != 0 {
			continue
		}

		fp := filepath.Join(s.Dir, entry.Name())
		file, err := s.fs.Open(fp)
		if err != nil {
			log.Warn("error opening", "path", fp, "err", err)
			return err
		}

		buff, err := ioutil.ReadAll(file)
		if err != nil {
			log.Warn("error reading", "path", fp, "err", err)
			return err
		}

		object, err := decode(buff)
		if err != nil {
			log.Warn("error decoding", "path", fp, "err", err)
			return err
		}

		if getTags != nil && len(tags) > 0 {
			found := getTags(object)
			if hasDifferentTags(tags, found) {
				continue
			}
		}

		if continued, err := visit(object); err != nil {
			log.Warn("error", "path", fp, "err", err)
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
