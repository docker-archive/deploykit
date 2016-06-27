package spi

import (
	"strconv"
)

// CreateInstanceEventType is the identifier for an instance create event.
type CreateInstanceEventType int

// DestroyInstanceEventType is the identifier for an instance destroy event.
type DestroyInstanceEventType int

const (
	// CreateInstanceStarted indicates that creation has begun.
	CreateInstanceStarted CreateInstanceEventType = iota

	// CreateInstanceCompleted indicates that creation was successful.
	CreateInstanceCompleted

	// CreateInstanceError indicates a problem creating the instance.
	CreateInstanceError

	// DestroyInstanceStarted indicates that destruction has begun.
	DestroyInstanceStarted DestroyInstanceEventType = iota

	// DestroyInstanceCompleted indicates that destruction was successful.
	DestroyInstanceCompleted

	// DestroyInstanceError indicates a problem destroying the instance.
	DestroyInstanceError
)

// HasMachineState allows the events to return the state of the machine for storage
type HasMachineState interface {
	GetState() MachineRequest
}

// HasError interface allows object that contain errors be detected and treated as error
type HasError interface {
	GetError() error
}

// A CreateInstanceEvent signals a state change in the instance create process.
type CreateInstanceEvent struct {
	Type       CreateInstanceEventType
	Error      error
	InstanceID string
	Machine    MachineRequest // HACK - this should be changed to a Machine state object
}

// A DestroyInstanceEvent signals a state change in the instance destroy process.
type DestroyInstanceEvent struct {
	Type  DestroyInstanceEventType
	Error error
}

// GetError returns the error
func (event CreateInstanceEvent) GetError() error {
	return event.Error
}

// GetState returns the state if any
func (event CreateInstanceEvent) GetState() MachineRequest {
	return event.Machine
}

// GetError returns the error4
func (event DestroyInstanceEvent) GetError() error {
	return event.Error
}

// Resource is a generic resource that has a friendly name and an identifier that is unique to the provisioner
type Resource interface {
	// TODO(wfarner): It isn't clear which function is meaningful, and when.  In tasks related to SSH keys, Name()
	// is used for a key identifier, but in tasks related to instances, ID() is used for a machine identifier.
	Name() string
	ID() string
}

// MachineRequest defines the basic attributes that any provisioner's creation request must define.
type MachineRequest interface {
	Name() string
	ProvisionerName() string
	Version() string
	ProvisionWorkflow() []string
	TeardownWorkflow() []string
}

// ProvisionControls are parameters that give the provisioner instructions on how to provision the
// machine.  For example, network timeouts to a third-party API would be configured here rather than
// in the MachineRequest.
type ProvisionControls map[string][]string

// GetString returns the string value of a control.
func (p ProvisionControls) GetString(key string) (string, bool) {
	value, present := p[key]
	if present {
		return "", false
	}
	return value[0], true
}

// GetInt returns the int value of a control.
func (p ProvisionControls) GetInt(key string) (int, bool, error) {
	value, present := p.GetString(key)
	if !present {
		return 0, false, nil
	}

	i, err := strconv.Atoi(value)
	return i, true, err
}

// TaskHandler is the unit of work that a provisioner is able to run.  It's identified by the TaskName
// Note that the data passed as parameters are all read-only, by value (copy).
type TaskHandler func(Resource, MachineRequest, chan<- interface{}) error

// Task is a descriptor of task that a provisioner supports.  Tasks are referenced by Name
// in a machine request or template.  This allows customization of provisioner behavior - such
// as skipping engine installs (if underlying image already has docker engine), skipping SSH
// key (if no sshd allowed), etc.
type Task interface {
	Name() string
	Run(Resource, MachineRequest, chan<- interface{}) error
}

type composedTask struct {
	description string
	task        Task
	handler     TaskHandler
	taskFirst   bool
}

func (c composedTask) Name() string {
	return c.task.Name()
}

func (c composedTask) Description() string {
	return c.description
}

func (c composedTask) Run(resource Resource, request MachineRequest, events chan<- interface{}) error {
	if c.taskFirst {
		err := c.task.Run(resource, request, events)
		if err != nil {
			return err
		}
	}

	err := c.handler(resource, request, events)
	if err != nil {
		return err
	}

	if !c.taskFirst {
		err := c.task.Run(resource, request, events)
		if err != nil {
			return err
		}
	}

	return nil
}

// DoBeforeTask chains a TaskHandler to run before a task, aborting if the handler fails.
func DoBeforeTask(description string, handler TaskHandler, task Task) Task {
	return &composedTask{
		description: description,
		task:        task,
		handler:     handler,
		taskFirst:   false,
	}
}

// DoAfterTask chains a TaskHandler to run after a task, aborting if the task fails.
func DoAfterTask(description string, task Task, handler TaskHandler) Task {
	return &composedTask{
		description: description,
		task:        task,
		handler:     handler,
		taskFirst:   true,
	}
}

// BaseMachineRequest defines fields that all machine request types should contain.  This struct
// should be embedded in all provider-specific request structs.
type BaseMachineRequest struct {
	MachineName        string   `yaml:"name" json:"name,omitempty"`
	Provisioner        string   `yaml:"provisioner" json:"provisioner,omitempty"`
	ProvisionerVersion string   `yaml:"version" json:"version,omitempty"`
	Provision          []string `yaml:"provision,omitempty" json:"provision,omitempty"`
	Teardown           []string `yaml:"teardown,omitempty" json:"teardown,omitempty"`
}

// ProvisionWorkflow returns the tasks to do
func (req BaseMachineRequest) ProvisionWorkflow() []string {
	return req.Provision
}

// TeardownWorkflow returns the tasks to do
func (req BaseMachineRequest) TeardownWorkflow() []string {
	return req.Teardown
}

// Name returns the name to give the machine, once created.
func (req BaseMachineRequest) Name() string {
	return req.MachineName
}

// ProvisionerName returns the provisioner name
func (req BaseMachineRequest) ProvisionerName() string {
	return req.Provisioner
}

// Version returns a version string.  This is used for provisioners for schema migration and not used by framework.
func (req BaseMachineRequest) Version() string {
	return req.ProvisionerVersion
}

// A Provisioner is a vendor-agnostic API used to create and manage
// resources with an infrastructure provider.
type Provisioner interface {
	// GetProvisionTasks returns the available tasks for provisioning a resource.
	// Task names are generally specific verbs that the user has specified.  The manager can either return
	// no implementation (thus using framework defaults, or its own override implementation.
	GetProvisionTasks() []Task

	// GetTeardownTasks returns a list of available tasks for tearing down a resource.
	GetTeardownTasks() []Task

	// GetInstanceKey returns an instanceID based on the request.  It's up to the provisioner
	// on how to manage the mapping of machine request (which has a user-friendly name) to
	// an actual infrastructure identifier for the resource.
	// TODO(chungers) - the machine request here is the *state* not the request
	// TODO(wfarner): Seems like InstanceID effectively becomes a required (but hidden) attribute of MachineRequest;
	// seems like it should just become a first-class attribute.  Same with IPAddress.
	GetInstanceID(MachineRequest) (string, error)

	// GetIp returns the IP address from the record.  It's up to the provisioner to decide
	// which ip address, if more than one network interface cards are on an instance, should be
	// preferable and returned to the framework for tracking and managing for user purposes (e.g. ssh sessions)
	// TODO(chungers) - the machine request here is the *state* not the request
	GetIPAddress(MachineRequest) (string, error)

	CreateInstance(request MachineRequest) (<-chan CreateInstanceEvent, error)

	DestroyInstance(instanceID string) (<-chan DestroyInstanceEvent, error)
}
