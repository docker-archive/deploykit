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

// MachineRequestBuilder is a provisioner-provided function that creates a typed request
// that satifies the MachineRequest interface.
type MachineRequestBuilder func() api.MachineRequest

var (
	machineCreators = map[string]MachineRequestBuilder{}
)

// RegisterMachineRequestBuilder registers by provisioner the request builder.
// This method should be invoke in the init() of the provisioner package.
func RegisterMachineRequestBuilder(provisionerName string, f MachineRequestBuilder) {
	lock.Lock()
	defer lock.Unlock()

	machineCreators[provisionerName] = f
}

// Machines manages the lifecycle of a machine / node.
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
	CreateMachine(provisioner api.Provisioner, ctx context.Context, cred api.Credential,
		template api.MachineRequest, input io.Reader, codec *Codec) (<-chan interface{}, *Error)
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
func (cm *machines) CreateMachine(provisioner api.Provisioner, ctx context.Context, cred api.Credential,
	template api.MachineRequest, input io.Reader, codec *Codec) (<-chan interface{}, *Error) {

	provisionerName := cred.ProvisionerName()
	mr := provisioner.NewRequestInstance()

	if template != nil {
		mr = template
	}

	buff, err := ioutil.ReadAll(input)
	if err == nil && len(buff) > 0 {
		if err = cm.Unmarshal(codec, buff, mr); err != nil {
			return nil, &Error{Message: err.Error()}
		}
	}

	key := mr.Name()
	if cm.Exists(key) {
		return nil, &Error{ErrDuplicate, fmt.Sprintf("Key exists: %v", key)}
	}

	log.Infoln("cred=", cred, "template=", template, "req=", mr)

	// First save a record
	record := &storage.MachineRecord{
		Name:        storage.MachineID(key),
		Provisioner: provisionerName,
		Created:     storage.Timestamp(time.Now().Unix()),
	}
	record.AppendEvent(storage.Event{Name: "init", Message: "Create starts", Data: mr})

	if err = cm.store.Save(*record, mr); err != nil {
		return nil, &Error{Message: err.Error()}
	}
	tasks := []api.Task{}
	for _, tn := range mr.ProvisionWorkflow() {

		if task, ok := GetTask(tn); ok {
			taskHandler := provisioner.GetTaskHandler(tn)
			if taskHandler != nil {
				task.Do = taskHandler // override the impl
			}
			tasks = append(tasks, task)
		}
	}

	if len(tasks) != len(mr.ProvisionWorkflow()) {
		return nil, &Error{Message: "unknown tasks"}
	} else {
		log.Infoln("About to run tasks:", tasks)
	}

	events := make(chan interface{})

	go func() {
		close(events)

		for _, task := range tasks {

			taskEvents := make(chan interface{})

			go func() {
				log.Infoln("RUNNING:", task)
				event := storage.Event{
					Name: string(task.Type),
				}
				if err := task.Do(ctx, cred, mr, taskEvents); err != nil {
					event.Message = task.Message + " errored: " + err.Error()
					event.Error = err
				} else {
					event.Message = task.Message + " completed"
				}

				taskEvents <- event
				close(taskEvents) // unblocks the listener

				log.Infoln("FINISH:", task)
			}()

			for te := range taskEvents {

				stop := false

				event := storage.Event{
					Name: string(task.Type),
				}

				event.Data = te

				switch te := te.(type) {
				case storage.Event:
					event = te
				case api.HasError:
					if e := te.GetError(); e != nil {
						event.Error = e
						stop = true
					}
				case error:
					event.Error = te
					stop = true
				}

				record.AppendEvent(event)
				err = cm.store.Save(*record, mr)
				log.Infoln("Saved:", "err=", err, len(record.Events), record)

				if stop {
					log.Warningln("Stopping due to error")
					return
				}
			}
		}

	}()

	return events, nil
}
