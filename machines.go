package libmachete

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"io"
	"io/ioutil"
	"sort"
	"time"
)

// MachineRequestBuilder is a provisioner-provided function that creates a typed request
// that satisfies the MachineRequest interface.
type MachineRequestBuilder func() api.MachineRequest

// Machines manages the lifecycle of a machine / node.
type Machines interface {
	// List
	List() ([]storage.MachineSummary, error)

	// ListIds
	ListIds() ([]string, error)

	// Get returns a machine identified by key
	Get(key string) (storage.MachineRecord, error)

	// CreateMachine adds a new machine from the input reader.
	CreateMachine(
		provisioner api.Provisioner,
		keystore api.KeyStore,
		cred api.Credential,
		template api.MachineRequest,
		input io.Reader,
		codec *Codec) (<-chan interface{}, *Error)

	// DeleteMachine delete a machine with input (optional) in the input reader.  The template contains workflow
	// tasks for tear down of the machine; however that behavior can also be overriden by the input.
	DeleteMachine(
		provisioner api.Provisioner,
		keystore api.KeyStore,
		cred api.Credential,
		record storage.MachineRecord) (<-chan interface{}, *Error)
}

type machines struct {
	store storage.Machines
}

// NewMachines creates an instance of the manager given the backing store.
func NewMachines(store storage.Machines) Machines {
	return &machines{store: store}
}

func (cm *machines) List() ([]storage.MachineSummary, error) {
	out := []storage.MachineSummary{}
	ids, err := cm.ListIds()
	if err != nil {
		return nil, err
	}
	for _, i := range ids {
		if record, err := cm.Get(string(i)); err == nil {
			out = append(out, record.MachineSummary)
		}
	}
	return out, nil
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
	sort.Strings(out)
	return out, nil
}

func (cm *machines) Get(key string) (storage.MachineRecord, error) {
	m, err := cm.store.GetRecord(storage.MachineID(key))
	if err != nil {
		return storage.MachineRecord{}, err
	}
	return *m, nil
}

func (cm *machines) exists(key string) bool {
	_, err := cm.Get(key)
	return err == nil
}

func (cm *machines) populateRequest(
	provisioner api.Provisioner,
	template api.MachineRequest,
	input io.Reader,
	codec *Codec) (api.MachineRequest, error) {

	// Three components are used to fully populate a MachineRequest:
	// 1. a stock request with low-level defaults from the provisioner
	// 2. an externally-defined template which may be incomplete
	// 3. extra parameters that should supplement (and possibly override) fields in (1) or (2)

	request := provisioner.NewRequestInstance()

	if template != nil {
		request = template
	}

	buff, err := ioutil.ReadAll(input)
	if err == nil && len(buff) > 0 {
		err = codec.unmarshal(buff, request)
		if err != nil {
			return nil, err
		}
	}

	return request, nil
}

// CreateMachine creates a new machine from the input reader.
func (cm *machines) CreateMachine(
	provisioner api.Provisioner,
	keystore api.KeyStore,
	cred api.Credential,
	template api.MachineRequest,
	input io.Reader,
	codec *Codec) (<-chan interface{}, *Error) {

	request, err := cm.populateRequest(provisioner, template, input, codec)
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}

	key := request.Name()
	if cm.exists(key) {
		return nil, &Error{ErrDuplicate, fmt.Sprintf("Key exists: %v", key)}
	}

	// First save a record
	record := &storage.MachineRecord{
		MachineSummary: storage.MachineSummary{
			Status:      "initiated",
			MachineName: storage.MachineID(key),
			Provisioner: provisioner.Name(),
			Created:     storage.Timestamp(time.Now().Unix()),
		},
	}
	record.AppendEvent("init", "Create started", request)
	record.AppendChange(request)

	if err := cm.store.Save(*record, request); err != nil {
		return nil, &Error{Message: err.Error()}
	}
	tasks, err := provisioner.GetProvisionTasks(request.ProvisionWorkflow())
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}

	return runTasks(provisioner, keystore, tasks, record, cred, request,
		func(record storage.MachineRecord, state api.MachineRequest) error {
			return cm.store.Save(record, state)
		},
		func(record *storage.MachineRecord, state api.MachineRequest) {
			record.Status = "provisioned"
			record.LastModified = storage.Timestamp(time.Now().Unix())
			if ip, err := provisioner.GetIPAddress(state); err == nil {
				record.IPAddress = ip
			}
			if id, err := provisioner.GetInstanceID(state); err == nil {
				record.InstanceID = id
			}
		})
}

