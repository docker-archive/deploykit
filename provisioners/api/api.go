package api

import (
	"golang.org/x/net/context"
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

// A CreateInstanceEvent signals a state change in the instance create process.
type CreateInstanceEvent struct {
	Type       CreateInstanceEventType
	Error      error
	InstanceID string
}

// A DestroyInstanceEvent signals a state change in the instance destroy process.
type DestroyInstanceEvent struct {
	Type  DestroyInstanceEventType
	Error error
}

type HasError interface {
	GetError() error
}

func (event CreateInstanceEvent) GetError() error {
	return event.Error
}
func (event DestroyInstanceEvent) GetError() error {
	return event.Error
}

// MachineRequest defines the basic attributes that any provisioner's creation request must define.
type MachineRequest interface {
	Name() string
	ProvisionerName() string
	Version() string
	ProvisionWorkflow() []TaskType
}

// TaskType is a kind of work that a provisioner is able to run
type TaskType string

// TaskHandler is the unit of work that a provisioner is able to run.  It's identified by the TaskName
type TaskHandler func(context.Context, Credential, MachineRequest, chan<- interface{}) error

// Task is a descriptor of task that a provisioner supports.  Tasks are referenced by TaskName
// in a machine request or template.  This allows customization of provisioner behavior - such
// as skipping engine installs (if underlying image already has docker engine), skipping SSH
// key (if no sshd allowed), etc.
type Task struct {
	Type    TaskType    `json:"type" yaml:"type"`
	Message string      `json:"message" yaml:"message"`
	Do      TaskHandler `json:"-" yaml:"-"`
}

// BaseMachineRequest defines fields that all machine request types should contain.  This struct
// should be embedded in all provider-specific request structs.
type BaseMachineRequest struct {
	MachineName        string     `yaml:"name" json:"name"`
	Provisioner        string     `yaml:"provisioner" json:"provisioner"`
	ProvisionerVersion string     `yaml:"version" json:"version"`
	Workflow           []TaskType `yaml:"workflow,omitempty" json:"workflow,omitempty"`
}

// ProvisionWorkflow returns the tasks to do
func (req BaseMachineRequest) ProvisionWorkflow() []TaskType {
	return req.Workflow
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
	// NewRequestInstance retrieves a new instance of the request type consumed by
	// CreateInstance.
	NewRequestInstance() MachineRequest

	// GetTask returns a task that the provisioner can override and implement.
	GetTaskHandler(taskType TaskType) TaskHandler

	// Proposal - remove these from the interface. Rather everthing is defined via TaskTypes
	CreateInstance(request MachineRequest) (<-chan CreateInstanceEvent, error)

	DestroyInstance(instanceID string) (<-chan DestroyInstanceEvent, error)
}
