package storage

import (
	"fmt"
	"strings"
)

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

// Timestamp is a unix epoch timestamp, in seconds.
type Timestamp uint64

// MachineRecord is the storage structure that will be included for all machines.
type MachineRecord struct {
	Name         MachineID
	Provisioner  string
	Created      Timestamp
	LastModified Timestamp
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

// TemplatesID is a -unique identifier for template within a provisioner namespace
type TemplateID struct {
	Provisioner string
	Name        string
}

func (t TemplateID) Key() string {
	return fmt.Sprintf("%s-%s", t.Provisioner, t.Name)
}

func TemplateIDFromString(s string) TemplateID {
	p := strings.Split(s, "-")
	if len(p) > 1 {
		return TemplateID{p[0], p[1]}
	} else {
		return TemplateID{"", p[1]} // Invalid template
	}
}

// Template handles storage of template
type Templates interface {
	Save(id TemplateID, templateData interface{}) error

	List() ([]TemplateID, error)

	GetTemplate(id TemplateID, templateData interface{}) error

	Delete(id TemplateID) error
}
