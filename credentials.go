package libmachete

import (
	"fmt"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"sync"
)

var (
	credentialers = map[string]func() api.Credential{}
	lock          sync.Mutex
)

// RegisterCredentialer registers the function that allocates an empty credential for a provisioner.
// This method should be invoke in the init() of the provisioner package.
func RegisterCredentialer(provisionerName string, f func() api.Credential) {
	lock.Lock()
	defer lock.Unlock()

	credentialers[provisionerName] = f
}

type Credentials interface {
	// NewCredentials creates an instance of the manager given the backing store.
	NewCredential(provisionerName string) (api.Credential, error)

	// Unmarshal decodes the bytes and applies onto the credential object, using a given encoding.
	// If nil codec is passed, the default encoding / content type will be used.
	Unmarshal(contentType *codec, data []byte, cred api.Credential) error

	// Marshal encodes the given credential object and returns the bytes.
	// If nil codec is passed, the default encoding / content type will be used.
	Marshal(contentType *codec, cred api.Credential) ([]byte, error)

	// ListIds
	ListIds() ([]string, error)

	// Saves the credential identified by key
	Save(key string, cred api.Credential) error

	// Get returns a credential identified by key
	Get(key string) (api.Credential, error)

	// Deletes the credential identified by key
	Delete(key string) error

	// Exists returns true if credential identified by key already exists
	Exists(key string) bool
}

type credentials struct {
	store storage.Credentials
}

// NewCredentials creates an instance of the manager given the backing store.
func NewCredentials(store storage.Credentials) Credentials {
	return &credentials{store: store}
}

func ensureValidCredentialContentType(ct *codec) *codec {
	if ct != nil {
		return ct
	}
	return DefaultContentType
}

// NewCredential returns an empty credential object for a provisioner.
func (cm *credentials) NewCredential(provisionerName string) (api.Credential, error) {
	if c, has := credentialers[provisionerName]; has {
		return c(), nil
	}
	return nil, fmt.Errorf("Unknown provisioner: %v", provisionerName)
}

// Unmarshal decodes the bytes and applies onto the credential object, using a given encoding.
// If nil codec is passed, the default encoding / content type will be used.
func (cm *credentials) Unmarshal(contentType *codec, data []byte, cred api.Credential) error {
	return ensureValidCredentialContentType(contentType).unmarshal(data, cred)
}

// Marshal encodes the given credential object and returns the bytes.
// If nil codec is passed, the default encoding / content type will be used.
func (cm *credentials) Marshal(contentType *codec, cred api.Credential) ([]byte, error) {
	return ensureValidCredentialContentType(contentType).marshal(cred)
}

func (cm *credentials) ListIds() ([]string, error) {
	out := []string{}
	list, err := cm.store.List()
	if err != nil {
		return nil, err
	}
	for _, i := range list {
		out = append(out, string(i))
	}
	return out, nil
}

func (cm *credentials) Save(key string, cred api.Credential) error {
	return cm.store.Save(storage.CredentialsID(key), cred)
}

func (cm *credentials) Get(key string) (api.Credential, error) {
	// Since we don't know the provider, we need to read twice: first with a base
	// structure, then with a specific structure by provisioner.
	base := new(api.CredentialBase)
	err := cm.store.GetCredentials(storage.CredentialsID(key), base)
	if err != nil {
		return nil, err
	}

	detail, err := cm.NewCredential(base.ProvisionerName())
	if err != nil {
		return nil, err
	}

	err = cm.store.GetCredentials(storage.CredentialsID(key), detail)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (cm *credentials) Delete(key string) error {
	return cm.store.Delete(storage.CredentialsID(key))
}

func (cm *credentials) Exists(key string) bool {
	base := new(api.CredentialBase)
	err := cm.store.GetCredentials(storage.CredentialsID(key), base)
	return err == nil
}

const (
	ErrCredentialDuplicate int = iota
	ErrCredentialNotFound
)

type CredentialError struct {
	Code  int
	Error string
}

func (e CredentialError) Error() {
	return e.Error
}

func CreateCredential(c Credentials, provisioner, key string, input io.Reader) *CredentialError {
	if c.Exists(key) {
		return &CredentialError{ErrCredentialDuplicate, fmt.Stringf("Key exists: %v", key)}
	}

	cr, err := c.NewCredential(provisioner)
	if err != nil {
		return &CredentialError{ErrCredentialNotFound, fmt.Stringf("Unknown provisioner:%s", provisioner)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &CredentialError{Error: err.Error()}
	}

	if err = c.Unmarshal(CodecByContentTypeHeader(req), buff, cr); err != nil {
		return &CredentialError{Error: err.Error()}
	}
	if err = c.Save(key, cr); err != nil {
		return &CredentialError{Error: err.Error()}
	}
	return nil
}
