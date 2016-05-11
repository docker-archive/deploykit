package libmachete

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"time"
)

type MachineRequestBuilder func() api.MachineRequest

var (
	machineCreators = map[string]MachineRequestBuilder{}
)

// RegisterMachineCreator registers the function that allocates an empty credential for a provisioner.
// This method should be invoke in the init() of the provisioner package.
func RegisterMachineRequestBuilder(provisionerName string, f MachineRequestBuilder) {
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

	// Get returns a machine identified by key
	Get(key string) (storage.MachineRecord, error)

	// Deletes the machine identified by key
	Delete(key string) error

	// Exists returns true if machine identified by key already exists
	Exists(key string) bool

	// CreateMachine adds a new machine from the input reader.
	CreateMachine(ctx context.Context, cred api.Credential,
		template api.MachineRequest, key string, input io.Reader, codec *Codec) *MachineError
}

type machines struct {
	store storage.Machines
}

// NewMachines creates an instance of the manager given the backing store.
func NewMachines(store storage.Machines) Machines {
	return &machines{store: store}
}

// NewMachine returns an empty machine object for a provisioner.
func (cm *machines) NewMachineRequest(provisionerName string) (api.MachineRequest, error) {
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

func (cm *machines) Get(key string) (storage.MachineRecord, error) {
	m, err := cm.store.GetRecord(storage.MachineID(key))
	if err != nil {
		return storage.MachineRecord{}, err
	}
	return *m, nil
}

func (cm *machines) Delete(key string) error {
	return cm.store.Delete(storage.MachineID(key))
}

func (cm *machines) Terminate(key string) error {
	mr, err := cm.Get(key)
	if err != nil {
		return err
	}
	mr.AppendEvent(storage.Event{Name: "terminate", Message: "Deleted"})
	return cm.store.Save(mr, nil)
}

func (cm *machines) Exists(key string) bool {
	_, err := cm.Get(key)
	return err == nil
}

// CreateMachine creates a new machine from the input reader.
func (cm *machines) CreateMachine(ctx context.Context, cred api.Credential,
	template api.MachineRequest, key string, input io.Reader, codec *Codec) *MachineError {

	if cm.Exists(key) {
		return &MachineError{ErrMachineDuplicate, fmt.Sprintf("Key exists: %v", key)}
	}

	provisioner := cred.ProvisionerName()
	mr, err := cm.NewMachineRequest(provisioner)
	if err != nil {
		return &MachineError{ErrMachineNotFound, fmt.Sprintf("Unknown provisioner:%s", provisioner)}
	}

	if template != nil {
		mr = template
	}

	buff, err := ioutil.ReadAll(input)
	if err == nil && len(buff) > 0 {
		if err = cm.Unmarshal(codec, buff, mr); err != nil {
			return &MachineError{Message: err.Error()}
		}
	}

	mr.SetName(key)

	log.Infoln("cred=", cred, "template=", template, "req=", mr)

	// First save a record
	record := &storage.MachineRecord{
		Name:        storage.MachineID(key),
		Provisioner: provisioner,
		Created:     storage.Timestamp(time.Now().Unix()),
	}
	record.AppendEvent(storage.Event{Name: "init", Message: "Create starts", Data: mr})

	if err = cm.store.Save(*record, mr); err != nil {
		return &MachineError{Message: err.Error()}
	}

	// TODO - start process
	go func() {

		for _, e := range []storage.Event{
			{Name: "ssh", Message: "generated ssh key"},
			{Name: "instance", Message: "instance created."},
			{Name: "userdata", Message: "set up per instance userdata"},
			{Name: "engine", Message: "installing engine"},
			{Name: "engine", Message: "engine installed"},
			{Name: "provisioned", Message: "machine is ready."},
		} {
			time.Sleep(10 * time.Second) // BOGUS
			record.AppendEvent(e)
			err := cm.store.Save(*record, mr)
			log.Infoln("Saved:", "err=", err, len(record.Events), record)
		}
	}()

	return nil
}
