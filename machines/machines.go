package machines

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/api"
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
	List() ([]api.MachineSummary, *api.Error)

	// ListIds returns the identifiers for all machines.
	ListIds() ([]api.MachineID, *api.Error)

	// Get returns a machine identified by key
	Get(id api.MachineID) (api.MachineRecord, *api.Error)

	// CreateMachine adds a new machine from the input reader.
	CreateMachine(
		provisioner spi.Provisioner,
		template spi.MachineRequest,
		input io.Reader,
		codec api.Codec) (<-chan interface{}, *api.Error)

	// DeleteMachine delete a machine.  The record contains workflow tasks for tear down of the machine.
	DeleteMachine(provisioner spi.Provisioner, record api.MachineRecord) (<-chan interface{}, *api.Error)
}

type machines struct {
	store storage.KvStore
}

const (
	detailsRoot = "details"
	recordsRoot = "records"
)

var recordsRootKey = storage.Key{Path: []string{recordsRoot}}

// NewMachines creates an instance of the manager given the backing store.
func NewMachines(store storage.KvStore) Machines {
	return &machines{store: store}
}

func machineIDFromRecordKey(key storage.Key) api.MachineID {
	// Strip the record root path component.
	key.RequirePathLength(2)
	return api.MachineID(key.Path[1])
}

func keyFromMachineID(id api.MachineID) storage.Key {
	return storage.Key{Path: []string{string(id)}}
}

func (cm *machines) List() ([]api.MachineSummary, *api.Error) {
	keys, err := cm.store.ListRecursive(recordsRootKey)
	if err != nil {
		return nil, api.UnknownError(err)
	}

	summaries := []api.MachineSummary{}
	for _, key := range keys {
		record, err := cm.Get(machineIDFromRecordKey(key))
		if err != nil {
			return nil, api.UnknownError(err)
		}
		summaries = append(summaries, record.MachineSummary)
	}

	return summaries, nil
}

func (cm *machines) ListIds() ([]api.MachineID, *api.Error) {
	keys, err := cm.store.ListRecursive(recordsRootKey)
	if err != nil {
		return nil, api.UnknownError(err)
	}

	storage.SortKeys(keys)

	ids := []api.MachineID{}
	for _, key := range keys {
		ids = append(ids, machineIDFromRecordKey(key))
	}
	return ids, nil
}

func (cm *machines) Get(id api.MachineID) (api.MachineRecord, *api.Error) {
	data, err := cm.store.Get(machineRecordKey(keyFromMachineID(id)))
	if err != nil {
		return api.MachineRecord{}, api.UnknownError(err)
	}

	record := api.MachineRecord{}
	err = json.Unmarshal(data, &record)
	if err != nil {
		return record, api.UnknownError(err)
	}

	return record, nil
}

func (cm *machines) exists(id api.MachineID) bool {
	_, err := cm.store.Get(machineRecordKey(keyFromMachineID(id)))
	return err == nil
}

func (cm *machines) populateRequest(
	provisioner spi.Provisioner,
	template spi.MachineRequest,
	input io.Reader,
	codec api.Codec) (spi.MachineRequest, error) {

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
		err = codec.Unmarshal(buff, request)
		if err != nil {
			return nil, err
		}
	}

	return request, nil
}

func machineRecordKey(machineKey storage.Key) storage.Key {
	path := []string{recordsRoot}
	path = append(path, machineKey.Path...)
	return storage.Key{Path: path}
}

func machineDetailsKey(machineKey storage.Key) storage.Key {
	path := []string{detailsRoot}
	path = append(path, machineKey.Path...)
	return storage.Key{Path: path}
}

func (cm machines) marshalAndSave(key storage.Key, value interface{}) *api.Error {
	data, err := json.Marshal(value)
	if err != nil {
		return api.UnknownError(err)
	}

	if err := cm.store.Save(key, data); err != nil {
		return api.UnknownError(err)
	}
	return nil
}

