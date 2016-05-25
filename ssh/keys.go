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
	"golang.org/x/crypto/ssh"
	"io"
)

var (
	// ErrKeyGeneration is the error when keys cannot be generated
	ErrKeyGeneration = errors.New("Unable to generate key")

	// ErrValidation is the error when bytes cannot be verified to be valid keys
	ErrValidation = errors.New("Unable to validate key")

	// ErrPublicKey is the error when generating a new public key fails
	ErrPublicKey = errors.New("Unable to convert public key")
)

// KeyPair is a public and private key pair
type KeyPair struct {
	PrivateKey []byte `json:"-" yaml:"-"`
	PublicKey  []byte `json:"-" yaml:"-"`

	// EncodedPrivateKey contains the bytes in the x509 PCKS DER format. In JSON it's encoded in Base64
	EncodedPrivateKey []byte `json:"public_key" yaml:"public_key"`

	// EncodedPublicKey contains the bytes in the OpenSSH authorized_keys format.  In this form it's ready
	// for import to most infrastructure providers as SSH key.  Note that when marshaled in JSON it's Base64 encoded.
	EncodedPublicKey []byte `json:"private_key" yaml:"private_key"`
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

	pubSSH, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, ErrPublicKey
	}

	pubPem := ssh.MarshalAuthorizedKey(pubSSH)
	privPem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Headers: nil, Bytes: privDer})

	return &KeyPair{
		PrivateKey:        privDer,
		PublicKey:         pubPem,
		EncodedPublicKey:  pubPem,
		EncodedPrivateKey: privPem,
	}, nil
}

// Fingerprint calculates the fingerprint of the public key
func (kp *KeyPair) Fingerprint() string {
	b, _ := base64.StdEncoding.DecodeString(string(kp.PublicKey))
	h := md5.New()
	io.WriteString(h, string(b))
	return fmt.Sprintf("%x", h.Sum(nil))
}
