package filestore

import (
	"github.com/docker/libmachete/storage"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"testing"
)

type fixture struct {
	t     *testing.T
	store storage.KvStore
}

func (f fixture) save(key storage.Key, value string) {
	err := f.store.Save(key, []byte(value))
	require.NoError(f.t, err)

	// Any value saved should be immediately retrievable with the same value.
	stored, err := f.store.Get(key)
	require.NoError(f.t, err)
	require.Equal(f.t, value, string(stored))
}

func (f fixture) delete(key storage.Key) {
	err := f.store.Delete(key)
	require.NoError(f.t, err)

	// Any key deleted should not be retrievable.
	_, err = f.store.Get(key)
	require.Error(f.t, err)
}

func (f fixture) requireContents(expected ...storage.Key) {
	keys, err := f.store.ListRecursive(storage.RootKey)
	require.NoError(f.t, err)
	require.Equal(f.t, expected, keys)
}

func key(path ...string) storage.Key {
	return storage.Key{Path: path}
}

func TestCrud(t *testing.T) {
	f := fixture{t: t, store: NewFileStore(afero.NewMemMapFs(), "/")}

	a := key("letters", "a")
	b := key("letters", "b")
	c := key("letters", "c")

	require.Error(t, f.store.Delete(a))

	_, err := f.store.Get(a)
	require.Error(t, err)

	f.save(a, "a")
	f.requireContents(a)

	f.save(c, "c")
	f.requireContents(a, c)

	// Store listings are lexically sorted.
	f.save(b, "b")
	f.requireContents(a, b, c)

	// Update a value
	f.save(a, "A")
	f.requireContents(a, b, c)

	f.delete(a)
	f.requireContents(b, c)
}

func TestIllegalKeys(t *testing.T) {
	store := NewFileStore(afero.NewMemMapFs(), "/")

	illegalKeys := []string{
		" key",
		"key/",
		"ke,y",
	}

	for _, k := range illegalKeys {
		badKey := key(k)
		require.Error(t, store.Save(badKey, []byte("data")))
		require.Error(t, store.Delete(badKey))
		_, err := store.Get(badKey)
		require.Error(t, err)
	}
}
