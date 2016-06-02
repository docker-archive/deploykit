package storage

// NestedStore is a KvStore applies a scope to all operations on another KvStore.
type nestedStore struct {
	wrapped KvStore
	scope   Key
}

// NestedStore creates a store that will scope all operations.
func NestedStore(wrapped KvStore, scope ...string) KvStore {
	return &nestedStore{wrapped: wrapped, scope: Key{Path: scope}}
}

func (n nestedStore) applyScope(key Key) Key {
	return Key{Path: append(n.scope.Path, key.Path...)}
}

func (n nestedStore) removeScope(key Key) Key {
	return Key{Path: key.Path[len(n.scope.Path):]}
}

// Save implements KvStore.Save.
func (n nestedStore) Save(key Key, data []byte) error {
	return n.wrapped.Save(n.applyScope(key), data)
}

// List implements KvStore.List.
func (n nestedStore) ListRecursive(key Key) ([]Key, error) {
	scoped, err := n.wrapped.ListRecursive(n.applyScope(key))
	if err != nil {
		return nil, err
	}

	unscoped := []Key{}
	for _, key := range scoped {
		unscoped = append(unscoped, n.removeScope(key))
	}
	return unscoped, nil
}

// Get implements KvStore.Get.
func (n nestedStore) Get(key Key) ([]byte, error) {
	return n.wrapped.Get(n.applyScope(key))
}

// Delete implements KvStore.Delete.
func (n nestedStore) Delete(key Key) error {
	return n.wrapped.Delete(n.applyScope(key))
}
