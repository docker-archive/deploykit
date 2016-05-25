package ssh

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	gossh "golang.org/x/crypto/ssh"
)

var (
	// ErrKeyGeneration is the error when keys cannot be generated
	ErrKeyGeneration = errors.New("Unable to generate key")

	// ErrValidation is the error when bytes cannot be verified to be valid keys
	ErrValidation = errors.New("Unable to validate key")

	// ErrPublicKey is the error when generating a new public key fails
	ErrPublicKey = errors.New("Unable to convert public key")

	// ErrUnableToWriteFile is the error when keys cannot be written to files
	ErrUnableToWriteFile = errors.New("Unable to write file")
)

// KeyPair is a public and private key pair
type KeyPair struct {
	PrivateKey []byte
	PublicKey  []byte
}

// NewKeyPair generates a new SSH keypair
// This will return a private & public key encoded as DER.
func NewKeyPair() (keyPair *KeyPair, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, ErrKeyGeneration
	}

	if err := priv.Validate(); err != nil {
		return nil, ErrValidation
	}

	privDer := x509.MarshalPKCS1PrivateKey(priv)

	pubSSH, err := gossh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, ErrPublicKey
	}

	return &KeyPair{
		PrivateKey: privDer,
		PublicKey:  gossh.MarshalAuthorizedKey(pubSSH),
	}, nil
}

// KeyWriter writes the key data at the given path
type KeyWriter func(path string, data []byte, perm os.FileMode) error

// Write writes the key pair
func (kp *KeyPair) Write(privateKeyPath, publicKeyPath string, sink KeyWriter) error {
	files := []struct {
		File  string
		Type  string
		Value []byte
	}{
		{
			File:  privateKeyPath,
			Value: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Headers: nil, Bytes: kp.PrivateKey}),
		},
		{
			File:  publicKeyPath,
			Value: kp.PublicKey,
		},
	}

	for _, v := range files {
		if err := sink(v.File, v.Value, 0600); err != nil {
			return err
		}
	}

	return nil
}

// WriteToFile writes keypair to files
func (kp *KeyPair) WriteToFile(privateKeyPath string, publicKeyPath string) error {
	files := []struct {
		File  string
		Type  string
		Value []byte
	}{
		{
			File:  privateKeyPath,
			Value: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Headers: nil, Bytes: kp.PrivateKey}),
		},
		{
			File:  publicKeyPath,
			Value: kp.PublicKey,
		},
	}

	for _, v := range files {
		f, err := os.Create(v.File)
		if err != nil {
			return ErrUnableToWriteFile
		}

		if _, err := f.Write(v.Value); err != nil {
			return ErrUnableToWriteFile
		}

		// windows does not support chmod
		switch runtime.GOOS {
		case "darwin", "linux":
			if err := f.Chmod(0600); err != nil {
				return err
			}
		}
	}

	return nil
}

// Fingerprint calculates the fingerprint of the public key
func (kp *KeyPair) Fingerprint() string {
	b, _ := base64.StdEncoding.DecodeString(string(kp.PublicKey))
	h := md5.New()

	io.WriteString(h, string(b))

	return fmt.Sprintf("%x", h.Sum(nil))
}

// GenerateSSHKey generates SSH keypair based on path of the private key
// The public key would be generated to the same path with ".pub" added
func GenerateSSHKey(path string) error {
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("Desired directory for SSH keys does not exist: %s", err)
		}

		kp, err := NewKeyPair()
		if err != nil {
			return fmt.Errorf("Error generating key pair: %s", err)
		}

		if err := kp.WriteToFile(path, fmt.Sprintf("%s.pub", path)); err != nil {
			return fmt.Errorf("Error writing keys to file(s): %s", err)
		}
	}

	return nil
}
