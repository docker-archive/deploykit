package storage

import (
	"fmt"
	"github.com/docker/libmachete/provisioners/api"
	"github.com/docker/libmachete/ssh"
	"strings"
	"sync"
	"time"
)

// ContextID is the type of the context key
type ContextID string

// Contexts handles the storage of context objects
type Contexts interface {
	Save(id ContextID, contextData interface{}) error

	List() ([]ContextID, error)

	GetContext(id ContextID, contextData interface{}) error

	Delete(id ContextID) error
}

// MachineID is the globally-unique identifier for machines.
type MachineID string

// Machines handles storage of machine inventory.  In addition to standard fields for all machines,
// it allows provisioners to include custom data.
type Machines interface {
	Save(record MachineRecord, provisionerData interface{}) error

	List() ([]MachineID, error)

	GetRecord(id MachineID) (*MachineRecord, error)

	GetDetails(id MachineID, provisionerData interface{}) error

	Delete(id MachineID) error
}

// KeyID is a unique id for the SSH key
type KeyID string

// Keys manage the SSH keys for a machine
type Keys interface {
	Save(id KeyID, keyPair *ssh.KeyPair) error

	List() ([]KeyID, error)

	GetPublicKey(id KeyID) ([]byte, error)

	Delete(id KeyID) error
}

// Timestamp is a unix epoch timestamp, in seconds.
type Timestamp uint64

// Event is
type Event struct {
	Timestamp time.Time   `json:"on" yaml:"on"`
	Name      string      `json:"event" yaml:"event"`
	Message   string      `json:"message" yaml:"message"`
	Data      interface{} `json:"data,omitempty" yaml:"data"`
	Error     string      `json:"error,omitempty" yaml:"error"`
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
	MachineSummary

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
	Changes []api.MachineRequest `json:"changes" yaml:"changes"`

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
		m.Changes = []api.MachineRequest{}
	}
	m.Changes = append(m.Changes, c)
}

// AppendEvent appends an event to the machine record
func (m *MachineRecord) AppendEvent(e Event) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.Events == nil {
		m.Events = []Event{}
	}
	e.Timestamp = time.Now()
	m.Events = append(m.Events, e)
}

// CredentialsID is the globally-unique identifier for credentials.
type CredentialsID string

// Credentials handles storage of identities and secrets for authenticating with third-party
// systems.
type Credentials interface {
	Save(id CredentialsID, credentialsData interface{}) error

	List() ([]CredentialsID, error)

	GetCredentials(id CredentialsID, credentialsData interface{}) error

	Delete(id CredentialsID) error
}

// TemplateID is a unique identifier for template within a provisioner namespace
type TemplateID struct {
	Provisioner string
	Name        string
}

// Key returns the key used for looking up the template.  Key is composed of the provisioner
// name and the name of the template (scoped to a provisioner).
func (t TemplateID) Key() string {
	return fmt.Sprintf("%s-%s", t.Provisioner, t.Name)
}

// TemplateIDFromString returns a TemplateID from a simple untyped string of some format.
func TemplateIDFromString(s string) TemplateID {
	p := strings.Split(s, "-")
	if len(p) > 1 {
		return TemplateID{p[0], p[1]}
	}
	return TemplateID{"", p[1]} // Invalid template
}

// Templates handles storage of template
type Templates interface {
	Save(id TemplateID, templateData interface{}) error

	List() ([]TemplateID, error)

	GetTemplate(id TemplateID, templateData interface{}) error

	Delete(id TemplateID) error
}
