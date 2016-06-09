package libmachete

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"io"
	"io/ioutil"
	"time"
)

// MachineRequestBuilder is a provisioner-provided function that creates a typed request
// that satisfies the MachineRequest interface.
type MachineRequestBuilder func() api.MachineRequest

// Machines manages the lifecycle of a machine / node.
type Machines interface {
	// List returns summaries of all machines.
	List() ([]MachineSummary, *Error)

	// ListIds returns the identifiers for all machines.
	ListIds() ([]MachineID, *Error)

	// Get returns a machine identified by key
	Get(id MachineID) (MachineRecord, *Error)

	// CreateMachine adds a new machine from the input reader.
	CreateMachine(
		provisioner api.Provisioner,
		keystore api.KeyStore,
		cred api.Credential,
		template api.MachineRequest,
		input io.Reader,
		codec *Codec) (<-chan interface{}, *Error)

	// DeleteMachine delete a machine with input (optional) in the input reader.  The template contains workflow
	// tasks for tear down of the machine; however that behavior can also be overridden by the input.
	DeleteMachine(
		provisioner api.Provisioner,
		keystore api.KeyStore,
		cred api.Credential,
		record MachineRecord) (<-chan interface{}, *Error)
}

type machines struct {
	store storage.KvStore
}

// NewMachines creates an instance of the manager given the backing store.
func NewMachines(store storage.KvStore) Machines {
	return &machines{store: store}
}

func machineIDFromKey(key storage.Key) MachineID {
	requirePathLength(key, 1)
	return MachineID(key.Path[0])
}

func keyFromMachineID(id MachineID) storage.Key {
	return storage.Key{Path: []string{string(id)}}
}

func (cm *machines) List() ([]MachineSummary, *Error) {
	keys, err := cm.store.ListRecursive(storage.RootKey)
	if err != nil {
		return nil, UnknownError(err)
	}

	summaries := []MachineSummary{}
	for _, key := range keys {
		record, err := cm.Get(machineIDFromKey(key))
		if err != nil {
			return nil, UnknownError(err)
		}
		summaries = append(summaries, record.MachineSummary)
	}

	return summaries, nil
}

func (cm *machines) ListIds() ([]MachineID, *Error) {
	keys, err := cm.store.ListRecursive(storage.RootKey)
	if err != nil {
		return nil, UnknownError(err)
	}

	storage.SortKeys(keys)

	ids := []MachineID{}
	for _, key := range keys {
		ids = append(ids, machineIDFromKey(key))
	}
	return ids, nil
}

func (cm *machines) Get(id MachineID) (MachineRecord, *Error) {
	data, err := cm.store.Get(keyFromMachineID(id))
	if err != nil {
		return MachineRecord{}, UnknownError(err)
	}

	record := MachineRecord{}
	err = json.Unmarshal(data, &record)
	if err != nil {
		return record, UnknownError(err)
	}

	return record, nil
}

func (cm *machines) exists(id MachineID) bool {
	_, err := cm.store.Get(keyFromMachineID(id))
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

func machineRecordKey(baseKey storage.Key) storage.Key {
	return storage.Key{Path: append(baseKey.Path, "record.json")}
}

func machineDetailsKey(baseKey storage.Key) storage.Key {
	return storage.Key{Path: append(baseKey.Path, "details.json")}
}

func (cm machines) marshalAndSave(key storage.Key, value interface{}) *Error {
	data, err := json.Marshal(value)
	if err != nil {
		return UnknownError(err)
	}

	if err := cm.store.Save(key, data); err != nil {
		return UnknownError(err)
	}
	return nil
}

func (cm machines) saveMachineData(record MachineRecord, details api.MachineRequest) *Error {
	key := keyFromMachineID(record.MachineName)
	if err := cm.marshalAndSave(machineRecordKey(key), record); err != nil {
		return err
	}

	if err := cm.marshalAndSave(machineDetailsKey(key), details); err != nil {
		return err
	}
	return nil
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
		return nil, UnknownError(err)
	}

	machineID := MachineID(request.Name())
	if cm.exists(machineID) {
		return nil, &Error{Code: ErrDuplicate, Message: fmt.Sprintf("Key exists: %v", machineID)}
	}

	// First save a record
	record := MachineRecord{
		MachineSummary: MachineSummary{
			Status:      "initiated",
			MachineName: machineID,
			Provisioner: provisioner.Name(),
			Created:     Timestamp(time.Now().Unix()),
		},
	}
	record.AppendEvent("init", "Create started", request)
	record.AppendChange(request)

	apiErr := cm.saveMachineData(record, request)
	if apiErr != nil {
		return nil, apiErr
	}

	tasks, err := provisioner.GetProvisionTasks(request.ProvisionWorkflow())
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}

	return runTasks(provisioner, keystore, tasks, &record, cred, request,
		func(record MachineRecord, state api.MachineRequest) error {
			return cm.saveMachineData(record, state)
		},
		func(record *MachineRecord, state api.MachineRequest) {
			record.Status = "provisioned"
			record.LastModified = Timestamp(time.Now().Unix())
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
	record MachineRecord) (<-chan interface{}, *Error) {

	lastChange := record.GetLastChange()
	if lastChange == nil {
		return nil, &Error{Message: fmt.Sprintf("Impossible state. Machine has no history:%v", record.Name())}
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

	// TODO(wfarner): Nothing is actually deleted, is that intentional?
	//if err := cm.store.Save(record, nil); err != nil {
	//	return nil, &Error{Message: err.Error()}
	//}

	return runTasks(provisioner, keystore, tasks, &record, cred, lastChange,
		func(record MachineRecord, state api.MachineRequest) error {
			return cm.saveMachineData(record, state)
		},
		func(record *MachineRecord, state api.MachineRequest) {
			record.Status = "terminated"
		})
}

// runTasks is the main task execution loop
func runTasks(
	provisioner api.Provisioner,
	keystore api.KeyStore,
	tasks []api.Task,
	record *MachineRecord,
	cred api.Credential,
	request api.MachineRequest,
	save func(MachineRecord, api.MachineRequest) error,
	onComplete func(*MachineRecord, api.MachineRequest)) (<-chan interface{}, *Error) {

	events := make(chan interface{})
	go func() {
		defer close(events)
		for _, task := range tasks {
			taskEvents := make(chan interface{})

			record.Status = "pending"
			record.LastModified = Timestamp(time.Now().Unix())
			save(*record, nil)

			go func(task api.Task) {
				log.Infoln("START", task.Name)
				event := Event{Name: string(task.Name)}
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

			// TODO(chungers) - until we separate out the state from the request, at least here
			// in code we attempt to communicate the proper treatment of request vs state.
			machineState := request

			for te := range taskEvents {
				stop := false
				event := Event{
					Name:      string(task.Name),
					Timestamp: time.Now(),
				}
				event.AddData(te)

				switch te := te.(type) {
				case Event:
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
				record.LastModified = Timestamp(time.Now().Unix())
				save(*record, machineState)

				events <- event

				if stop {
					log.Warningln("Stopping due to error")
					record.Status = "failed"
					record.LastModified = Timestamp(time.Now().Unix())
					save(*record, machineState)
					return
				}
			}
		}

		onComplete(record, nil)
		record.LastModified = Timestamp(time.Now().Unix())
		save(*record, nil)
		return
	}()

	return events, nil
}
