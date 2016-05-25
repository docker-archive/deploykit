package api

// KeyStore manages key required to access the machine instance
type KeyStore interface {

	// NewKeyPair creates and saves a new key pair identified by the id
	NewKeyPair(id string) error

	// GetPublicKey returns the public key bytes for the key pair identified by id
	GetPublicKey(id string) ([]byte, error)

	// Exists returns true if a key pair identified by the id exists
	Exists(id string) bool

	// Remove the keypair
	Remove(id string) error
}