func (cm machines) saveMachineData(record api.MachineRecord, details spi.MachineRequest) *api.Error {
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
	codec api.Codec) (<-chan interface{}, *api.Error) {

	request, err := cm.populateRequest(provisioner, template, input, codec)
	if err != nil {
		return nil, api.UnknownError(err)
	}

	machineID := api.MachineID(request.Name())
	if len(machineID) == 0 {
		return nil, &api.Error{Code: api.ErrBadInput, Message: "Machine name may not be empty"}
	}

	if cm.exists(machineID) {
		return nil, &api.Error{Code: api.ErrDuplicate, Message: fmt.Sprintf("Key exists: %v", machineID)}
	}

	// First save a record
	record := api.MachineRecord{
		MachineSummary: api.MachineSummary{
			Status:      "initiated",
			MachineName: machineID,
			Provisioner: provisioner.Name(),
			Created:     api.Timestamp(time.Now().Unix()),
		},
	}
	record.AppendEvent("init", "Create started")
	record.AppendChange(request)

	apiErr := cm.saveMachineData(record, request)
	if apiErr != nil {
		return nil, apiErr
	}

	tasks, err := filterTasks(provisioner.GetProvisionTasks(), request.ProvisionWorkflow())
	if err != nil {
		return nil, &api.Error{Message: err.Error()}
	}

	return runTasks(
		tasks,
		record,
		request,
		func(record api.MachineRecord, state spi.MachineRequest) error {
			return cm.saveMachineData(record, state)
		},
		func(record *api.MachineRecord, state spi.MachineRequest) {
			record.Status = "provisioned"
			record.LastModified = api.Timestamp(time.Now().Unix())
			// TODO(wfarner): Should errors here be fatal?  Currently they're silent.
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
	record api.MachineRecord) (<-chan interface{}, *api.Error) {

	lastChange := record.GetLastChange()
	if lastChange == nil {
		return nil, &api.Error{Message: fmt.Sprintf("Impossible state. Machine has no history:%v", record.Name())}
	}

	// On managing changes (or even removal) of the template and the impact on already-provisioned instances:
	// If the template is removed, we can still teardown the instance using a copy of the original request / intent.
	// If the template is updated and has new workflow impact, the user can 'upgrade' the machine
	// (method to be provided) so that the last change request correctly reflects the correct tasks to run to
	// teardown.

	tasks, err := filterTasks(provisioner.GetTeardownTasks(), lastChange.TeardownWorkflow())
	if err != nil {
		return nil, &api.Error{Message: err.Error()}
	}

	record.AppendEvent("init-destroy", "Destroy started")

	// TODO(wfarner): Nothing is actually deleted, is that intentional?
	//if err := cm.store.Save(record, nil); err != nil {
	//	return nil, &api.Error{Message: err.Error()}
	//}

	return runTasks(
		tasks,
		record,
		lastChange,
		func(record api.MachineRecord, state spi.MachineRequest) error {
			return cm.saveMachineData(record, state)
		},
		func(record *api.MachineRecord, state spi.MachineRequest) {
			record.Status = "terminated"
		})
}

// runTasks is the main task execution loop
func runTasks(
	tasks []spi.Task,
	record api.MachineRecord,
	request spi.MachineRequest,
	rawSave func(api.MachineRecord, spi.MachineRequest) error,
	onComplete func(*api.MachineRecord, spi.MachineRequest)) (<-chan interface{}, *api.Error) {

	save := func(record api.MachineRecord, machineState spi.MachineRequest) error {
		record.LastModified = api.Timestamp(time.Now().Unix())
		return rawSave(record, machineState)
	}

	events := make(chan interface{})
	go func() {
		defer close(events)
		for _, task := range tasks {
			taskEvents := make(chan interface{})

			// TODO(wfarner): The 'pending' status is used for both creating and destroying.  These should
			// be different states.
			record.Status = "pending"
			save(record, nil)

			go func(task spi.Task) {
				log.Infoln("START", task.Name)
				event := api.Event{Name: task.Name()}
				err := task.Run(record, request, taskEvents)
				if err != nil {
					event.Message = "failed"
					event.Error = err.Error()
				} else {
					event.Message = "completed"
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
				event := api.Event{Name: task.Name(), Timestamp: time.Now()}

				switch te := te.(type) {
				case api.Event:
					event = te
					if len(event.Error) > 0 {
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
				save(record, machineState)

				events <- event

				if stop {
					log.Warningln("Stopping due to error")
					record.Status = "failed"
					save(record, machineState)
					return
				}
			}
		}

		onComplete(&record, request)
		save(record, nil)
		return
	}()

	return events, nil
}
