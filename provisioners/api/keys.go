package api

// KeyStore manages key required to access the machine instance
type KeyStore interface {

	// NewKeyPair creates and saves a new key pair identified by the id
	NewKeyPair(id string) error

	// GetEncodedPublicKey returns the public key bytes for the key pair identified by id.
	// The format is in the OpenSSH authorized_keys format.
	GetEncodedPublicKey(id string) ([]byte, error)

	// Remove the keypair
	Remove(id string) error
}
