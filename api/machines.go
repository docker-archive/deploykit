package api

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provisioners/spi"
	"github.com/docker/libmachete/storage"
	"io"
	"io/ioutil"
	"time"
)

// MachineRequestBuilder is a provisioner-provided function that creates a typed request
// that satisfies the MachineRequest interface.
type MachineRequestBuilder func() spi.MachineRequest

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
		provisioner spi.Provisioner,
		template spi.MachineRequest,
		input io.Reader,
		codec *Codec) (<-chan interface{}, *Error)

	// DeleteMachine delete a machine.  The record contains workflow tasks for tear down of the machine.
	DeleteMachine(provisioner spi.Provisioner, record MachineRecord) (<-chan interface{}, *Error)
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
	provisioner spi.Provisioner,
	template spi.MachineRequest,
	input io.Reader,
	codec *Codec) (spi.MachineRequest, error) {

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

func (cm machines) saveMachineData(record MachineRecord, details spi.MachineRequest) *Error {
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
	provisioner spi.Provisioner,
	template spi.MachineRequest,
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

	tasks, err := filterTasks(provisioner.GetProvisionTasks(), request.ProvisionWorkflow())
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}

	return runTasks(
		tasks,
		record,
		request,
		func(record MachineRecord, state spi.MachineRequest) error {
			return cm.saveMachineData(record, state)
		},
		func(record *MachineRecord, state spi.MachineRequest) {
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

func findTask(tasks []spi.Task, name string) *spi.Task {
	for _, task := range tasks {
		if task.Name() == name {
			return &task
		}
	}
	return nil
}

func taskNames(tasks []spi.Task) []string {
	names := []string{}
	for _, task := range tasks {
		names = append(names, task.Name())
	}
	return names
}

func filterTasks(tasks []spi.Task, selectNames []string) ([]spi.Task, error) {
	filtered := []spi.Task{}
	for _, name := range selectNames {
		task := findTask(tasks, name)
		if task != nil {
			filtered = append(filtered, *task)
		} else {
			return nil, fmt.Errorf(
				"Task %s is not supported, must be one of %s", name, taskNames(tasks))
		}
	}

	return filtered, nil
}

// DeleteMachine deletes an existing machine.  Completion of this marks the record as 'terminated'.
// TODO(chungers) - There needs to be a way for user to clean / garbage collect the records marked 'terminated'.
func (cm *machines) DeleteMachine(
	provisioner spi.Provisioner,
	record MachineRecord) (<-chan interface{}, *Error) {

	lastChange := record.GetLastChange()
	if lastChange == nil {
		return nil, &Error{Message: fmt.Sprintf("Impossible state. Machine has no history:%v", record.Name())}
	}

	// On managing changes (or even removal) of the template and the impact on already-provisioned instances:
	// If the template is removed, we can still teardown the instance using a copy of the original request / intent.
	// If the template is updated and has new workflow impact, the user can 'upgrade' the machine
	// (method to be provided) so that the last change request correctly reflects the correct tasks to run to
	// teardown.

	tasks, err := filterTasks(provisioner.GetTeardownTasks(), lastChange.TeardownWorkflow())
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}

	record.AppendEvent("init-destroy", "Destroy started", lastChange)

	// TODO(wfarner): Nothing is actually deleted, is that intentional?
	//if err := cm.store.Save(record, nil); err != nil {
	//	return nil, &Error{Message: err.Error()}
	//}

	return runTasks(
		tasks,
		record,
		lastChange,
		func(record MachineRecord, state spi.MachineRequest) error {
			return cm.saveMachineData(record, state)
		},
		func(record *MachineRecord, state spi.MachineRequest) {
			record.Status = "terminated"
		})
}

// runTasks is the main task execution loop
func runTasks(
	tasks []spi.Task,
	record MachineRecord,
	request spi.MachineRequest,
	save func(MachineRecord, spi.MachineRequest) error,
	onComplete func(*MachineRecord, spi.MachineRequest)) (<-chan interface{}, *Error) {

	events := make(chan interface{})
	go func() {
		defer close(events)
		for _, task := range tasks {
			taskEvents := make(chan interface{})

			record.Status = "pending"
			record.LastModified = Timestamp(time.Now().Unix())
			save(record, nil)

			go func(task spi.Task) {
				log.Infoln("START", task.Name())
				event := Event{Name: string(task.Name())}
				err := task.Run(record, request, taskEvents)
				if err != nil {
					event.Message = task.Name() + " failed: " + err.Error()
					event.Error = err.Error()
					event.Status = -1
				} else {
					event.Message = task.Name() + " completed"
					event.Status = 1
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
				event := Event{Name: task.Name(), Timestamp: time.Now()}
				event.AddData(te)

				switch te := te.(type) {
				case Event:
					event = te
					if event.Status < 0 {
						stop = true
					}
				case spi.HasError:
					if e := te.GetError(); e != nil {
						event.Error = e.Error()
						stop = true
					}
				case error:
					event.Error = te.Error()
					stop = true
				}

				if change, is := te.(spi.MachineRequest); is {
					log.Infoln("MachineRequest mutated. Logging it.")
					record.AppendChange(change)
				}

				if ms, is := te.(spi.HasMachineState); is {
					log.Infoln("HasMachineState:", te)
					if provisionedState := ms.GetState(); provisionedState != nil {
						log.Infoln("Final provisioned state:", provisionedState)
						machineState = provisionedState
					}
				}

				record.AppendEventObject(event)
				record.LastModified = Timestamp(time.Now().Unix())
				save(record, machineState)

				events <- event

				if stop {
					log.Warningln("Stopping due to error")
					record.Status = "failed"
					record.LastModified = Timestamp(time.Now().Unix())
					save(record, machineState)
					return
				}
			}
		}

		onComplete(&record, nil)
		record.LastModified = Timestamp(time.Now().Unix())
		save(record, nil)
		return
	}()

	return events, nil
}
