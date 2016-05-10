package libmachete

import (
	"fmt"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"io"
	"io/ioutil"
	"sync"
)

var (
	machineCreators = map[string]func() api.Credential{}
)

// RegisterMachineCreator registers the function that allocates an empty credential for a provisioner.
// This method should be invoke in the init() of the provisioner package.
func RegisterMachineCreator(provisionerName string, f func() api.Credential) {
	lock.Lock()
	defer lock.Unlock()

	machineCreators[provisionerName] = f
}

const (
	ErrMachineDuplicate int = iota
	ErrMachineNotFound
)

type MachineError struct {
	Code    int
	Message string
}

func (e MachineError) Error() string {
	return e.Message
}

type Machines interface {
	// NewMachines creates an instance of the manager given the backing store.
	NewMachineRequest(provisionerName string) (api.MachineRequest, error)

	// Unmarshal decodes the bytes and applies onto the machine object, using a given encoding.
	// If nil codec is passed, the default encoding / content type will be used.
	Unmarshal(contentType *Codec, data []byte, m api.MachineRequest) error

	// Marshal encodes the given machine object and returns the bytes.
	// If nil codec is passed, the default encoding / content type will be used.
	Marshal(contentType *Codec, m api.MachineRequest) ([]byte, error)

	// ListIds
	ListIds() ([]string, error)

	// Saves the machine identified by key
	Save(key string, m api.MachineRequest) error

	// Get returns a machine identified by key
	Get(key string) (api.MachineRequest, error)

	// Deletes the machine identified by key
	Delete(key string) error

	// Exists returns true if machine identified by key already exists
	Exists(key string) bool

	// CreateMachine adds a new machine from the input reader.
	CreateMachine(ctx context.Context, cred api.Credential, key string, input io.Reader, codec *Codec) *MachineError

	// UpdateMachine updates an existing machine
	UpdateMachine(key string, input io.Reader, codec *Codec) *MachineError
}

type machines struct {
	store storage.Machines
}

// NewMachines creates an instance of the manager given the backing store.
func NewMachines(store storage.Machines) Machines {
	return &machines{store: store}
}

func ensureValidContentType(ct *Codec) *Codec {
	if ct != nil {
		return ct
	}
	return DefaultContentType
}

// NewMachine returns an empty machine object for a provisioner.
func (cm *machines) NewMachine(provisionerName string) (api.MachineRequest, error) {
	if c, has := machineCreators[provisionerName]; has {
		return c(), nil
	}
	return nil, fmt.Errorf("Unknown provisioner: %v", provisionerName)
}

// Unmarshal decodes the bytes and applies onto the machine object, using a given encoding.
// If nil codec is passed, the default encoding / content type will be used.
func (cm *machines) Unmarshal(contentType *Codec, data []byte, m api.MachineRequest) error {
	return ensureValidContentType(contentType).unmarshal(data, m)
}

// Marshal encodes the given machine object and returns the bytes.
// If nil codec is passed, the default encoding / content type will be used.
func (cm *machines) Marshal(contentType *Codec, m api.MachineRequest) ([]byte, error) {
	return ensureValidContentType(contentType).marshal(m)
}

func (cm *machines) ListIds() ([]string, error) {
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

func (cm *machines) Save(key string, m api.MachineRequest) error {
	return cm.store.Save(storage.MachinesID(key), m)
}

func (cm *machines) Get(key string) (api.MachineRequest, error) {
	// Since we don't know the provider, we need to read twice: first with a base
	// structure, then with a specific structure by provisioner.
	base := new(api.MachineRequestBase)
	err := cm.store.GetMachines(storage.MachinesID(key), base)
	if err != nil {
		return nil, err
	}

	detail, err := cm.NewMachine(base.ProvisionerName())
	if err != nil {
		return nil, err
	}

	err = cm.store.GetMachines(storage.MachinesID(key), detail)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (cm *machines) Delete(key string) error {
	return cm.store.Delete(storage.MachinesID(key))
}

func (cm *machines) Exists(key string) bool {
	base := new(api.MachineRequestBase)
	err := cm.store.GetMachines(storage.MachinesID(key), base)
	return err == nil
}

// CreateMachine creates a new machine from the input reader.
func (c *machines) CreateMachine(provisioner, key string, input io.Reader, codec *Codec) *MachineError {
	if c.Exists(key) {
		return &MachineError{ErrMachineDuplicate, fmt.Sprintf("Key exists: %v", key)}
	}

	cr, err := c.NewMachine(provisioner)
	if err != nil {
		return &MachineError{ErrMachineNotFound, fmt.Sprintf("Unknown provisioner:%s", provisioner)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &MachineError{Message: err.Error()}
	}

	if err = c.Unmarshal(codec, buff, cr); err != nil {
		return &MachineError{Message: err.Error()}
	}
	if err = c.Save(key, cr); err != nil {
		return &MachineError{Message: err.Error()}
	}
	return nil
}

func (c *machines) UpdateMachine(key string, input io.Reader, codec *Codec) *MachineError {
	if !c.Exists(key) {
		return &MachineError{ErrMachineNotFound, fmt.Sprintf("Machine not found: %v", key)}
	}

	buff, err := ioutil.ReadAll(input)
	if err != nil {
		return &MachineError{Message: err.Error()}

	}

	base := new(api.MachineRequestBase)
	if err = c.Unmarshal(codec, buff, base); err != nil {
		return &MachineError{Message: err.Error()}
	}

	detail, err := c.NewMachine(base.ProvisionerName())
	if err != nil {
		return &MachineError{ErrMachineNotFound, fmt.Sprintf("Unknow provisioner: %v", base.ProvisionerName())}
	}

	if err = c.Unmarshal(codec, buff, detail); err != nil {
		return &MachineError{Message: err.Error()}
	}

	if err = c.Save(key, detail); err != nil {
		return &MachineError{Message: err.Error()}
	}
	return nil
}
