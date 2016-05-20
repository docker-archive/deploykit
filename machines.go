package libmachete

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/storage"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"reflect"
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

	// Exists returns true if machine identified by key already exists
	Exists(key string) bool

	// CreateMachine adds a new machine from the input reader.
	CreateMachine(provisioner api.Provisioner, ctx context.Context, cred api.Credential,
		template api.MachineRequest, input io.Reader, codec *Codec) (<-chan interface{}, *Error)

	// DeleteMachine delete a machine with input (optional) in the input reader.  The template contains workflow tasks
	// for tear down of the machine; however that behavior can also be overriden by the input.
	DeleteMachine(provisioner api.Provisioner, ctx context.Context, cred api.Credential,
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
	list, err := cm.store.List()
	if err != nil {
		return nil, err
	}
	for _, i := range list {
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
	return out, nil
}

func (cm *machines) Get(key string) (storage.MachineRecord, error) {
	m, err := cm.store.GetRecord(storage.MachineID(key))
	if err != nil {
		return storage.MachineRecord{}, err
	}
	return *m, nil
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
		err = ensureValidContentType(codec).unmarshal(buff, request)
		if err != nil {
			return nil, err
		}
	}

	return request, nil
}

// CreateMachine creates a new machine from the input reader.
func (cm *machines) CreateMachine(
	provisioner api.Provisioner,
	ctx context.Context,
	cred api.Credential,
	template api.MachineRequest,
	input io.Reader,
	codec *Codec) (<-chan interface{}, *Error) {

	provisionerName := cred.ProvisionerName()
	request, err := cm.populateRequest(provisioner, template, input, codec)
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}

	key := request.Name()

	log.Infoln("name=", key, "cred=", cred, "template=", template, "req=", request)

	if cm.Exists(key) {
		return nil, &Error{ErrDuplicate, fmt.Sprintf("Key exists: %v", key)}
	}

	// First save a record
	record := &storage.MachineRecord{
		MachineSummary: storage.MachineSummary{
			Status:      "initiated",
			Name:        storage.MachineID(key),
			Provisioner: provisionerName,
			Created:     storage.Timestamp(time.Now().Unix()),
		},
	}
	record.AppendEvent(storage.Event{Name: "init", Message: "Create starts", Data: request})
	record.AppendChange(request)

	if err := cm.store.Save(*record, request); err != nil {
		return nil, &Error{Message: err.Error()}
	}
	tasks, err := provisioner.GetProvisionTasks(request.ProvisionWorkflow())
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}

	log.Infoln("About to run tasks:", tasks)

	return cm.runTasks(provisioner, tasks, record, ctx, cred, request,
		func() error {
			record.Status = "running"
			record.LastModified = storage.Timestamp(time.Now().Unix())
			if ip, err := provisioner.GetIPAddress(request); err == nil {
				record.IPAddress = ip
			}
			return cm.store.Save(*record, request)
		})
}

// DeleteMachine creates a new machine from the input reader.
func (cm *machines) DeleteMachine(
	provisioner api.Provisioner,
	ctx context.Context,
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

	record.AppendEvent(storage.Event{Name: "init-destroy", Message: "Destroy starts", Data: lastChange})

	if err := cm.store.Save(record, lastChange); err != nil {
		return nil, &Error{Message: err.Error()}
	}

	log.Infoln("About to run tasks:", tasks)

	// Need a way to clean up the database of lots of terminated instances.
	return cm.runTasks(provisioner, tasks, &record, ctx, cred, lastChange,
		func() error {
			record.Status = "terminated"
			record.LastModified = storage.Timestamp(time.Now().Unix())
			return cm.store.Save(record, lastChange)
		})
}

func (cm *machines) runTasks(
	provisioner api.Provisioner, tasks []api.Task, record *storage.MachineRecord,
	ctx context.Context, cred api.Credential, request api.MachineRequest,
	onComplete func() error) (<-chan interface{}, *Error) {

	events := make(chan interface{})

	go func() {
		close(events)

		for _, task := range tasks {

			taskEvents := make(chan interface{})

			go func() {
				log.Infoln("RUNNING:", task)
				event := storage.Event{
					Name: string(task.Name),
				}
				if err := task.Do(provisioner, ctx, cred, request, taskEvents); err != nil {
					event.Message = task.Message + " errored: " + err.Error()
					event.Error = err.Error()
				} else {
					event.Message = task.Message + " completed"
				}

				taskEvents <- event
				close(taskEvents) // unblocks the listener

				log.Infoln("FINISH:", task)
			}()

			record.Status = "pending"

			for te := range taskEvents {

				stop := false

				event := storage.Event{
					Name: string(task.Name),
				}

				event.Data = te

				// Some events implement both Error and HashMachineState.
				// So first check for errors then do type switch on HashMachineState

				log.Infoln("Check error:", te)
				switch te := te.(type) {
				case storage.Event:
					event = te
				case api.HasError:
					if e := te.GetError(); e != nil {
						event.Error = e.Error()
						stop = true
					}
				case error:
					event.Error = te.Error()
					stop = true
				}

				ms, is := te.(api.HasMachineState)
				log.Infoln("Check MachineState:", te, "is=", is, "type=", reflect.TypeOf(te))
				if is {
					log.Infoln("HasMachineState:", te)
					if provisionedState := ms.GetState(); provisionedState != nil {
						log.Infoln("Final provisioned state:", provisionedState)
						request = provisionedState
					}
				}

				record.AppendEvent(event)
				err := cm.store.Save(*record, request)
				log.Infoln("Saved:", "err=", err, len(record.Events), record)

				if stop {
					log.Warningln("Stopping due to error")

					record.Status = "failed"
					record.LastModified = storage.Timestamp(time.Now().Unix())
					err := cm.store.Save(*record, request)
					if err != nil {
						log.Warningln("err=", err)
					}
					return
				}
			}

			if err := onComplete(); err != nil {
				log.Warningln("complete-err=", err)
			}
			return
		}

	}()

	return events, nil
}
