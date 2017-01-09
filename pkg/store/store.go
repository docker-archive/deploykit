package store

// Snapshot provides means to save and load an object.  This is not meant to be
// a generic k-v store.
type Snapshot interface {

	// Save marshals (encodes) and saves a snapshot of the given object.
	Save(obj interface{}) error

	// Load loads a snapshot and marshals (decodes) into the given reference.
	// If no data is available to unmarshal into the given struct, the fuction returns nil.
	Load(output interface{}) error
}
