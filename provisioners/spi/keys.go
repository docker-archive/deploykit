package spi

// TODO(wfarner): This appears to be split away from api.SSHKeys just to work around a circular import.  The root
// of the cycle (which we appear destined to revisit) appears to be TaskHandler.  Revisit that signature and the means
// by which Tasks receive dependencies to avoid this problem.

// KeyStore manages key required to access the machine instance
type KeyStore interface {

	// NewKeyPair creates and saves a new key pair identified by the id
	NewKeyPair(id SSHKeyID) error

	// GetEncodedPublicKey returns the public key bytes for the key pair identified by id.
	// The format is in the OpenSSH authorized_keys format.
	GetEncodedPublicKey(id SSHKeyID) ([]byte, error)

	// Remove the keypair
	Remove(id SSHKeyID) error
}

// SSHKeyID is a unique id for an SSH key
type SSHKeyID string
