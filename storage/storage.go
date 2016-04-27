package storage

//go:generate mockgen -package mock -destination mock/mock_storage.go github.com/docker/libmachete/storage Storage

// Storage defines the operations available for accessing and modifying machine inventory state.
type Storage interface {
	// Reads the value stored at a key.  If the key is not found or there is a problem reading
	// from the store, an error is returned.
	Read(key string) ([]byte, error)

	ReadAll() (map[string][]byte, error)

	// Writes a value associated with a key.
	Write(key string, data []byte) error

	// Deletes any association with a key, will not return an error if the key does not exist.
	Delete(key string) error

	Close()
}
