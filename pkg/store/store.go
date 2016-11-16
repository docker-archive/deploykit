package store

// Snapshot provides means to save and load an object.  This is not meant to be
// a generic k-v store.
type Snapshot interface {

	// Save marshals (encodes) and saves a snapshot of the given object.
	Save(obj interface{}) error

	// Load loads a snapshot and marshals (decodes) into the given reference
	Load(output interface{}) error
}