// DeleteMachine deletes an existing machine.  Completion of this marks the record as 'terminated'.
// TODO(chungers) - There needs to be a way for user to clean / garbage collect the records marked 'terminated'.
func (cm *machines) DeleteMachine(
	provisioner api.Provisioner,
	keystore api.KeyStore,
	cred api.Credential,
	record storage.MachineRecord) (<-chan interface{}, *Error) {

	lastChange := record.GetLastChange()
	if lastChange == nil {
		return nil, &Error{Message: fmt.Sprintf("Impossible state. Machine has no history:%v", record.Name)}
	}

	// On managing changes (or even removal) of the template and the impact on already-provisioned instances:
	// If the template is removed, we can still teardown the instance using a copy of the original request / intent.
	// If the template is updated and has new workflow impact, the user can 'upgrade' the machine (method to be provided)
	// so that the last change request correctly reflects the correct tasks to run to teardown.

	tasks, err := provisioner.GetTeardownTasks(lastChange.TeardownWorkflow())
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}

	record.AppendEvent("init-destroy", "Destroy started", lastChange)

	if err := cm.store.Save(record, nil); err != nil {
		return nil, &Error{Message: err.Error()}
	}

	return runTasks(provisioner, keystore, tasks, &record, cred, lastChange,
		func(record storage.MachineRecord, state api.MachineRequest) error {
			return cm.store.Save(record, state)
		},
		func(record *storage.MachineRecord, state api.MachineRequest) {
			record.Status = "terminated"
		})
}

// runTasks is the main task execution loop
func runTasks(
	provisioner api.Provisioner, keystore api.KeyStore,
	tasks []api.Task,
	record *storage.MachineRecord,
	cred api.Credential,
	request api.MachineRequest,
	save func(storage.MachineRecord, api.MachineRequest) error,
	onComplete func(*storage.MachineRecord, api.MachineRequest)) (<-chan interface{}, *Error) {

	events := make(chan interface{})
	go func() {
		defer close(events)
		for _, task := range tasks {
			taskEvents := make(chan interface{})

			record.Status = "pending"
			record.LastModified = storage.Timestamp(time.Now().Unix())
			save(*record, nil)

			go func(task api.Task) {
				log.Infoln("START", task.Name)
				event := storage.Event{Name: string(task.Name)}
				if task.Do != nil {
					if err := task.Do(provisioner, keystore, cred, *record, request, taskEvents); err != nil {
						event.Message = task.Message + " errored: " + err.Error()
						event.Error = err.Error()
						event.Status = -1
					} else {
						event.Message = task.Message + " completed"
						event.Status = 1
					}
				} else {
					event.Message = task.Message + " skipped"
				}
				taskEvents <- event
				close(taskEvents) // unblocks the listener
				log.Infoln("FINISH", task.Name)

			}(task) // Goroutine with a copy of the task to avoid data races.

			// TOOD(chungers) - until we separate out the state from the request, at least here
			// in code we attempt to communicate the proper treatment of request vs state.
			machineState := request

			for te := range taskEvents {
				stop := false
				event := storage.Event{
					Name:      string(task.Name),
					Timestamp: time.Now(),
				}
				event.AddData(te)

				switch te := te.(type) {
				case storage.Event:
					event = te
					if event.Status < 0 {
						stop = true
					}
				case api.HasError:
					if e := te.GetError(); e != nil {
						event.Error = e.Error()
						stop = true
					}
				case error:
					event.Error = te.Error()
					stop = true
				}

				if change, is := te.(api.MachineRequest); is {
					log.Infoln("MachineRequest mutated. Logging it.")
					record.AppendChange(change)
				}

				if ms, is := te.(api.HasMachineState); is {
					log.Infoln("HasMachineState:", te)
					if provisionedState := ms.GetState(); provisionedState != nil {
						log.Infoln("Final provisioned state:", provisionedState)
						machineState = provisionedState
					}
				}

				record.AppendEventObject(event)
				record.LastModified = storage.Timestamp(time.Now().Unix())
				save(*record, machineState)

				events <- event

				if stop {
					log.Warningln("Stopping due to error")
					record.Status = "failed"
					record.LastModified = storage.Timestamp(time.Now().Unix())
					save(*record, machineState)
					return
				}
			}
		}

		onComplete(record, nil)
		record.LastModified = storage.Timestamp(time.Now().Unix())
		save(*record, nil)
		return
	}()

	return events, nil
}
