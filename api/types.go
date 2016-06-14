package api

import (
	"github.com/docker/libmachete/provisioners/spi"
	"time"
)

// MachineID is the globally-unique identifier for machines.
type MachineID string

// Timestamp is a unix epoch timestamp, in seconds.
type Timestamp uint64

// Event is captures the data / emitted by tasks
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Name      string    `json:"name"`
	Message   string    `json:"message,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// MachineSummary keeps minimal information about a machine
type MachineSummary struct {
	Status       string    `json:"status" yaml:"status"`
	MachineName  MachineID `json:"name" yaml:"name"`
	InstanceID   string    `json:"instance_id" ymal:"instance_id"`
	IPAddress    string    `json:"ip" yaml:"ip"`
	Provisioner  string    `json:"provisioner" yaml:"provisioner"`
	Created      Timestamp `json:"created" yaml:"created"`
	LastModified Timestamp `json:"modified" yaml:"modified"`
}

// Name implements Resource.Name
func (m MachineSummary) Name() string {
	return string(m.MachineName)
}

// ID implements Resource.ID
func (m MachineSummary) ID() string {
	return m.InstanceID
}

// MachineRecord is the storage structure that will be included for all machines.
type MachineRecord struct {
	MachineSummary `yaml:",inline"`

	// Events are just a time-linear list of events with timestamp.
	Events []Event `json:"events" yaml:"events"`

	// Changes is an append-only slice of changes to be made to the state of the instance.
	// Unlike Events, which are more or less free-form with untyped 'data' attachments and timestamps,
	// Changes are appended only on well-defined phases like beginning of provision and upgrade.
	//
	// A few caveats:
	// 1. We really need to better separate request from actual state.  This is TBD
	// 2. A provisioned instance will have at least len(Changes) = 1. It's possible that some
	// machines (especially those baremetal/ home provisioned machines) can support the notion
	// of upgrade and we could see upgrades / downgrades and other states for this machine. It's also
	// possible that changes to workflow are applied to pre-existing records to fix-up the records.
	Changes []*spi.BaseMachineRequest `json:"changes" yaml:"changes"`
}

// GetLastChange returns the last change requested.
func (m *MachineRecord) GetLastChange() spi.MachineRequest {
	if len(m.Changes) > 0 {
		return m.Changes[len(m.Changes)-1]
	}
	return nil
}

// AppendChange appends a change to the record
func (m *MachineRecord) AppendChange(c spi.MachineRequest) {
	if m.Changes == nil {
		m.Changes = []*spi.BaseMachineRequest{}
	}
	m.Changes = append(m.Changes, &spi.BaseMachineRequest{
		MachineName:        c.Name(),
		Provisioner:        c.ProvisionerName(),
		ProvisionerVersion: c.Version(),
		Provision:          c.ProvisionWorkflow(),
		Teardown:           c.TeardownWorkflow(),
	})
}

// AppendEvent appends an event to the machine record
func (m *MachineRecord) AppendEvent(name, message string) {
	e := Event{
		Name:      name,
		Message:   message,
		Timestamp: time.Now(),
	}
	m.AppendEventObject(e)
}

// AppendEventObject appends the full event object
func (m *MachineRecord) AppendEventObject(event Event) {
	if m.Events == nil {
		m.Events = []Event{}
	}
	m.Events = append(m.Events, event)
}

// CredentialsID is the globally-unique identifier for credentials.
type CredentialsID struct {
	Provisioner string
	Name        string
}

// TemplateID is a unique identifier for template within a provisioner namespace
type TemplateID struct {
	Provisioner string `json:"provisioner"`
	Name        string `json:"name"`
}

// SSHKeyID is a unique id for an SSH key
type SSHKeyID string
