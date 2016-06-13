package filestore

import (
	"fmt"
	"github.com/docker/libmachete/storage"
	"github.com/spf13/afero"
	"os"
	"path"
	"regexp"
	"strings"
)

type fileStore struct {
	fs  afero.Fs
	dir string
}

const slash = string(os.PathSeparator)

// NewFileStore creates a file store backed by the provided file system.
func NewFileStore(fs afero.Fs, dir string) storage.KvStore {
	return fileStore{fs: fs, dir: dir}
}

// NewOsFileStore creates a file store backed by the OS file system.
func NewOsFileStore(dir string) storage.KvStore {
	return NewFileStore(afero.NewOsFs(), dir)
}

// Mkdirs ensures that the file store directory exists.  The creator of a file store should call this prior to using it.
func (f fileStore) Mkdirs() error {
	return f.fs.MkdirAll(f.dir, 0750)
}

const pathRegexp = "^[\\w\\.\\-]+$"

func validateKey(key storage.Key) error {
	for _, part := range key.Path {
		if len(part) == 0 {
			return fmt.Errorf("A key may not have empty path parts")
		}

		if matched, _ := regexp.MatchString(pathRegexp, part); !matched {
			return fmt.Errorf("Key part contains illegal characters: %s", part)
		}
	}

	return nil
}

func (f fileStore) keyToPath(key storage.Key) string {
	elements := []string{f.dir}
	elements = append(elements, key.Path...)
	return path.Join(elements...)
}

func (f fileStore) Save(key storage.Key, data []byte) error {
	err := validateKey(key)
	if err != nil {
		return err
	}

	return afero.WriteFile(f.fs, f.keyToPath(key), data, 0700)
}

func (f fileStore) ListRecursive(key storage.Key) ([]storage.Key, error) {
	err := validateKey(key)
	if err != nil {
		return nil, err
	}

	keys := []storage.Key{}

	err = afero.Walk(f.fs, f.keyToPath(key), func(path string, info os.FileInfo, err error) error {
		if info != nil && !info.IsDir() {
			keys = append(keys, storage.Key{Path: strings.Split(strings.Trim(path, slash), slash)})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return keys, nil
}

func (f fileStore) Get(key storage.Key) ([]byte, error) {
	err := validateKey(key)
	if err != nil {
		return nil, err
	}

	return afero.ReadFile(f.fs, f.keyToPath(key))
}

func (f fileStore) Delete(key storage.Key) error {
	err := validateKey(key)
	if err != nil {
		return err
	}

	return f.fs.Remove(f.keyToPath(key))
}
