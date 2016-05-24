package storage

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libmachete/provisioners/api"
	"strings"
	"sync"
	"time"
)

// ContextID is the type of the context key
type ContextID string

// MachineID is the globally-unique identifier for machines.
type MachineID string

// Timestamp is a unix epoch timestamp, in seconds.
type Timestamp uint64

// Event is captures the data / emitted by tasks
type Event struct {
	Timestamp time.Time `json:"on" yaml:"on"`
	Name      string    `json:"event" yaml:"event"`
	Message   string    `json:"message" yaml:"message"`

	// Data is an encode string of some relevant data
	Data string `json:"data,omitempty" yaml:"data"`

	// ContentType is the content type of the data (yaml or json -- MIME type)
	ContentType string `json:"content_type" yaml:"content_type"`

	Error string `json:"error,omitempty" yaml:"error"`

	// Status: 0 = not run, 1 = success, -1 = error
	Status int `json:"status" yaml:"status"`
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
	Changes []*api.BaseMachineRequest `json:"changes" yaml:"changes"`

	lock sync.Mutex
}

// GetLastChange returns the last change requested.
func (m *MachineRecord) GetLastChange() api.MachineRequest {
	if len(m.Changes) > 0 {
		return m.Changes[len(m.Changes)-1]
	}
	return nil
}

// AppendChange appends a change to the record
func (m *MachineRecord) AppendChange(c api.MachineRequest) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.Changes == nil {
		m.Changes = []*api.BaseMachineRequest{}
	}
	m.Changes = append(m.Changes, &api.BaseMachineRequest{
		MachineName:        c.Name(),
		Provisioner:        c.ProvisionerName(),
		ProvisionerVersion: c.Version(),
		Provision:          c.ProvisionWorkflow(),
		Teardown:           c.TeardownWorkflow(),
	})
}

// AddData appends custom data with a default encoding
func (e *Event) AddData(data interface{}) {
	if buff, err := json.Marshal(data); err == nil {
		e.ContentType = "application/json"
		e.Data = string(buff)
	}
}

// AppendEvent appends an event to the machine record
func (m *MachineRecord) AppendEvent(name, message string, data interface{}) {
	e := Event{
		Name:      name,
		Message:   message,
		Timestamp: time.Now(),
	}
	e.AddData(data)
	m.AppendEventObject(e)
}

// AppendEventObject appends the full event object
func (m *MachineRecord) AppendEventObject(e Event) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.Events == nil {
		m.Events = []Event{}
	}
	m.Events = append(m.Events, e)
}

// CredentialsID is the globally-unique identifier for credentials.
type CredentialsID string

// TemplateID is a unique identifier for template within a provisioner namespace
type TemplateID struct {
	Provisioner string `json:"provisioner"`
	Name        string `json:"name"`
}

// Key returns the key used for looking up the template.  Key is composed of the provisioner
// name and the name of the template (scoped to a provisioner).
func (t TemplateID) Key() string {
	return fmt.Sprintf("%s-%s", t.Provisioner, t.Name)
}

// TemplateIDFromString returns a TemplateID from a simple untyped string of some format.
// TODO(wfarner): Consider solving this in the store to avoid the string formatting dance.
func TemplateIDFromString(s string) TemplateID {
	p := strings.Split(s, "-")
	if len(p) > 1 {
		return TemplateID{p[0], p[1]}
	}
	return TemplateID{"", p[1]} // Invalid template
}
