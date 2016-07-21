package sshutil

import (
	"fmt"
	"github.com/spf13/afero"
	"path/filepath"
)

// KeyStore stores SSH private key data.
type KeyStore interface {
	Write(name string, data []byte) error

	Read(name string) ([]byte, error)

	Delete(name string) error
}

// FileSystemKeyStore creates a KeyStore that saves keys on a file system in plaintext.
func FileSystemKeyStore(fs afero.Fs, basePath string) KeyStore {
	return &localKeyStore{fs: fs, basePath: basePath}
}

type localKeyStore struct {
	fs       afero.Fs
	basePath string
}

func (k localKeyStore) keyPath(name string) string {
	rootDir := "./"
	if k.basePath != "" {
		rootDir = k.basePath
	}

	return fmt.Sprintf("%sec2/ssh-keys/%s", rootDir, name)
}

func (k localKeyStore) Write(name string, data []byte) error {
	path := k.keyPath(name)
	k.fs.MkdirAll(filepath.Dir(path), 0700)
	return afero.WriteFile(k.fs, path, data, 0600)
}

func (k localKeyStore) Read(name string) ([]byte, error) {
	return afero.ReadFile(k.fs, k.keyPath(name))
}

func (k localKeyStore) Delete(name string) error {
	return k.fs.Remove(k.keyPath(name))
}
